/*
Copyright 2022.

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

package control

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	cassapi "github.com/k8ssandra/cass-operator/apis/control/v1alpha1"
	k8capi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/k8ssandra/k8ssandra-operator/apis/control/v1alpha1"
)

const (
	k8ssandraTaskFinalizer = "k8ssandratask.k8ssandra.io/finalizer"
	defaultTTL             = time.Duration(86400) * time.Second
)

var (
	noTTL int32 = 0
)

// K8ssandraTaskReconciler reconciles a K8ssandraTask object
type K8ssandraTaskReconciler struct {
	*config.ReconcilerConfig
	client.Client
	Scheme      *runtime.Scheme
	ClientCache *clientcache.ClientCache
}

//+kubebuilder:rbac:groups=control.k8ssandra.io,namespace="k8ssandra",resources=k8ssandratasks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=control.k8ssandra.io,namespace="k8ssandra",resources=k8ssandratasks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=control.k8ssandra.io,namespace="k8ssandra",resources=k8ssandratasks/finalizers,verbs=update

//+kubebuilder:rbac:groups=control.k8ssandra.io,namespace="k8ssandra",resources=cassandratasks,verbs=get;list;watch;delete
//+kubebuilder:rbac:groups=control.k8ssandra.io,namespace="k8ssandra",resources=cassandratasks/status,verbs=get

func (r *K8ssandraTaskReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("K8ssandraTask", req.NamespacedName)

	logger.Info("Fetching task", "K8ssandraTask", req.NamespacedName)
	k8Task := &api.K8ssandraTask{}
	if err := r.Get(ctx, req.NamespacedName, k8Task); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	kc, kcExists, err := r.getCluster(ctx, k8Task)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Set owner reference so that the task is deleted when the K8ssandraCluster is deleted.
	if k8Task.OwnerReferences == nil && kcExists {
		logger.Info("Setting owner reference", "K8ssandraTask", req.NamespacedName, "K8ssandraCluster", k8Task.GetClusterKey())
		if err := controllerutil.SetControllerReference(kc, k8Task, r.Scheme); err != nil {
			return ctrl.Result{}, fmt.Errorf("setting owner reference on K8ssandraTask: %s", err)
		}
		if err := r.Update(ctx, k8Task); err != nil {
			return ctrl.Result{}, fmt.Errorf("updating K8ssandraTask to set owner reference: %s", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Handle deletion
	if k8Task.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(k8Task, k8ssandraTaskFinalizer) {
			logger.Info("Adding finalizer", "K8ssandraTask", req.NamespacedName)
			controllerutil.AddFinalizer(k8Task, k8ssandraTaskFinalizer)
			if err := r.Update(ctx, k8Task); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	} else { // The task is being deleted
		if controllerutil.ContainsFinalizer(k8Task, k8ssandraTaskFinalizer) {
			// First time we've noticed the deletion, clean up dependents and remove the finalizer
			if kcExists {
				if err := r.deleteDcTasks(ctx, k8Task, kc, logger); err != nil {
					return ctrl.Result{}, err
				}
			}
			logger.Info("Removing finalizer", "K8ssandraTask", req.NamespacedName)
			controllerutil.RemoveFinalizer(k8Task, k8ssandraTaskFinalizer)
			if err := r.Update(ctx, k8Task); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Handle TTL
	if k8Task.Status.CompletionTime != nil {
		timeNow := metav1.Now()
		deletionTime := calculateDeletionTime(k8Task)
		if deletionTime.IsZero() { // No TTL set, we're done
			return ctrl.Result{}, nil
		}

		if deletionTime.Before(timeNow.Time) {
			logger.Info("deleting task due to expired TTL", "Request", req.NamespacedName)
			err := r.Delete(ctx, k8Task)
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		// Reschedule for later deletion
		nextRunTime := deletionTime.Sub(timeNow.Time)
		logger.Info("scheduling task deletion", "Request", req.NamespacedName, "Delay", nextRunTime)
		return ctrl.Result{RequeueAfter: nextRunTime}, nil
	}

	// Create subtasks and/or gather their status
	if !kcExists {
		return ctrl.Result{}, fmt.Errorf("unknown K8ssandraCluster %s", k8Task.GetClusterKey())
	}
	patch := client.MergeFrom(k8Task.DeepCopy())
	if dcs, err := filterDcs(kc, k8Task.Spec.Datacenters); err != nil {
		return ctrl.Result{}, err
	} else {
		for _, dc := range dcs {
			dcNamespace := utils.FirstNonEmptyString(dc.Meta.Namespace, kc.Namespace)
			desiredDcTask := newDcTask(k8Task, dcNamespace, dc.Meta.Name)

			remoteClient, err := r.ClientCache.GetRemoteClient(dc.K8sContext)
			if err != nil {
				return ctrl.Result{}, err
			}

			dcTaskKey := client.ObjectKeyFromObject(desiredDcTask)
			actualDcTask := &cassapi.CassandraTask{}
			if err = remoteClient.Get(ctx, dcTaskKey, actualDcTask); err != nil {
				if errors.IsNotFound(err) {
					logger.Info("Creating DC task", "CassandraTask", dcTaskKey)
					if err = remoteClient.Create(ctx, desiredDcTask); err != nil {
						return ctrl.Result{}, err
					}
				} else {
					return ctrl.Result{}, err
				}
			} else {
				logger.Info("DC task already exists, updating status", "CassandraTask", dcTaskKey)
				if k8Task.Status.Datacenters == nil {
					k8Task.Status.Datacenters = make(map[string]cassapi.CassandraTaskStatus)
				}
				k8Task.Status.Datacenters[dc.Meta.Name] = actualDcTask.Status
			}
		}
		wasComplete := k8Task.Status.CompletionTime != nil
		k8Task.BuildGlobalStatus()
		if err = r.Status().Patch(ctx, k8Task, patch); err != nil {
			return ctrl.Result{}, err
		}
		// If the status update set a completion time, we want to reconcile again in order to handle the TTL. But
		// because we configured GenerationChangedPredicate in SetupWithManager(), we ignore our own status updates.
		// Requeue manually just for this case.
		if !wasComplete && k8Task.Status.CompletionTime != nil {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{}, nil
	}
}

func (r *K8ssandraTaskReconciler) getCluster(ctx context.Context, k8Task *api.K8ssandraTask) (*k8capi.K8ssandraCluster, bool, error) {
	kc := &k8capi.K8ssandraCluster{}
	if err := r.Get(ctx, k8Task.GetClusterKey(), kc); errors.IsNotFound(err) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}
	return kc, true, nil
}

func (r *K8ssandraTaskReconciler) deleteDcTasks(ctx context.Context, k8Task *api.K8ssandraTask, kc *k8capi.K8ssandraCluster, logger logr.Logger) error {
	if dcs, err := filterDcs(kc, k8Task.Spec.Datacenters); err != nil {
		return err
	} else {
		for _, dc := range dcs {
			dcNamespace := utils.FirstNonEmptyString(dc.Meta.Namespace, kc.Namespace)
			actualDcTask := &cassapi.CassandraTask{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: dcNamespace,
					Name:      dcTaskName(k8Task, dc.Meta.Name),
				},
			}
			remoteClient, err := r.ClientCache.GetRemoteClient(dc.K8sContext)
			if err != nil {
				return err
			}
			logger.Info("Deleting DC task", "CassandraTask", actualDcTask)
			if err := remoteClient.Delete(ctx, actualDcTask); err != nil {
				if !errors.IsNotFound(err) {
					return fmt.Errorf("deleting CassandraTask %s.%s in context %s: %s",
						actualDcTask.ObjectMeta.Namespace,
						actualDcTask.ObjectMeta.Name,
						dc.K8sContext,
						err)
				}
			}
		}
		return nil
	}
}

func filterDcs(kc *k8capi.K8ssandraCluster, dcNames []string) ([]k8capi.CassandraDatacenterTemplate, error) {

	if len(dcNames) == 0 {
		return kc.Spec.Cassandra.Datacenters, nil
	}

	dcs := make([]k8capi.CassandraDatacenterTemplate, 0, len(dcNames))
	for _, dcName := range dcNames {
		found := false
		for _, dc := range kc.Spec.Cassandra.Datacenters {
			if dc.Meta.Name == dcName {
				dcs = append([]k8capi.CassandraDatacenterTemplate{dc}, dcs...)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown DC %s", dcName)
		}
	}
	return dcs, nil
}

func newDcTask(k8Task *api.K8ssandraTask, namespace string, dcName string) *cassapi.CassandraTask {
	dcTask := &cassapi.CassandraTask{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        dcTaskName(k8Task, dcName),
			Annotations: map[string]string{},
			Labels: map[string]string{
				k8capi.NameLabel:                k8capi.NameLabelValue,
				k8capi.PartOfLabel:              k8capi.PartOfLabelValue,
				k8capi.ComponentLabel:           k8capi.ComponentLabelValueCassandra,
				k8capi.CreatedByLabel:           k8capi.CreatedByLabelValueK8ssandraTaskController,
				api.K8ssandraTaskNameLabel:      k8Task.Name,
				api.K8ssandraTaskNamespaceLabel: k8Task.Namespace,
			},
		},

		Spec: cassapi.CassandraTaskSpec{
			Datacenter: corev1.ObjectReference{Namespace: namespace, Name: dcName},

			// TODO revisit this once k8ssandra/cass-operator#458 merged
			ScheduledTime:     k8Task.Spec.Template.ScheduledTime,
			Jobs:              k8Task.Spec.Template.Jobs,
			RestartPolicy:     k8Task.Spec.Template.RestartPolicy,
			ConcurrencyPolicy: k8Task.Spec.Template.ConcurrencyPolicy,

			// Never set the TTL here. We manage it ourselves on the K8ssandraTask, once it expires we'll cascade-delete
			// the CassandraTasks.
			TTLSecondsAfterFinished: &noTTL,
		},
	}
	return dcTask
}

func dcTaskName(k8Task *api.K8ssandraTask, dcName string) string {
	return k8Task.Name + "-" + dcName
}

func calculateDeletionTime(k8Task *api.K8ssandraTask) time.Time {
	if k8Task.Spec.Template.TTLSecondsAfterFinished != nil {
		if *k8Task.Spec.Template.TTLSecondsAfterFinished == 0 {
			return time.Time{}
		}
		return k8Task.Status.CompletionTime.Add(time.Duration(*k8Task.Spec.Template.TTLSecondsAfterFinished) * time.Second)
	}
	return k8Task.Status.CompletionTime.Add(defaultTTL)
}

// SetupWithManager sets up the controller with the Manager.
func (r *K8ssandraTaskReconciler) SetupWithManager(mgr ctrl.Manager, clusters []cluster.Cluster) error {
	cb := ctrl.NewControllerManagedBy(mgr).
		For(&api.K8ssandraTask{}, builder.WithPredicates(predicate.GenerationChangedPredicate{}))

	k8TaskLabelFilter := func(mapObj client.Object) []reconcile.Request {
		requests := make([]reconcile.Request, 0)

		taskName := labels.GetLabel(mapObj, api.K8ssandraTaskNameLabel)
		taskNamespace := labels.GetLabel(mapObj, api.K8ssandraTaskNamespaceLabel)

		if taskName != "" && taskNamespace != "" {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: taskNamespace, Name: taskName}})
		}
		return requests
	}

	cb = cb.Watches(&source.Kind{Type: &cassapi.CassandraTask{}},
		handler.EnqueueRequestsFromMapFunc(k8TaskLabelFilter))

	for _, c := range clusters {
		cb = cb.Watches(source.NewKindWithCache(&cassapi.CassandraTask{}, c.GetCache()),
			handler.EnqueueRequestsFromMapFunc(k8TaskLabelFilter))
	}

	return cb.Complete(r)
}
