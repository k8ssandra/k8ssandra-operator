/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package reaper

import (
	"context"
	"fmt"
	"math"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/encryption"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reaper"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// ReaperReconciler reconciles a Reaper object
type ReaperReconciler struct {
	*config.ReconcilerConfig
	client.Client
	Scheme     *runtime.Scheme
	NewManager func() reaper.Manager
}

// +kubebuilder:rbac:groups=reaper.k8ssandra.io,namespace="k8ssandra",resources=reapers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=reaper.k8ssandra.io,namespace="k8ssandra",resources=reapers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=reaper.k8ssandra.io,namespace="k8ssandra",resources=reapers/finalizers,verbs=update
// +kubebuilder:rbac:groups="apps",namespace="k8ssandra",resources=deployments;statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="core",namespace="k8ssandra",resources=pods;secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="core",namespace="k8ssandra",resources=services,verbs=get;list;watch;create

func (r *ReaperReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx, "Reaper", req.NamespacedName)

	logger.Info("Starting Reaper reconciliation")

	// Fetch the Reaper instance
	actualReaper := &reaperapi.Reaper{}
	if err := r.Get(ctx, req.NamespacedName, actualReaper); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Reaper resource not found")
			return ctrl.Result{}, nil
		}
		logger.Info("Failed to fetch Reaper resource")
		return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
	}

	actualReaper = actualReaper.DeepCopy()
	patch := client.MergeFromWithOptions(actualReaper.DeepCopy())

	result, err := r.reconcile(ctx, actualReaper, logger)

	if patchErr := r.Status().Patch(ctx, actualReaper, patch); patchErr != nil {
		logger.Error(patchErr, "Failed to update Reaper status")
	} else {
		logger.Info("Updated Reaper status")
	}

	return result, err
}

func (r *ReaperReconciler) reconcile(ctx context.Context, actualReaper *reaperapi.Reaper, logger logr.Logger) (ctrl.Result, error) {

	actualReaper.Status.Progress = reaperapi.ReaperProgressPending
	actualReaper.Status.SetNotReady()

	var actualDc *cassdcapi.CassandraDatacenter
	if actualReaper.Spec.DatacenterRef.Name == "" {
		logger.Info("No CassandraDatacenter reference specified, skipping CassandraDatacenter reconciliation")
		actualDc = &cassdcapi.CassandraDatacenter{}
	} else {
		reconciledDc, result, err := r.reconcileDatacenter(ctx, actualReaper, logger)
		if !result.IsZero() || err != nil {
			return result, err
		}
		actualDc = reconciledDc.DeepCopy()
	}

	actualReaper.Status.Progress = reaperapi.ReaperProgressDeploying

	if result, err := r.reconcileDeployment(ctx, actualReaper, actualDc, logger); !result.IsZero() || err != nil {
		return result, err
	}

	if result, err := r.reconcileService(ctx, actualReaper, logger); !result.IsZero() || err != nil {
		return result, err
	}

	actualReaper.Status.Progress = reaperapi.ReaperProgressConfiguring

	if actualReaper.Spec.DatacenterRef.Name == "" {
		logger.Info("skipping adding DC to Reaper because its a Control Plane Reaper")
	} else {
		if result, err := r.configureReaper(ctx, actualReaper, actualDc, logger); !result.IsZero() || err != nil {
			return result, err
		}
	}

	actualReaper.Status.Progress = reaperapi.ReaperProgressRunning
	actualReaper.Status.SetReady()

	logger.Info("Reaper successfully reconciled")
	return ctrl.Result{}, nil
}

func (r *ReaperReconciler) reconcileDatacenter(
	ctx context.Context,
	actualReaper *reaperapi.Reaper,
	logger logr.Logger,
) (*cassdcapi.CassandraDatacenter, ctrl.Result, error) {
	dcNamespace := actualReaper.Spec.DatacenterRef.Namespace
	if dcNamespace == "" {
		dcNamespace = actualReaper.Namespace
	}
	dcKey := client.ObjectKey{Namespace: dcNamespace, Name: actualReaper.Spec.DatacenterRef.Name}
	logger = logger.WithValues("CassandraDatacenter", dcKey)
	logger.Info("Fetching CassandraDatacenter resource")
	actualDc := &cassdcapi.CassandraDatacenter{}
	if err := r.Get(ctx, dcKey, actualDc); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Waiting for datacenter to be created")
			return nil, ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
		} else {
			logger.Error(err, "Failed to fetch CassandraDatacenter")
			return nil, ctrl.Result{}, err
		}
	}
	actualDc = actualDc.DeepCopy()
	if !cassandra.DatacenterReady(actualDc) {
		logger.Info("Waiting for datacenter to become ready")
		return nil, ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
	}
	return actualDc, ctrl.Result{}, nil
}

func (r *ReaperReconciler) reconcileDeployment(
	ctx context.Context,
	actualReaper *reaperapi.Reaper,
	actualDc *cassdcapi.CassandraDatacenter,
	logger logr.Logger,
) (ctrl.Result, error) {

	deploymentKey := types.NamespacedName{Namespace: actualReaper.Namespace, Name: actualReaper.Name}
	logger = logger.WithValues("Deployment", deploymentKey)
	logger.Info(fmt.Sprintf("Reconciling reaper deployment, req was %#v", actualReaper))

	authVars, err := r.collectAuthVars(ctx, actualReaper, logger)
	if err != nil {
		logger.Error(err, "Failed to collect Reaper auth variables")
		return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
	}
	logger.Info("Collected Reaper auth variables", "authVars", authVars)

	var keystorePassword *string
	var truststorePassword *string

	if actualReaper.Spec.ClientEncryptionStores != nil && !actualReaper.Spec.UseExternalSecrets() {
		if password, err := cassandra.ReadEncryptionStorePassword(ctx, actualReaper.Namespace, r.Client, actualReaper.Spec.ClientEncryptionStores, encryption.StoreNameKeystore); err != nil {
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
		} else {
			keystorePassword = ptr.To(password)
		}

		if password, err := cassandra.ReadEncryptionStorePassword(ctx, actualReaper.Namespace, r.Client, actualReaper.Spec.ClientEncryptionStores, encryption.StoreNameTruststore); err != nil {
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
		} else {
			truststorePassword = ptr.To(password)
		}
	}

	// reconcile Vector configmap
	// do this only if we're not a control plane reaper. a CP reaper has no vector agent with it
	if actualReaper.Spec.DatacenterRef.Name != "" {
		if vectorReconcileResult, err := r.reconcileVectorConfigMap(ctx, *actualReaper, actualDc, r.Client, logger); err != nil {
			return vectorReconcileResult, err
		} else if vectorReconcileResult.Requeue {
			return vectorReconcileResult, nil
		}
	}

	logger.Info("Reconciling reaper deployment", "actualReaper", actualReaper)

	// work out how to deploy Reaper
	actualDeployment, err := reaper.MakeActualDeploymentType(actualReaper)
	if err != nil {
		return ctrl.Result{}, err
	}
	desiredDeployment, err := reaper.MakeDesiredDeploymentType(actualReaper, actualDc, keystorePassword, truststorePassword, logger, authVars...)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Get(ctx, deploymentKey, actualDeployment); err != nil {
		if errors.IsNotFound(err) {
			if err = controllerutil.SetControllerReference(actualReaper, desiredDeployment, r.Scheme); err != nil {
				logger.Error(err, "Failed to set owner on Reaper Deployment")
				return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
			} else if err = r.Create(ctx, desiredDeployment); err != nil {
				if errors.IsAlreadyExists(err) {
					// the read from the local cache didn't catch that the resource was created
					// already; simply requeue until the cache is up-to-date
					return ctrl.Result{Requeue: true}, nil
				} else {
					logger.Error(err, "Failed to create Reaper Deployment")
					return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
				}
			}
			logger.Info("Reaper Deployment created successfully")
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
		} else {
			logger.Error(err, "Failed to get Reaper Deployment")
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
		}
	}

	if err = r.reconcileReaperTelemetry(ctx, actualReaper, logger, r.Client); err != nil {
		logger.Error(err, "reconcileReaperTelemetry failed")
		return ctrl.Result{}, err
	}

	actualDeployment, err = reaper.DeepCopyActualDeployment(actualDeployment)
	if err != nil {
		return ctrl.Result{}, err
	}

	// if using memory storage, we need to ensure only one Reaper exists ~ the STS has at most 1 replica
	if actualReaper.Spec.StorageType == reaperapi.StorageTypeLocal {
		var desiredReplicas int32
		if desiredReplicas, err = getDeploymentReplicas(desiredDeployment); err != nil {
			return ctrl.Result{}, err
		}
		if desiredReplicas > 1 {
			logger.Info(fmt.Sprintf("reaper with memory storage can only have one replica, not allowing the %d that are desired", desiredReplicas))
			if err = forceSingleReplica(&desiredDeployment); err != nil {
				return ctrl.Result{}, err
			}
		}
		var actualReplicas int32
		if actualReplicas, err = getDeploymentReplicas(actualDeployment); err != nil {
			return ctrl.Result{}, err
		}
		if actualReplicas > 1 {
			logger.Info(fmt.Sprintf("reaper with memory storage currently has %d replicas, scaling down to 1", actualReplicas))
			if err = forceSingleReplica(&desiredDeployment); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// Check if the deployment needs to be updated
	if !annotations.CompareHashAnnotations(actualDeployment, desiredDeployment) {
		logger.Info("Updating Reaper Deployment")
		resourceVersion := actualDeployment.GetResourceVersion()

		// the DeepCopyInto is called on an actual Deployment or STS, can't easily refactor that out
		switch desired := desiredDeployment.(type) {
		case *appsv1.Deployment:
			desired.DeepCopyInto(actualDeployment.(*appsv1.Deployment))
		case *appsv1.StatefulSet:
			desired.DeepCopyInto(actualDeployment.(*appsv1.StatefulSet))
		default:
			err := fmt.Errorf("unexpected type %T", desiredDeployment)
			return ctrl.Result{}, err
		}

		actualDeployment.SetResourceVersion(resourceVersion)
		if err := controllerutil.SetControllerReference(actualReaper, actualDeployment, r.Scheme); err != nil {
			logger.Error(err, "Failed to set controller reference on updated Reaper Deployment")
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
		} else if err := r.Update(ctx, actualDeployment); err != nil {
			logger.Error(err, "Failed to update Reaper Deployment")
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
		} else {
			logger.Info("Reaper Deployment updated successfully")
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
		}
	}

	logger.Info("Reaper Deployment ready")
	return ctrl.Result{}, nil
}

func getDeploymentReplicas(actualDeployment client.Object) (int32, error) {
	switch actual := actualDeployment.(type) {
	case *appsv1.Deployment:
		return *actual.Spec.Replicas, nil
	case *appsv1.StatefulSet:
		return *actual.Spec.Replicas, nil
	default:
		err := fmt.Errorf("failed to get replicas of unexpected deployment type type %T", actualDeployment)
		return math.MaxInt32, err
	}
}

func forceSingleReplica(desiredDeployment *client.Object) error {
	switch desired := (*desiredDeployment).(type) {
	case *appsv1.Deployment:
		desired.Spec.Replicas = ptr.To[int32](1)
	case *appsv1.StatefulSet:
		desired.Spec.Replicas = ptr.To[int32](1)
	default:
		err := fmt.Errorf("failed to set replicas for deployment of unexpected type %T", desiredDeployment)
		return err
	}
	return nil
}

func (r *ReaperReconciler) reconcileService(
	ctx context.Context,
	actualReaper *reaperapi.Reaper,
	logger logr.Logger,
) (ctrl.Result, error) {
	serviceKey := types.NamespacedName{Namespace: actualReaper.Namespace, Name: reaper.GetServiceName(actualReaper.Name)}
	logger = logger.WithValues("Service", serviceKey)
	logger.Info("Reconciling Reaper Service")
	desiredService := reaper.NewService(serviceKey, actualReaper)
	actualService := &corev1.Service{}
	if err := r.Client.Get(ctx, serviceKey, actualService); err != nil {
		if errors.IsNotFound(err) {
			// create the service
			if err = controllerutil.SetControllerReference(actualReaper, desiredService, r.Scheme); err != nil {
				logger.Error(err, "Failed to set controller reference on Reaper Service")
				return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
			}
			logger.Info("Creating Reaper service")
			if err = r.Client.Create(ctx, desiredService); err != nil {
				if errors.IsAlreadyExists(err) {
					// the read from the local cache didn't catch that the resource was created
					// already; simply requeue until the cache is up-to-date
					return ctrl.Result{Requeue: true}, nil
				} else {
					logger.Error(err, "Failed to create Reaper Service")
					return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
				}
			}
			logger.Info("Reaper Service created successfully")
			return ctrl.Result{}, nil
		} else {
			logger.Error(err, "Failed to get Reaper Service")
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
		}
	}
	if !annotations.CompareHashAnnotations(actualService, desiredService) {
		logger.Info("Updating Reaper Service")
		updatedService := actualService.DeepCopy()
		desiredService.DeepCopyInto(updatedService)
		updatedService.SetResourceVersion(actualService.GetResourceVersion())
		updatedService.Spec.ClusterIP = actualService.Spec.ClusterIP
		updatedService.Spec.ClusterIPs = actualService.Spec.ClusterIPs
		if err := controllerutil.SetControllerReference(actualReaper, updatedService, r.Scheme); err != nil {
			logger.Error(err, "Failed to set controller reference on updated Reaper Service")
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
		} else if err := r.Update(ctx, updatedService); err != nil {
			logger.Error(err, "Failed to update Reaper Service")
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
		} else {
			logger.Info("Reaper Service updated successfully")
			return ctrl.Result{}, nil
		}
	}
	logger.Info("Reaper Service is ready")
	return ctrl.Result{}, nil
}

func (r *ReaperReconciler) configureReaper(ctx context.Context, actualReaper *reaperapi.Reaper, actualDc *cassdcapi.CassandraDatacenter, logger logr.Logger) (ctrl.Result, error) {
	manager := r.NewManager()
	manager.SetK8sClient(r)
	// Get the Reaper UI secret username and password values if auth is enabled
	if username, password, err := manager.GetUiCredentials(ctx, actualReaper.Spec.UiUserSecretRef, actualReaper.Namespace); err != nil {
		return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
	} else {
		if err := manager.Connect(ctx, actualReaper, username, password); err != nil {
			logger.Error(err, "failed to connect to reaper")
			logger.Info("Reaper doesn't seem to be running yet")
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
		} else if found, err := manager.VerifyClusterIsConfigured(ctx, actualDc); err != nil {
			logger.Info("failed to verify the cluster is registered with reaper. Maybe reaper is still starting up.")
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
		} else if !found {
			logger.Info("registering cluster with reaper")
			if err = manager.AddClusterToReaper(ctx, actualDc); err != nil {
				logger.Error(err, "failed to register cluster with reaper")
				return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
			}
		}
	}
	return ctrl.Result{}, nil
}

func (r *ReaperReconciler) collectAuthVars(ctx context.Context, actualReaper *reaperapi.Reaper, logger logr.Logger) ([]*corev1.EnvVar, error) {
	cqlVars, err := r.collectAuthVarsForType(ctx, actualReaper, logger, "cql")
	if err != nil {
		return nil, err
	}
	jmxVars, err := r.collectAuthVarsForType(ctx, actualReaper, logger, "jmx")
	if err != nil {
		return nil, err
	}
	uiVars, err := r.collectAuthVarsForType(ctx, actualReaper, logger, "ui")
	if err != nil {
		return nil, err
	}

	if len(uiVars) == 0 {
		// if there are no ui vars, we need to disable auth in the reaper UI
		uiVars = []*corev1.EnvVar{reaper.DisableAuthVar}
	}

	authVars := append(cqlVars, jmxVars...)
	authVars = append(authVars, uiVars...)
	return authVars, nil
}

func (r *ReaperReconciler) collectAuthVarsForType(ctx context.Context, actualReaper *reaperapi.Reaper, logger logr.Logger, authType string) ([]*corev1.EnvVar, error) {
	var secretRef *corev1.LocalObjectReference
	var envVars []*corev1.EnvVar
	switch authType {
	case "cql":
		secretRef = &actualReaper.Spec.CassandraUserSecretRef
		envVars = []*corev1.EnvVar{reaper.EnableCassAuthVar}
	case "jmx":
		// JMX auth is based on the CQL role, so reuse the same secret (JmxUserSecretRef is deprecated)
		secretRef = &actualReaper.Spec.CassandraUserSecretRef
		envVars = []*corev1.EnvVar{}
	case "ui":
		secretRef = actualReaper.Spec.UiUserSecretRef
		envVars = []*corev1.EnvVar{reaper.EnableAuthVar}
	}

	if secretRef != nil && len(secretRef.Name) > 0 && !actualReaper.Spec.UseExternalSecrets() {
		secretKey := types.NamespacedName{Namespace: actualReaper.Namespace, Name: secretRef.Name}
		if secret, err := r.getSecret(ctx, secretKey); err != nil {
			logger.Error(err, "Failed to get Cassandra authentication secret", authType, secretKey)
			return nil, err
		} else if usernameEnvVar, passwordEnvVar, err := reaper.GetAuthEnvironmentVars(secret, authType); err != nil {
			logger.Error(err, "Failed to get Cassandra authentication env vars", authType, secretKey)
			return nil, err
		} else {
			logger.Info("Found authentication secret", authType, secretKey.Name)
			return append(envVars, usernameEnvVar, passwordEnvVar), nil
		}
	}
	logger.Info("No authentication secret found", "authType", authType)
	return nil, nil
}

func (r *ReaperReconciler) getSecret(ctx context.Context, secretKey types.NamespacedName) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := r.Get(ctx, secretKey, secret)
	return secret, err
}

func (r *ReaperReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&reaperapi.Reaper{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
