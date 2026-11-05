package medusa

import (
	"context"
	"fmt"
	"testing"
	"time"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type backupJobUnitEnv struct {
	ctx     context.Context
	client  client.Client
	r       *MedusaBackupJobReconciler
	req     reconcile.Request
	factory *fakeMedusaClientFactory
}

func newBackupJobUnitEnv(t *testing.T, status api.MedusaBackupJobStatus, funcs *interceptor.Funcs, extraObjs ...client.Object) *backupJobUnitEnv {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, cassdcapi.AddToScheme(scheme))
	require.NoError(t, api.AddToScheme(scheme))

	dc := &cassdcapi.CassandraDatacenter{ObjectMeta: metav1.ObjectMeta{Name: "dc1", Namespace: "test", UID: "dc1"}}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: dc.Namespace, Labels: dc.GetDatacenterLabels()},
		Status:     corev1.PodStatus{PodIP: dc1PodPrefix + "1"},
	}
	job := &api.MedusaBackupJob{
		ObjectMeta: metav1.ObjectMeta{Name: successfulBackupName, Namespace: dc.Namespace, OwnerReferences: []metav1.OwnerReference{{APIVersion: cassdcapi.GroupVersion.String(), Kind: "CassandraDatacenter", Name: dc.Name, UID: dc.UID}}},
		Spec:       api.MedusaBackupJobSpec{CassandraDatacenter: dc.Name},
		Status:     status,
	}

	builder := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(append([]client.Object{dc, pod, job}, extraObjs...)...).
		WithStatusSubresource(&api.MedusaBackupJob{}, &api.MedusaBackup{})
	if funcs != nil {
		builder = builder.WithInterceptorFuncs(*funcs)
	}
	c := builder.Build()

	factory := NewMedusaClientFactory()
	medusaClient, err := factory.NewClient(fmt.Sprintf("%s:%d", pod.Status.PodIP, medusa.DefaultMedusaPort))
	require.NoError(t, err)
	medusaClient.(*fakeMedusaClient).RequestedBackups = []string{job.Name}

	return &backupJobUnitEnv{
		ctx:     context.Background(),
		client:  c,
		r:       &MedusaBackupJobReconciler{Client: c, Scheme: scheme, ReconcilerConfig: &config.ReconcilerConfig{DefaultDelay: time.Second}, ClientFactory: factory},
		req:     reconcile.Request{NamespacedName: client.ObjectKeyFromObject(job)},
		factory: factory,
	}
}

func (e *backupJobUnitEnv) job(t *testing.T) *api.MedusaBackupJob {
	job := &api.MedusaBackupJob{}
	require.NoError(t, e.client.Get(e.ctx, e.req.NamespacedName, job))
	return job
}

// An InProgress pod that no longer exists must be reported as failed, not silently dropped.
func TestMedusaBackupJobMissingInProgressPodIsFailed(t *testing.T) {
	env := newBackupJobUnitEnv(t, api.MedusaBackupJobStatus{
		StartTime:  metav1.Now(),
		InProgress: []string{"pod-that-is-gone"},
	}, nil)

	_, err := env.r.Reconcile(env.ctx, env.req)
	require.NoError(t, err)

	job := env.job(t)
	require.Empty(t, job.Status.InProgress)
	require.Equal(t, []string{"pod-that-is-gone"}, job.Status.Failed)

	_, err = env.r.Reconcile(env.ctx, env.req)
	require.NoError(t, err)
	require.False(t, env.job(t).Status.FinishTime.IsZero())

	err = env.client.Get(env.ctx, env.req.NamespacedName, &api.MedusaBackup{})
	require.Error(t, err, "no MedusaBackup must be created for a failed backup")
}

// If the status patch that records the started backups is lost, the job must check every pod again
// instead of finishing immediately.
func TestMedusaBackupJobLostStartPatch(t *testing.T) {
	env := newBackupJobUnitEnv(t, api.MedusaBackupJobStatus{StartTime: metav1.Now()}, nil)

	_, err := env.r.Reconcile(env.ctx, env.req)
	require.NoError(t, err)

	job := env.job(t)
	require.True(t, job.Status.FinishTime.IsZero())
	require.Equal(t, []string{"pod1"}, job.Status.InProgress)
}

// The MedusaBackupJob must not be marked finished until the MedusaBackup has been created with its status,
// so that a failure while writing it is retried.
func TestMedusaBackupJobRetriesMedusaBackupCreation(t *testing.T) {
	failNext := true
	funcs := &interceptor.Funcs{
		SubResourcePatch: func(ctx context.Context, c client.Client, subResourceName string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
			if _, ok := obj.(*api.MedusaBackup); ok && failNext {
				failNext = false
				return fmt.Errorf("injected status patch failure")
			}
			return c.SubResource(subResourceName).Patch(ctx, obj, patch, opts...)
		},
	}
	env := newBackupJobUnitEnv(t, api.MedusaBackupJobStatus{
		StartTime: metav1.Now(),
		Finished:  []string{"pod1"},
	}, funcs)

	_, err := env.r.Reconcile(env.ctx, env.req)
	require.Error(t, err)
	require.True(t, env.job(t).Status.FinishTime.IsZero(), "job must not be finished while the MedusaBackup status is missing")

	backup := &api.MedusaBackup{}
	require.NoError(t, env.client.Get(env.ctx, env.req.NamespacedName, backup))
	require.True(t, backup.Status.FinishTime.IsZero())

	_, err = env.r.Reconcile(env.ctx, env.req)
	require.NoError(t, err)
	require.False(t, env.job(t).Status.FinishTime.IsZero())

	require.NoError(t, env.client.Get(env.ctx, env.req.NamespacedName, backup))
	require.False(t, backup.Status.FinishTime.IsZero())
	require.Equal(t, int32(3), backup.Status.TotalNodes)
	require.Equal(t, medusa.StatusType_SUCCESS.String(), backup.Status.Status)
}
