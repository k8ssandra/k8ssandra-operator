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
	cassapi "github.com/k8ssandra/cass-operator/apis/control/v1alpha1"
	k8sapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/k8ssandra/k8ssandra-operator/apis/control/v1alpha1"
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

func (r *K8ssandraTaskReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("K8ssandraTask", req.NamespacedName)

	logger.Info("Fetching task", "K8ssandraTask", req.NamespacedName)
	k8Task := &api.K8ssandraTask{}
	err := r.Get(ctx, req.NamespacedName, k8Task)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{RequeueAfter: r.ReconcilerConfig.DefaultDelay}, err
	}
	// TODO k8Task.DeepCopy()? why?

	if k8Task.DeletionTimestamp != nil {
		// TODO delete dependent objects?
		return ctrl.Result{}, nil
	}

	kcKey := k8Task.GetClusterKey()
	logger.Info("Fetching cluster", "K8ssandraCluster", kcKey)
	kc := &k8sapi.K8ssandraCluster{}
	err = r.Get(ctx, kcKey, kc)
	if err != nil {
		return ctrl.Result{}, err
	}

	dcs, err := filterDcs(kc, k8Task.Spec.Datacenters)
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, dc := range dcs {
		dcNamespace := utils.FirstNonEmptyString(dc.Meta.Namespace, kc.Namespace)

		desiredDcTask := newDcTask(k8Task, dcNamespace, dc.Meta.Name)

		remoteClient, err := r.ClientCache.GetRemoteClient(dc.K8sContext)
		if err != nil {
			return ctrl.Result{}, err
		}

		dcTaskKey := client.ObjectKey{
			Namespace: desiredDcTask.Namespace,
			Name:      desiredDcTask.Name,
		}
		actualDcTask := &cassapi.CassandraTask{}
		err = remoteClient.Get(ctx, dcTaskKey, actualDcTask)
		if err != nil {
			if errors.IsNotFound(err) {
				logger.Info("Creating DC task", "CassandraTask", dcTaskKey)
				err = remoteClient.Create(ctx, desiredDcTask)
				if err != nil {
					return ctrl.Result{}, err
				}
			} else {
				return ctrl.Result{}, err
			}
		} else {
			if k8Task.Status.Datacenters == nil {
				k8Task.Status.Datacenters = make(map[string]cassapi.CassandraTaskStatus)
			}
			k8Task.Status.Datacenters[dc.Meta.Name] = actualDcTask.Status
		}

		k8Task.BuildGlobalStatus()
		err = r.Status().Update(ctx, k8Task)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func filterDcs(kc *k8sapi.K8ssandraCluster, dcNames []string) ([]k8sapi.CassandraDatacenterTemplate, error) {

	if len(dcNames) == 0 {
		return kc.Spec.Cassandra.Datacenters, nil
	}

	dcs := make([]k8sapi.CassandraDatacenterTemplate, 0, len(dcNames))
	for _, dcName := range dcNames {
		found := false
		for _, dc := range kc.Spec.Cassandra.Datacenters {
			if dc.Meta.Name == dcName {
				dcs = append([]k8sapi.CassandraDatacenterTemplate{dc}, dcs...)
				found = true
				break
			}
		}
		if !found {
			// TODO ignore instead? could be a race maybe
			return nil, fmt.Errorf("unknown DC %s", dcName)
		}
	}
	return dcs, nil
}

func newDcTask(k8Task *api.K8ssandraTask, namespace string, dcName string) *cassapi.CassandraTask {
	dcTask := &cassapi.CassandraTask{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        k8Task.Name + "-" + dcName,
			Annotations: map[string]string{},
			Labels: map[string]string{
				k8sapi.NameLabel:                k8sapi.NameLabelValue,
				k8sapi.PartOfLabel:              k8sapi.PartOfLabelValue,
				k8sapi.ComponentLabel:           k8sapi.ComponentLabelValueCassandra,
				k8sapi.CreatedByLabel:           k8sapi.CreatedByLabelValueK8ssandraTaskController,
				api.K8ssandraTaskNameLabel:      k8Task.Name,
				api.K8ssandraTaskNamespaceLabel: k8Task.Namespace,
			},
		},

		Spec: cassapi.CassandraTaskSpec{
			Datacenter: v1.ObjectReference{Namespace: namespace, Name: dcName},

			// TODO revisit this once k8ssandra/cass-operator#458 merged
			ScheduledTime:           k8Task.Spec.Template.ScheduledTime,
			Jobs:                    k8Task.Spec.Template.Jobs,
			RestartPolicy:           k8Task.Spec.Template.RestartPolicy,
			TTLSecondsAfterFinished: k8Task.Spec.Template.TTLSecondsAfterFinished,
			ConcurrencyPolicy:       k8Task.Spec.Template.ConcurrencyPolicy,
		},
	}
	return dcTask
}

// SetupWithManager sets up the controller with the Manager.
func (r *K8ssandraTaskReconciler) SetupWithManager(mgr ctrl.Manager, clusters []cluster.Cluster) error {
	cb := ctrl.NewControllerManagedBy(mgr).
		For(&api.K8ssandraTask{})

	clusterLabelFilter := func(mapObj client.Object) []reconcile.Request {
		requests := make([]reconcile.Request, 0)

		taskName := labels.GetLabel(mapObj, api.K8ssandraTaskNameLabel)
		taskNamespace := labels.GetLabel(mapObj, api.K8ssandraTaskNamespaceLabel)

		if taskName != "" && taskNamespace != "" {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: taskNamespace, Name: taskName}})
		}
		return requests
	}

	cb = cb.Watches(&source.Kind{Type: &cassapi.CassandraTask{}},
		handler.EnqueueRequestsFromMapFunc(clusterLabelFilter))

	for _, c := range clusters {
		cb = cb.Watches(source.NewKindWithCache(&cassapi.CassandraTask{}, c.GetCache()),
			handler.EnqueueRequestsFromMapFunc(clusterLabelFilter))
	}

	return cb.Complete(r)
}
