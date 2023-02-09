/*
Copyright 2021.

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

package stargate

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reconciliation"
	"github.com/k8ssandra/k8ssandra-operator/pkg/stargate"
	stargateutil "github.com/k8ssandra/k8ssandra-operator/pkg/stargate"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	api "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
)

// +kubebuilder:rbac:groups=stargate.k8ssandra.io,namespace="k8ssandra",resources=stargates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=stargate.k8ssandra.io,namespace="k8ssandra",resources=stargates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=stargate.k8ssandra.io,namespace="k8ssandra",resources=stargates/finalizers,verbs=update
// +kubebuilder:rbac:groups=cassandra.datastax.com,namespace="k8ssandra",resources=cassandradatacenters,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,namespace="k8ssandra",resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,namespace="k8ssandra",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,namespace="k8ssandra",resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete;deletecollection

// StargateReconciler reconciles a Stargate object
type StargateReconciler struct {
	*config.ReconcilerConfig
	client.Client
	Scheme        *runtime.Scheme
	ManagementApi cassandra.ManagementApiFactory
}

func (r *StargateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Stargate resource
	logger.Info("Fetching Stargate resource", "Stargate", req.NamespacedName)
	stargate := &api.Stargate{}
	if err := r.Get(ctx, req.NamespacedName, stargate); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Stargate resource not found", "Stargate", req.NamespacedName)
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, err
		}
	}
	stargate = stargate.DeepCopy()

	// Set initial status
	if stargate.Status.Progress == "" {
		ratio := fmt.Sprintf("0/%v", stargate.Spec.Size)
		now := metav1.Now()
		stargate.Status = api.StargateStatus{
			Conditions: []api.StargateCondition{{
				Type:               api.StargateReady,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: &now,
			}},
			Progress:           api.StargateProgressPending,
			ReadyReplicasRatio: &ratio,
		}
		if err := r.Status().Update(ctx, stargate); err != nil {
			logger.Error(err, "Failed to update Stargate status", "Stargate", req.NamespacedName)
			return ctrl.Result{}, err
		}
	}

	// Fetch the target CassandraDatacenter resource
	actualDc := &cassdcapi.CassandraDatacenter{}
	dcKey := client.ObjectKey{Namespace: req.Namespace, Name: stargate.Spec.DatacenterRef.Name}
	logger.Info("Fetching CassandraDatacenter resource", "CassandraDatacenter", dcKey)
	if err := r.Get(ctx, dcKey, actualDc); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Waiting for datacenter to be created", "CassandraDatacenter", dcKey)
			if stargate.Status.Progress != api.StargateProgressPending {
				stargate.Status.Progress = api.StargateProgressPending
				if err := r.Status().Update(ctx, stargate); err != nil {
					logger.Error(err, "Failed to update Stargate status", "Stargate", req.NamespacedName)
					return ctrl.Result{}, err
				}
			}
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
		} else {
			logger.Error(err, "Failed to fetch CassandraDatacenter", "CassandraDatacenter", dcKey)
			return ctrl.Result{}, err
		}
	}
	actualDc = actualDc.DeepCopy()

	// Wait until the DC is ready
	if !cassandra.DatacenterReady(actualDc) {
		if stargate.Status.Progress != api.StargateProgressPending {
			stargate.Status.Progress = api.StargateProgressPending
			if err := r.Status().Update(ctx, stargate); err != nil {
				logger.Error(err, "Failed to update Stargate status", "Stargate", req.NamespacedName)
				return ctrl.Result{}, err
			}
		}
		logger.Info("Waiting for datacenter to become ready", "CassandraDatacenter", dcKey)
		return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
	}

	// if a configmap is specified, we need to read its content to merge it with the generated one
	userStargateCassandraYaml := ""
	userStargateCqlYaml := ""
	if stargate.Spec.CassandraConfigMapRef != nil {
		userConfigMap := &corev1.ConfigMap{}
		configMapKey := types.NamespacedName{Namespace: req.Namespace, Name: stargate.Spec.CassandraConfigMapRef.Name}
		err := r.Get(ctx, configMapKey, userConfigMap)
		if err != nil {
			return ctrl.Result{}, err
		}
		userStargateCassandraYaml = userConfigMap.Data["cassandra.yaml"]
		userStargateCqlYaml = userConfigMap.Data[stargateutil.CqlConfigName]
	}

	logger.Info("Reconciling Stargate configmap")
	// Reconcile the Stargate cassandra-config configmap using the desiredConfig content marshalled to yaml
	var dcConfig map[string]interface{}
	if actualDc.Spec.Config != nil {
		err := json.Unmarshal(actualDc.Spec.Config, &dcConfig)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if stargateConfigResult, err := r.reconcileStargateConfigMap(ctx, stargate, dcConfig, userStargateCassandraYaml, userStargateCqlYaml, req.Namespace, *actualDc, logger); err != nil {
		return ctrl.Result{}, err
	} else {
		if stargateConfigResult.Requeue {
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
		}
	}

	racks := len(actualDc.GetRacks())
	if int(stargate.Spec.Size) < racks {
		logger.Info(
			fmt.Sprintf(
				"Stargate size (%v) is lesser than the number of racks (%v): some racks won't have any Stargate pod",
				stargate.Spec.Size,
				racks,
			),
			"Stargate", req.NamespacedName)
	} else if int(stargate.Spec.Size)%racks != 0 {
		logger.Info(
			fmt.Sprintf(
				"Stargate size (%v) cannot be evenly distributed across %v racks: some racks will have more Stargate pods than others",
				stargate.Spec.Size,
				racks,
			),
			"Stargate", req.NamespacedName)
	}

	if !labels.IsOwnedByK8ssandraController(stargate) {
		logger.Info("Stargate resource is standalone, reconciling auth schema now")
		managementApi, err := r.ManagementApi.NewManagementApiFacade(ctx, actualDc, r.Client, logger)
		if err != nil {
			logger.Error(err, "Failed to create ManagementApiFacade")
			return ctrl.Result{}, err
		} else if err = stargateutil.ReconcileAuthKeyspace(managementApi, cassandra.ComputeReplication(3, actualDc), logger); err != nil {
			return ctrl.Result{}, err
		}
	}

	// reconcile Vector configmap
	if vectorReconcileResult, err := r.reconcileVectorConfigMap(ctx, *stargate, actualDc, r.Client, logger); err != nil {
		return vectorReconcileResult, err
	} else if vectorReconcileResult.Requeue {
		return vectorReconcileResult, nil
	}

	// Compute the desired deployments
	desiredDeployments := stargateutil.NewDeployments(stargate, actualDc, logger)

	// Transition status from Created/Pending to Deploying
	if stargate.Status.Progress == api.StargateProgressPending {
		stargate.Status.Progress = api.StargateProgressDeploying
		stargate.Status.DeploymentRefs = make([]string, 0)
		for _, deployment := range desiredDeployments {
			stargate.Status.DeploymentRefs = append(stargate.Status.DeploymentRefs, deployment.Name)
		}
		if err := r.Status().Update(ctx, stargate); err != nil {
			logger.Error(err, "Failed to update Stargate status", "Stargate", req.NamespacedName)
			return ctrl.Result{}, err
		}
	}

	var replicas int32 = 0
	var readyReplicas int32 = 0
	var updatedReplicas int32 = 0
	var availableReplicas int32 = 0

	actualDeployments := &appsv1.DeploymentList{}
	if err := r.List(
		ctx,
		actualDeployments,
		client.InNamespace(req.Namespace),
		client.MatchingLabels(labels.MapOf(labels.StargateName(stargate.Name))),
	); err != nil {
		logger.Error(err, "Failed to list Stargate Deployments", "Stargate", req.NamespacedName)
		return ctrl.Result{}, err
	}

	// NOTE: Desired deployments need to be updated prior to this point. Once this code path is reached,
	// any changes to desired deployments may not be applied as they will be deleted below if no changes
	// are detected.
	for _, actualDeployment := range actualDeployments.Items {
		deploymentKey := client.ObjectKey{Namespace: req.Namespace, Name: actualDeployment.Name}
		if desiredDeployment, found := desiredDeployments[actualDeployment.Name]; !found {
			// Deployment exists but is not desired anymore: delete it
			logger.Info("Deleting Stargate Deployment", "Deployment", deploymentKey)
			if err := r.Delete(ctx, &actualDeployment); err != nil {
				logger.Error(err, "Failed to delete Stargate Deployment", "Deployment", deploymentKey)
				return ctrl.Result{}, err
			} else {
				logger.Info("Stargate Deployment deleted successfully", "Deployment", deploymentKey)
				return ctrl.Result{RequeueAfter: r.ReconcilerConfig.LongDelay}, nil
			}
		} else {
			// Deployment already exists: check if it needs to be updated
			if !annotations.CompareHashAnnotations(&desiredDeployment, &actualDeployment) {
				logger.Info("Updating Stargate Deployment", "Deployment", deploymentKey)
				resourceVersion := actualDeployment.GetResourceVersion()
				desiredDeployment.DeepCopyInto(&actualDeployment)
				actualDeployment.SetResourceVersion(resourceVersion)
				// Set Stargate instance as the owner and controller
				if err := ctrl.SetControllerReference(stargate, &actualDeployment, r.Scheme); err != nil {
					logger.Error(err, "Failed to set controller reference on updated Stargate Deployment", "Deployment", deploymentKey)
					return ctrl.Result{}, err
				} else if err := r.Update(ctx, &actualDeployment); err != nil {
					logger.Error(err, "Failed to update Stargate Deployment", "Deployment", deploymentKey)
					return ctrl.Result{}, err
				} else {
					logger.Info("Stargate Deployment updated successfully", "Deployment", deploymentKey)
					return ctrl.Result{RequeueAfter: r.ReconcilerConfig.LongDelay}, nil
				}
			}
			logger.Info("Deleting Stargate desired deployment", "Deployment", actualDeployment.Name)
			delete(desiredDeployments, actualDeployment.Name)
			replicas += actualDeployment.Status.Replicas
			readyReplicas += actualDeployment.Status.ReadyReplicas
			updatedReplicas += actualDeployment.Status.UpdatedReplicas
			availableReplicas += actualDeployment.Status.AvailableReplicas
		}
	}

	for _, desiredDeployment := range desiredDeployments {
		// Deployment does not exist yet: create a new one
		deploymentKey := client.ObjectKey{Namespace: req.Namespace, Name: desiredDeployment.Name}
		logger.Info("Stargate Deployment not found, creating a new one", "Deployment", deploymentKey)
		// Set Stargate instance as the owner and controller
		if err := ctrl.SetControllerReference(stargate, &desiredDeployment, r.Scheme); err != nil {
			logger.Error(err, "Failed to set controller reference on new Stargate Deployment", "Deployment", deploymentKey)
			return ctrl.Result{}, err
		} else if err := r.Create(ctx, &desiredDeployment); err != nil {
			if errors.IsAlreadyExists(err) {
				// the read from the local cache didn't catch that the resource was created
				// already; simply requeue until the cache is up-to-date
				return ctrl.Result{Requeue: true}, nil
			} else {
				logger.Error(err, "Failed to create new Stargate Deployment", "Deployment", deploymentKey)
				return ctrl.Result{}, err
			}
		} else {
			logger.Info("Stargate Deployment created successfully", "Deployment", deploymentKey)
			return ctrl.Result{RequeueAfter: r.ReconcilerConfig.LongDelay}, nil
		}
	}

	_, err := r.reconcileStargateTelemetry(ctx, stargate, logger, r.Client)
	if err != nil {
		logger.Error(err, "reconcileStargateTelemetry failed")
		return ctrl.Result{}, err
	}

	// Update status to reflect deployment status
	if stargate.Status.Replicas != replicas ||
		stargate.Status.ReadyReplicas != readyReplicas ||
		stargate.Status.UpdatedReplicas != updatedReplicas ||
		stargate.Status.AvailableReplicas != availableReplicas {
		ratio := fmt.Sprintf("%v/%v", readyReplicas, stargate.Spec.Size)
		stargate.Status.ReadyReplicasRatio = &ratio
		stargate.Status.Replicas = replicas
		stargate.Status.ReadyReplicas = readyReplicas
		stargate.Status.UpdatedReplicas = updatedReplicas
		stargate.Status.AvailableReplicas = availableReplicas
		if err := r.Status().Update(ctx, stargate); err != nil {
			logger.Error(err, "Failed to update Stargate status", "Stargate", req.NamespacedName)
			return ctrl.Result{}, err
		}
	}

	// Wait until all deployments are rolled out
	if readyReplicas != stargate.Spec.Size {
		// Transition status back to "Deploying" if it was "Running"
		if stargate.Status.Progress != api.StargateProgressDeploying {
			stargate.Status.Progress = api.StargateProgressDeploying
			if err := r.Status().Update(ctx, stargate); err != nil {
				logger.Error(err, "Failed to update Stargate status", "Stargate", req.NamespacedName)
				return ctrl.Result{}, err
			}
		}
		logger.Info("Waiting for deployments to be rolled out", "Stargate", req.NamespacedName)
		return ctrl.Result{RequeueAfter: r.ReconcilerConfig.DefaultDelay}, nil
	}

	// Compute the desired service
	desiredService := stargateutil.NewService(stargate, actualDc)

	// Check if a service already exists, if not create a new one
	serviceKey := client.ObjectKey{Namespace: req.Namespace, Name: desiredService.Name}
	actualService := &corev1.Service{}
	logger.Info("Fetching Stargate Service", "Service", serviceKey)
	if err := r.Get(ctx, serviceKey, actualService); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Stargate Service not found, creating a new one", "Service", serviceKey)
			// Set Stargate instance as the owner and controller
			if err := ctrl.SetControllerReference(stargate, desiredService, r.Scheme); err != nil {
				logger.Error(err, "Failed to set controller reference on new Stargate Service", "Service", serviceKey)
				return ctrl.Result{}, err
			} else if err := r.Create(ctx, desiredService); err != nil {
				if errors.IsAlreadyExists(err) {
					// the read from the local cache didn't catch that the resource was created
					// already; simply requeue until the cache is up-to-date
					return ctrl.Result{Requeue: true}, nil
				} else {
					logger.Error(err, "Failed to create new Stargate Service", "Service", serviceKey)
					return ctrl.Result{}, err
				}
			} else {
				logger.Info("Stargate Service created successfully", "Service", serviceKey)
				return ctrl.Result{RequeueAfter: r.ReconcilerConfig.DefaultDelay}, nil
			}
		} else {
			logger.Error(err, "Failed to fetch Stargate Service", "Service", serviceKey)
			return ctrl.Result{}, err
		}
	}

	// Check if the service needs to be updated
	if !annotations.CompareHashAnnotations(desiredService, actualService) {
		logger.Info("Updating Stargate Service", "Service", serviceKey)
		resourceVersion := actualService.GetResourceVersion()
		desiredService.DeepCopyInto(actualService)
		actualService.SetResourceVersion(resourceVersion)
		// Set Stargate instance as the owner and controller
		if err := ctrl.SetControllerReference(stargate, actualService, r.Scheme); err != nil {
			logger.Error(err, "Failed to set controller reference on updated Stargate Service", "Service", serviceKey)
			return ctrl.Result{}, err
		} else if err := r.Update(ctx, actualService); err != nil {
			logger.Error(err, "Failed to update Stargate Service", "Service", serviceKey)
			return ctrl.Result{}, err
		} else {
			logger.Info("Stargate Service updated successfully", "Service", serviceKey)
			return ctrl.Result{RequeueAfter: r.ReconcilerConfig.LongDelay}, nil
		}
	}

	// Transition status to Running
	if stargate.Status.Progress != api.StargateProgressRunning {
		stargate.Status.Progress = api.StargateProgressRunning
		stargate.Status.ServiceRef = &actualService.Name
		now := metav1.Now()
		stargate.Status.SetCondition(api.StargateCondition{
			Type:               api.StargateReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: &now,
		})
		if err := r.Status().Update(ctx, stargate); err != nil {
			logger.Error(err, "Failed to update Stargate status", "Stargate", req.NamespacedName)
			return ctrl.Result{}, err
		}
	}

	logger.Info("Stargate successfully reconciled", "Stargate", req.NamespacedName)
	return ctrl.Result{}, nil
}

func (r *StargateReconciler) reconcileStargateConfigMap(
	ctx context.Context,
	stargateObject *api.Stargate,
	dcConfig map[string]interface{},
	userCassandraYaml, userCqlYaml string,
	namespace string,
	dc cassdcapi.CassandraDatacenter,
	logger logr.Logger,
) (ctrl.Result, error) {
	logger.Info(fmt.Sprintf("Reconciling Stargate Cassandra yaml configMap on namespace %s for cluster %s and dc %s", namespace, dc.Spec.ClusterName, dc.Name))

	var cassandraYaml, cqlYaml string
	var err error
	if cassandraYaml, err = reconcileStargateConfigFile(userCassandraYaml, dcConfig, stargate.CassandraYamlRetainedSettings); err != nil {
		return ctrl.Result{}, err
	}
	if cqlYaml, err = reconcileStargateConfigFile(userCqlYaml, dcConfig, stargate.CqlYamlRetainedSettings); err != nil {
		return ctrl.Result{}, err
	}
	if len(cqlYaml) == 0 {
		// The Stargate code fails on an empty file. Replace with this which is valid YAML:
		cqlYaml = "{}"
	}

	configMapKey := client.ObjectKey{
		Namespace: namespace,
		Name:      stargate.GeneratedConfigMapName(dc.Spec.ClusterName, dc.Name),
	}

	logger = logger.WithValues("StargateConfigMap", configMapKey)
	desiredConfigMap := stargate.CreateStargateConfigMap(namespace, cassandraYaml, cqlYaml, dc)
	// Compute a hash which will allow to compare desired and actual configMaps
	annotations.AddHashAnnotation(desiredConfigMap)
	if err = controllerutil.SetControllerReference(stargateObject, desiredConfigMap, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	recRes := reconciliation.ReconcileObject(ctx, r.Client, r.DefaultDelay, *desiredConfigMap)
	switch {
	case recRes.IsError():
		return ctrl.Result{}, recRes.GetError()
	case recRes.IsRequeue():
		return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
	}
	logger.Info("Stargate ConfigMap successfully reconciled")
	return ctrl.Result{}, nil
}

// reconcileStargateConfigFile builds a Stargate config file from two different sources: userConfig is the user-provided
// config passed via cassandraConfigMapRef in the Stargate spec; dcConfig is the DC-level config from the
// CassandraDatacenter spec, of which we'll only keep retainedOptions.
func reconcileStargateConfigFile(
	userConfig string, dcConfig map[string]interface{}, retainedOptions []string,
) (string, error) {
	var dcYaml map[string]interface{}
	dcFullYaml, exists := dcConfig["cassandra-yaml"]
	if exists {
		dcYaml = stargate.FilterConfig(dcFullYaml.(map[string]interface{}), retainedOptions)
	}
	dcYamlString := ""
	if len(dcYaml) > 0 {
		if out, err := yaml.Marshal(dcYaml); err != nil {
			return "", err
		} else {
			dcYamlString = string(out)
		}
	}
	return stargate.MergeYamlString(userConfig, dcYamlString), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StargateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.Stargate{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
