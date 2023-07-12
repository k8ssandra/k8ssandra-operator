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
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/k8ssandra/k8ssandra-operator/apis/control/v1alpha1"
)

const (
	k8ssandraTaskFinalizer = "control.k8ssandra.io/finalizer"
	defaultTTL             = time.Duration(86400) * time.Second
)

var (
	noTTL int32 = 0
)

// K8ssandraTaskReconciler reconciles a K8ssandraTask object:
//   - create a CassandraTask for each DC specified by the K8ssandraTask. Those will be picked up and executed by
//     cass-operator.
//   - watch the CassandraTasks (see SetupWithManager), and update the K8ssandraTask's status accordingly.
type K8ssandraTaskReconciler struct {
	*config.ReconcilerConfig
	client.Client
	Scheme      *runtime.Scheme
	ClientCache *clientcache.ClientCache
	Recorder    record.EventRecorder
}

//+kubebuilder:rbac:groups=control.k8ssandra.io,namespace="k8ssandra",resources=k8ssandratasks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=control.k8ssandra.io,namespace="k8ssandra",resources=k8ssandratasks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=control.k8ssandra.io,namespace="k8ssandra",resources=k8ssandratasks/finalizers,verbs=update

//+kubebuilder:rbac:groups=control.k8ssandra.io,namespace="k8ssandra",resources=cassandratasks,verbs=get;list;watch;delete
//+kubebuilder:rbac:groups=control.k8ssandra.io,namespace="k8ssandra",resources=cassandratasks/status,verbs=get

func (r *K8ssandraTaskReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("K8ssandraTask", req.NamespacedName)

	logger.Info("Fetching task")
	kTask := &api.K8ssandraTask{}
	if err := r.Get(ctx, req.NamespacedName, kTask); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	kcKey := kTask.GetClusterKey()
	kc, kcExists, err := r.getCluster(ctx, kcKey)
	if err != nil {
		return ctrl.Result{}, err
	}

	if kTask.OwnerReferences == nil && kcExists {
		// First time we've seen this task.
		logger.Info("Setting owner reference", "K8ssandraCluster", kcKey)
		if err := controllerutil.SetControllerReference(kc, kTask, r.Scheme); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "setting owner reference on K8ssandraTask")
		}
		logger.Info("Adding finalizer")
		controllerutil.AddFinalizer(kTask, k8ssandraTaskFinalizer)

		if err := r.Update(ctx, kTask); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if !kTask.ObjectMeta.DeletionTimestamp.IsZero() && controllerutil.ContainsFinalizer(kTask, k8ssandraTaskFinalizer) {
		// The task is getting deleted, and it's the first time we've noticed it.
		// Clean up dependents and remove the finalizer.
		if err := r.deleteCassandraTasks(ctx, kTask, kc, kcExists, logger); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("Removing finalizer")
		controllerutil.RemoveFinalizer(kTask, k8ssandraTaskFinalizer)
		if err := r.Update(ctx, kTask); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Handle TTL
	if !kTask.Status.CompletionTime.IsZero() {
		timeNow := metav1.Now()
		deletionTime := calculateDeletionTime(kTask)
		if deletionTime.IsZero() { // No TTL set, we're done
			return ctrl.Result{}, nil
		}

		if deletionTime.Before(timeNow.Time) {
			logger.Info("Deleting K8ssandraTask due to expired TTL")
			err := r.Delete(ctx, kTask)
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		// Reschedule for later deletion
		nextRunTime := deletionTime.Sub(timeNow.Time)
		logger.Info("Scheduling K8ssandraTask deletion", "Delay", nextRunTime)
		return ctrl.Result{RequeueAfter: nextRunTime}, nil
	}

	// Create CassandraTasks and/or gather their status
	if !kcExists {
		return r.reportInvalidSpec(ctx, kTask, "unknown K8ssandraCluster %s.%s", kcKey.Namespace, kcKey.Name)
	}
	if dcs, err := filterDcs(kc, kTask.Spec.Datacenters); err != nil {
		return r.reportInvalidSpec(ctx, kTask, err.Error())
	} else {
		for _, dc := range dcs {
			dcNamespace := utils.FirstNonEmptyString(dc.Meta.Namespace, kc.Namespace)
			desiredCTask := newCassandraTask(kTask, dcNamespace, dc.Meta.Name)

			remoteClient, err := r.ClientCache.GetRemoteClient(dc.K8sContext)
			if err != nil {
				return ctrl.Result{}, err
			}

			cTaskKey := client.ObjectKeyFromObject(desiredCTask)
			actualCTask := &cassapi.CassandraTask{}
			if err = remoteClient.Get(ctx, cTaskKey, actualCTask); err != nil {
				if k8serrors.IsNotFound(err) {
					actualCTask = desiredCTask
					if err = remoteClient.Create(ctx, actualCTask); err != nil {
						return ctrl.Result{}, err
					}
					r.recordCassandraTaskCreated(kTask, actualCTask, dc.K8sContext)
				} else {
					return ctrl.Result{}, err
				}
			} else {
				logger.Info("CassandraTask already exists, retrieving its status", "CassandraTask", cTaskKey)
				kTask.SetDcStatus(dc.Meta.Name, actualCTask.Status)
			}

			// If we're running sequentially, only continue creating tasks if this one is complete
			if kTask.Spec.DcConcurrencyPolicy != batchv1.AllowConcurrent && actualCTask.Status.CompletionTime.IsZero() {
				break
			}
		}
		kTask.RefreshGlobalStatus(len(dcs))
		if err = r.Status().Update(ctx, kTask); err != nil {
			if k8serrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err
		}
		// If the status update set a completion time, we want to reconcile again in order to handle the TTL. But
		// because we configured GenerationChangedPredicate in SetupWithManager(), we ignore our own status updates.
		// Requeue manually just for this case.
		if !kTask.Status.CompletionTime.IsZero() {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{}, nil
	}
}

func (r *K8ssandraTaskReconciler) getCluster(ctx context.Context, kcKey client.ObjectKey) (*k8capi.K8ssandraCluster, bool, error) {
	kc := &k8capi.K8ssandraCluster{}
	if err := r.Get(ctx, kcKey, kc); k8serrors.IsNotFound(err) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}
	return kc, true, nil
}

func (r *K8ssandraTaskReconciler) deleteCassandraTasks(
	ctx context.Context,
	kTask *api.K8ssandraTask,
	kc *k8capi.K8ssandraCluster,
	kcExists bool, logger logr.Logger,
) error {
	// If the K8ssandraCluster was deleted, the CassandraDatacenters and CassandraTasks will be deleted automatically.
	// If the spec was invalid, we didn't create the CassandraTasks in the first place.
	if !kcExists ||
		kTask.GetConditionStatus(api.JobInvalid) == corev1.ConditionTrue {
		return nil
	}

	logger.Info("Deleting CassandraTasks")
	if dcs, err := filterDcs(kc, kTask.Spec.Datacenters); err != nil {
		return err
	} else {
		for _, dc := range dcs {
			dcNamespace := utils.FirstNonEmptyString(dc.Meta.Namespace, kc.Namespace)
			actualCTask := &cassapi.CassandraTask{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: dcNamespace,
					Name:      cassandraTaskName(kTask, dc.Meta.Name),
				},
			}
			remoteClient, err := r.ClientCache.GetRemoteClient(dc.K8sContext)
			if err != nil {
				return err
			}
			logger.Info("Deleting CassandraTask", "CassandraTask", actualCTask)
			if err := remoteClient.Delete(ctx, actualCTask); err != nil {
				if !k8serrors.IsNotFound(err) {
					return errors.Wrapf(err, "deleting CassandraTask %s.%s in context %s",
						actualCTask.ObjectMeta.Namespace,
						actualCTask.ObjectMeta.Name,
						dc.K8sContext)
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
	var unknownDcNames []string = nil
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
			unknownDcNames = append(unknownDcNames, dcName)
		}
	}
	if len(unknownDcNames) > 0 {
		return nil, fmt.Errorf("unknown datacenters: %s", strings.Join(unknownDcNames, ", "))
	}
	return dcs, nil
}

func newCassandraTask(kTask *api.K8ssandraTask, namespace string, dcName string) *cassapi.CassandraTask {
	cTemplate := kTask.Spec.Template

	// Never set the TTL on the CassandraTask. We manage it ourselves on the K8ssandraTask, once it expires we'll
	// cascade-delete the dependent CassandraTasks.
	cTemplate.TTLSecondsAfterFinished = &noTTL

	return &cassapi.CassandraTask{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        cassandraTaskName(kTask, dcName),
			Annotations: map[string]string{},
			Labels: map[string]string{
				k8capi.NameLabel:                k8capi.NameLabelValue,
				k8capi.PartOfLabel:              k8capi.PartOfLabelValue,
				k8capi.ComponentLabel:           k8capi.ComponentLabelValueCassandra,
				api.K8ssandraTaskNameLabel:      kTask.Name,
				api.K8ssandraTaskNamespaceLabel: kTask.Namespace,
			},
		},
		Spec: cassapi.CassandraTaskSpec{
			Datacenter:            corev1.ObjectReference{Namespace: namespace, Name: dcName},
			CassandraTaskTemplate: cTemplate,
		},
	}
}

func cassandraTaskName(kTask *api.K8ssandraTask, dcName string) string {
	return kTask.Name + "-" + dcName
}

func calculateDeletionTime(kTask *api.K8ssandraTask) time.Time {
	if kTask.Spec.Template.TTLSecondsAfterFinished != nil {
		if *kTask.Spec.Template.TTLSecondsAfterFinished == 0 {
			return time.Time{}
		}
		return kTask.Status.CompletionTime.Add(time.Duration(*kTask.Spec.Template.TTLSecondsAfterFinished) * time.Second)
	}
	return kTask.Status.CompletionTime.Add(defaultTTL)
}

func (r *K8ssandraTaskReconciler) recordCassandraTaskCreated(
	kTask *api.K8ssandraTask,
	cTask *cassapi.CassandraTask,
	k8sContext string,
) {
	contextInfo := ""
	if k8sContext != "" {
		contextInfo = " in context " + k8sContext
	}
	r.Recorder.Event(kTask, "Normal", "CreateCassandraTask",
		fmt.Sprintf("Created CassandraTask %s.%s%s", cTask.Namespace, cTask.Name, contextInfo))
}

// reportInvalidSpec is called when the user provided an invalid K8ssandraTask spec. A warning event is emitted and the
// task's status is set to "Invalid".
func (r *K8ssandraTaskReconciler) reportInvalidSpec(
	ctx context.Context,
	kTask *api.K8ssandraTask,
	format string,
	arguments ...interface{},
) (ctrl.Result, error) {
	r.Recorder.Event(kTask, "Warning", "InvalidSpec", fmt.Sprintf(format, arguments...))

	patch := client.MergeFrom(kTask.DeepCopy())
	kTask.SetCondition(api.JobInvalid, corev1.ConditionTrue)
	err := r.Status().Patch(ctx, kTask, patch)
	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *K8ssandraTaskReconciler) SetupWithManager(mgr ctrl.Manager, clusters []cluster.Cluster) error {
	cb := ctrl.NewControllerManagedBy(mgr).
		For(&api.K8ssandraTask{}, builder.WithPredicates(predicate.GenerationChangedPredicate{}))

	kTaskLabelFilter := func(mapObj client.Object) []reconcile.Request {
		requests := make([]reconcile.Request, 0)

		taskName := labels.GetLabel(mapObj, api.K8ssandraTaskNameLabel)
		taskNamespace := labels.GetLabel(mapObj, api.K8ssandraTaskNamespaceLabel)

		if taskName != "" && taskNamespace != "" {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: taskNamespace, Name: taskName}})
		}
		return requests
	}

	cb = cb.Watches(&source.Kind{Type: &cassapi.CassandraTask{}},
		handler.EnqueueRequestsFromMapFunc(kTaskLabelFilter))

	for _, c := range clusters {
		cb = cb.Watches(source.NewKindWithCache(&cassapi.CassandraTask{}, c.GetCache()),
			handler.EnqueueRequestsFromMapFunc(kTaskLabelFilter))
	}

	return cb.Complete(r)
}
