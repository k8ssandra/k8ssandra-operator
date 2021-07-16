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

package controllers

import (
	"context"
	"fmt"
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
)

const DefaultStargateVersion = "1.0.30"

//+kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=stargates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=stargates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=stargates/finalizers,verbs=update
//+kubebuilder:rbac:groups=cassandra.datastax.com,namespace="k8ssandra",resources=cassandradatacenters,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,namespace="k8ssandra",resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,namespace="k8ssandra",resources=services,verbs=get;list;watch;create;update;patch;delete

// StargateReconciler reconciles a Stargate object
type StargateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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
			logger.Error(err, "Failed to fetch Stargate", "Stargate", req.NamespacedName)
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
	dcKey := client.ObjectKey{Namespace: req.Namespace, Name: stargate.Spec.DatacenterRef}
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
			return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
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
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	// Compute the desired deployment
	desiredDeployment := newStargateDeployment(stargate, actualDc)

	// Transition status from Created/Pending to Deploying
	if stargate.Status.Progress == api.StargateProgressPending {
		stargate.Status.Progress = api.StargateProgressDeploying
		stargate.Status.DeploymentRef = &desiredDeployment.Name
		if err := r.Status().Update(ctx, stargate); err != nil {
			logger.Error(err, "Failed to update Stargate status", "Stargate", req.NamespacedName)
			return ctrl.Result{}, err
		}
	}

	// Check if a deployment already exists, if not create a new one
	deploymentKey := client.ObjectKey{Namespace: req.Namespace, Name: desiredDeployment.Name}
	actualDeployment := &appsv1.Deployment{}
	logger.Info("Fetching Stargate Deployment", "Deployment", deploymentKey)
	if err := r.Get(ctx, deploymentKey, actualDeployment); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Stargate Deployment not found, creating a new one", "Deployment", deploymentKey)
			// Set Stargate instance as the owner and controller
			if err := ctrl.SetControllerReference(stargate, desiredDeployment, r.Scheme); err != nil {
				logger.Error(err, "Failed to set controller reference on new Stargate Deployment", "Deployment", deploymentKey)
				return ctrl.Result{}, err
			} else if err := r.Create(ctx, desiredDeployment); err != nil {
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
				return ctrl.Result{RequeueAfter: time.Minute}, nil
			}
		} else {
			logger.Error(err, "Failed to fetch Stargate Deployment", "Deployment", deploymentKey)
			return ctrl.Result{}, err
		}
	}

	// Check if the deployment needs to be updated
	desiredDeploymentHash := desiredDeployment.Annotations[resourceHashAnnotation]
	if actualDeploymentHash, found := actualDeployment.Annotations[resourceHashAnnotation]; !found || actualDeploymentHash != desiredDeploymentHash {
		logger.Info("Updating Stargate Deployment", "Deployment", deploymentKey)
		resourceVersion := actualDeployment.GetResourceVersion()
		desiredDeployment.DeepCopyInto(actualDeployment)
		actualDeployment.SetResourceVersion(resourceVersion)
		// Set Stargate instance as the owner and controller
		if err := ctrl.SetControllerReference(stargate, actualDeployment, r.Scheme); err != nil {
			logger.Error(err, "Failed to set controller reference on updated Stargate Deployment", "Deployment", deploymentKey)
			return ctrl.Result{}, err
		} else if err := r.Update(ctx, actualDeployment); err != nil {
			logger.Error(err, "Failed to update Stargate Deployment", "Deployment", deploymentKey)
			return ctrl.Result{}, err
		} else {
			logger.Info("Stargate Deployment updated successfully", "Deployment", deploymentKey)
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}
	}

	// Update status to reflect deployment status
	if stargate.Status.Replicas != actualDeployment.Status.Replicas ||
		stargate.Status.ReadyReplicas != actualDeployment.Status.ReadyReplicas ||
		stargate.Status.UpdatedReplicas != actualDeployment.Status.UpdatedReplicas ||
		stargate.Status.AvailableReplicas != actualDeployment.Status.AvailableReplicas {
		ratio := fmt.Sprintf("%v/%v", actualDeployment.Status.ReadyReplicas, stargate.Spec.Size)
		stargate.Status.ReadyReplicasRatio = &ratio
		stargate.Status.Replicas = actualDeployment.Status.Replicas
		stargate.Status.ReadyReplicas = actualDeployment.Status.ReadyReplicas
		stargate.Status.UpdatedReplicas = actualDeployment.Status.UpdatedReplicas
		stargate.Status.AvailableReplicas = actualDeployment.Status.AvailableReplicas
		if err := r.Status().Update(ctx, stargate); err != nil {
			logger.Error(err, "Failed to update Stargate status", "Stargate", req.NamespacedName)
			return ctrl.Result{}, err
		}
	}

	// Wait until the deployment is rolled out
	if actualDeployment.Status.ReadyReplicas != stargate.Spec.Size {
		// Transition status back to "Deploying" if it was "Running"
		if stargate.Status.Progress != api.StargateProgressDeploying {
			stargate.Status.Progress = api.StargateProgressDeploying
			if err := r.Status().Update(ctx, stargate); err != nil {
				logger.Error(err, "Failed to update Stargate status", "Stargate", req.NamespacedName)
				return ctrl.Result{}, err
			}
		}
		logger.Info("Waiting for deployment to be rolled out", "Deployment", deploymentKey)
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	// Compute the desired service
	desiredService := newStargateService(stargate, actualDc)

	// Check if a service already exists, if not create a new one
	serviceKey := client.ObjectKey{Namespace: req.Namespace, Name: desiredService.Name}
	actualService := &corev1.Service{}
	logger.Info("Fetching Stargate Service", "Service", serviceKey)
	if err := r.Get(ctx, serviceKey, actualService); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Stargate Service not found, creating a new one", "Service", serviceKey)
			// Set Stargate instance as the owner and controller
			if err := ctrl.SetControllerReference(stargate, desiredService, r.Scheme); err != nil {
				logger.Error(err, "Failed to set controller reference on new Stargate Service", "Deployment", deploymentKey)
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
				return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
			}
		} else {
			logger.Error(err, "Failed to fetch Stargate Service", "Service", serviceKey)
			return ctrl.Result{}, err
		}
	}

	// Check if the service needs to be updated
	desiredServiceHash := desiredService.Annotations[resourceHashAnnotation]
	if actualServiceHash, found := actualService.Annotations[resourceHashAnnotation]; !found || actualServiceHash != desiredServiceHash {
		logger.Info("Updating Stargate Service", "Service", serviceKey)
		resourceVersion := actualService.GetResourceVersion()
		desiredService.DeepCopyInto(actualService)
		actualService.SetResourceVersion(resourceVersion)
		// Set Stargate instance as the owner and controller
		if err := ctrl.SetControllerReference(stargate, actualService, r.Scheme); err != nil {
			logger.Error(err, "Failed to set controller reference on updated Stargate Service", "Deployment", deploymentKey)
			return ctrl.Result{}, err
		} else if err := r.Update(ctx, actualService); err != nil {
			logger.Error(err, "Failed to update Stargate Service", "Service", serviceKey)
			return ctrl.Result{}, err
		} else {
			logger.Info("Stargate Service updated successfully", "Service", serviceKey)
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}
	}

	// Transition status to Running
	if stargate.Status.Progress != api.StargateProgressRunning {
		stargate.Status.Progress = api.StargateProgressRunning
		stargate.Status.ServiceRef = &actualService.Name
		now := metav1.Now()
		stargate.Status.Conditions = []api.StargateCondition{{
			Type:               api.StargateReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: &now,
		}}
		if err := r.Status().Update(ctx, stargate); err != nil {
			logger.Error(err, "Failed to update Stargate status", "Stargate", req.NamespacedName)
			return ctrl.Result{}, err
		}
	}

	logger.Info("Stargate successfully reconciled", "Stargate", req.NamespacedName)
	return ctrl.Result{}, nil
}

// newStargateDeployment creates a Deployment object for the given Stargate and CassandraDatacenter
// resources.
func newStargateDeployment(stargate *api.Stargate, cassdc *cassdcapi.CassandraDatacenter) *appsv1.Deployment {

	cassandraVersion := cassdc.Spec.ServerVersion

	var clusterVersion string
	if strings.HasPrefix(cassandraVersion, "3") {
		clusterVersion = "3.11"
	} else {
		clusterVersion = "4.0"
	}

	var image string
	pullPolicy := corev1.PullIfNotPresent
	containerImage := stargate.Spec.StargateContainerImage
	if containerImage == nil {
		if clusterVersion == "3.11" {
			image = fmt.Sprintf("%s/%s:v%s", "stargateio", "stargate-3_11", DefaultStargateVersion)
		} else {
			image = fmt.Sprintf("%s/%s:v%s", "stargateio", "stargate-4_0", DefaultStargateVersion)
		}
	} else {
		if containerImage.Registry == nil {
			defaultRegistry := "docker.io"
			containerImage.Registry = &defaultRegistry
		}
		if containerImage.Tag == nil {
			defaultTag := "latest"
			containerImage.Tag = &defaultTag
		}
		image = fmt.Sprintf("%v/%v:%v", containerImage.Registry, containerImage.Repository, containerImage.Tag)
		if containerImage.PullPolicy != nil {
			pullPolicy = *containerImage.PullPolicy
		}
	}

	clusterName := cassdc.Spec.ClusterName
	// FIXME can this be customized? "{{ .Values.clusterDomain | default \"cluster.local\" }}
	clusterDomain := "cluster.local"

	dcName := cassdc.Name
	seedService := clusterName + "-seed-service." + cassdc.Namespace + ".svc." + clusterDomain

	deploymentName := cassdc.Spec.ClusterName + "-" + cassdc.Name + "-stargate-deployment"

	heapSize := stargate.Spec.HeapSize
	heapSizeInBytes := heapSize.Value()

	var resources *corev1.ResourceRequirements
	if stargate.Spec.Resources == nil {
		memoryRequest := heapSize.DeepCopy()
		memoryRequest.Add(memoryRequest) // heap x2
		memoryLimit := memoryRequest.DeepCopy()
		memoryLimit.Add(memoryLimit) // heap x4
		resources = &corev1.ResourceRequirements{
			Requests: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("200m"),
				corev1.ResourceMemory: memoryRequest,
			},
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("1000m"),
				corev1.ResourceMemory: memoryLimit,
			},
		}
	} else {
		resources = stargate.Spec.Resources
	}

	var livenessProbe *corev1.Probe
	if stargate.Spec.LivenessProbe == nil {
		livenessProbe = &corev1.Probe{
			TimeoutSeconds:      10,
			InitialDelaySeconds: 30,
			FailureThreshold:    5,
		}
	} else {
		livenessProbe = stargate.Spec.LivenessProbe
	}

	var readinessProbe *corev1.Probe
	if stargate.Spec.ReadinessProbe == nil {
		readinessProbe = &corev1.Probe{
			TimeoutSeconds:      10,
			InitialDelaySeconds: 30,
			FailureThreshold:    5,
		}
	} else {
		readinessProbe = stargate.Spec.ReadinessProbe
	}

	// The handlers cannot be user-specified, so force them now
	livenessProbe.Handler = corev1.Handler{
		HTTPGet: &corev1.HTTPGetAction{
			Path: "/checker/liveness",
			Port: intstr.FromString("health"),
		},
	}
	readinessProbe.Handler = corev1.Handler{
		HTTPGet: &corev1.HTTPGetAction{
			Path: "/checker/readiness",
			Port: intstr.FromString("health"),
		},
	}

	serviceAccountName := "default"
	if stargate.Spec.ServiceAccount != nil {
		serviceAccountName = *stargate.Spec.ServiceAccount
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{

			Name:        deploymentName,
			Namespace:   stargate.Namespace,
			Annotations: map[string]string{},
			Labels: map[string]string{
				api.StargateLabel: deploymentName,
			},
		},

		Spec: appsv1.DeploymentSpec{

			Replicas: &stargate.Spec.Size,

			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{api.StargateLabel: deploymentName},
			},

			Template: corev1.PodTemplateSpec{

				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{api.StargateLabel: deploymentName},
				},

				Spec: corev1.PodSpec{

					ServiceAccountName: serviceAccountName,

					Containers: []corev1.Container{{

						Name:            deploymentName,
						Image:           image,
						ImagePullPolicy: pullPolicy,

						Ports: []corev1.ContainerPort{
							{ContainerPort: 8080, Name: "graphql"},
							{ContainerPort: 8081, Name: "authorization"},
							{ContainerPort: 8082, Name: "rest"},
							{ContainerPort: 8084, Name: "health"},
							{ContainerPort: 8085, Name: "metrics"},
							{ContainerPort: 8090, Name: "http-schemaless"},
							{ContainerPort: 9042, Name: "native"},
							{ContainerPort: 8609, Name: "inter-node-msg"},
							{ContainerPort: 7000, Name: "intra-node"},
							{ContainerPort: 7001, Name: "tls-intra-node"},
						},

						Resources: *resources,

						Env: []corev1.EnvVar{
							{Name: "JAVA_OPTS", Value: fmt.Sprintf("-XX:+CrashOnOutOfMemoryError -Xms%v -Xmx%v", heapSizeInBytes, heapSizeInBytes)},
							{Name: "CLUSTER_NAME", Value: clusterName},
							{Name: "CLUSTER_VERSION", Value: clusterVersion},
							{Name: "SEED", Value: seedService},
							{Name: "DATACENTER_NAME", Value: dcName},
							// The rack name is temporarily hard coded until we get multi-rack support implemented.
							// See https://github.com/k8ssandra/k8ssandra/issues/54.
							{Name: "RACK_NAME", Value: "default"},
							{Name: "ENABLE_AUTH", Value: "true"},
						},

						LivenessProbe:  livenessProbe,
						ReadinessProbe: readinessProbe,
					}},

					Affinity:    stargate.Spec.Affinity,
					Tolerations: stargate.Spec.Tolerations,
				},
			},
		},
	}
	deployment.Annotations[resourceHashAnnotation] = deepHashString(deployment)
	return deployment
}

// newStargateService creates a Service object for the given Stargate and CassandraDatacenter
// resources.
func newStargateService(stargate *api.Stargate, cassdc *cassdcapi.CassandraDatacenter) *corev1.Service {
	serviceName := cassdc.Spec.ClusterName + "-" + cassdc.Name + "-stargate-service"
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        serviceName,
			Namespace:   stargate.Namespace,
			Annotations: map[string]string{},
			Labels: map[string]string{
				api.StargateLabel: serviceName,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{Port: 8080, Name: "graphql"},
				{Port: 8081, Name: "authorization"},
				{Port: 8082, Name: "rest"},
				{Port: 8084, Name: "health"},
				{Port: 8085, Name: "metrics"},
				{Port: 9042, Name: "cassandra"},
			},
			Selector: map[string]string{
				api.StargateLabel: serviceName,
			},
		},
	}
	service.Annotations[resourceHashAnnotation] = deepHashString(service)
	return service
}

// SetupWithManager sets up the controller with the Manager.
func (r *StargateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.Stargate{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
