package medusa

import (
	"context"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"testing"
	"time"

	medusav1alpha1 "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type FakeClock struct {
	currentTime time.Time
}

func (f *FakeClock) Now() time.Time {
	return f.currentTime
}

var _ Clock = &FakeClock{}

func TestScheduler(t *testing.T) {
	require := require.New(t)
	require.NoError(medusav1alpha1.AddToScheme(scheme.Scheme))
	require.NoError(cassdcapi.AddToScheme(scheme.Scheme))

	fClock := &FakeClock{}

	dc := cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dc1",
			Namespace: "test-ns",
		},
		Spec: cassdcapi.CassandraDatacenterSpec{},
	}

	// To manipulate time and requeue, we use fakeclient here instead of envtest
	backupSchedule := &medusav1alpha1.MedusaBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-schedule",
			Namespace: "test-ns",
		},
		Spec: medusav1alpha1.MedusaBackupScheduleSpec{
			CronSchedule: "* * * * *",
			BackupSpec: medusav1alpha1.MedusaBackupJobSpec{
				CassandraDatacenter: "dc1",
				Type:                "differential",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithObjects(backupSchedule, &dc).
		WithStatusSubresource(backupSchedule).
		WithScheme(scheme.Scheme).
		Build()

	nsName := types.NamespacedName{
		Name:      backupSchedule.Name,
		Namespace: backupSchedule.Namespace,
	}

	r := &MedusaBackupScheduleReconciler{
		Client: fakeClient,
		Scheme: scheme.Scheme,
		Clock:  fClock,
	}

	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nsName})
	require.NoError(err)
	require.True(res.RequeueAfter > 0)

	fClock.currentTime = fClock.currentTime.Add(1 * time.Minute).Add(1 * time.Second)

	_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nsName})
	require.NoError(err)
	require.True(res.RequeueAfter > 0)

	// We should have a backup now..
	backupRequests := medusav1alpha1.MedusaBackupJobList{}
	err = fakeClient.List(context.TODO(), &backupRequests)
	require.NoError(err)
	require.Equal(1, len(backupRequests.Items))

	// Ensure the backup object is created correctly
	backup := backupRequests.Items[0]
	require.Equal(backupSchedule.Spec.BackupSpec.CassandraDatacenter, backup.Spec.CassandraDatacenter)
	require.Equal(backupSchedule.Spec.BackupSpec.Type, backup.Spec.Type)

	// Verify the Status of the BackupSchedule is modified and the object is requeued
	backupScheduleLive := &medusav1alpha1.MedusaBackupSchedule{}
	err = fakeClient.Get(context.TODO(), nsName, backupScheduleLive)
	require.NoError(err)

	require.Equal(fClock.currentTime, backupScheduleLive.Status.LastExecution.Time.UTC())
	require.Equal(time.Time{}.Add(2*time.Minute), backupScheduleLive.Status.NextSchedule.Time.UTC())

	// Test that next invocation also works
	fClock.currentTime = fClock.currentTime.Add(1 * time.Minute)

	_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nsName})
	require.NoError(err)
	require.True(res.RequeueAfter > 0)

	// We should not have more than 1, since we never set the previous one as finished
	backupRequests = medusav1alpha1.MedusaBackupJobList{}
	err = fakeClient.List(context.TODO(), &backupRequests)
	require.NoError(err)
	require.Equal(1, len(backupRequests.Items))

	// Mark the first one as finished and try again
	backup.Status.FinishTime = metav1.NewTime(fClock.currentTime)
	require.NoError(fakeClient.Update(context.TODO(), &backup))

	backupRequests = medusav1alpha1.MedusaBackupJobList{}
	err = fakeClient.List(context.TODO(), &backupRequests)
	require.NoError(err)
	require.Equal(1, len(backupRequests.Items))

	_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nsName})
	require.NoError(err)
	require.True(res.RequeueAfter > 0)

	backupRequests = medusav1alpha1.MedusaBackupJobList{}
	err = fakeClient.List(context.TODO(), &backupRequests)
	require.NoError(err)
	require.Equal(2, len(backupRequests.Items))

	for _, backup := range backupRequests.Items {
		backup.Status.FinishTime = metav1.NewTime(fClock.currentTime)
		require.NoError(fakeClient.Update(context.TODO(), &backup))
	}

	// Verify that invocating again without reaching the next time does not generate another backup
	// or modify the Status
	backupScheduleLive = &medusav1alpha1.MedusaBackupSchedule{}
	err = fakeClient.Get(context.TODO(), nsName, backupScheduleLive)
	require.NoError(err)

	previousExecutionTime := backupScheduleLive.Status.LastExecution
	fClock.currentTime = fClock.currentTime.Add(30 * time.Second)
	_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nsName})
	require.NoError(err)
	require.True(res.RequeueAfter > 0)

	backupRequests = medusav1alpha1.MedusaBackupJobList{}
	err = fakeClient.List(context.TODO(), &backupRequests)
	require.NoError(err)
	require.Equal(2, len(backupRequests.Items))

	backupScheduleLive = &medusav1alpha1.MedusaBackupSchedule{}
	err = fakeClient.Get(context.TODO(), nsName, backupScheduleLive)
	require.NoError(err)
	require.Equal(previousExecutionTime, backupScheduleLive.Status.LastExecution)

	// Set to disabled and verify that the backups aren't scheduled anymore - but the status is updated
	backupScheduleLive.Spec.Disabled = true
	err = fakeClient.Update(context.TODO(), backupScheduleLive)
	require.NoError(err)

	fClock.currentTime = fClock.currentTime.Add(1 * time.Minute)

	_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nsName})
	require.NoError(err)
	require.True(res.RequeueAfter > 0)

	backupRequests = medusav1alpha1.MedusaBackupJobList{}
	err = fakeClient.List(context.TODO(), &backupRequests)
	require.NoError(err)
	require.Equal(2, len(backupRequests.Items)) // No new items were created

	backupScheduleLive = &medusav1alpha1.MedusaBackupSchedule{}
	err = fakeClient.Get(context.TODO(), nsName, backupScheduleLive)
	require.NoError(err)
	require.True(previousExecutionTime.Before(&backupScheduleLive.Status.LastExecution)) // Status time is still updated
}

func TestPurgeScheduler(t *testing.T) {
	require := require.New(t)
	require.NoError(medusav1alpha1.AddToScheme(scheme.Scheme))
	require.NoError(cassdcapi.AddToScheme(scheme.Scheme))

	fClock := &FakeClock{}

	dc := cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dc1",
			Namespace: "test-ns",
		},
		Spec: cassdcapi.CassandraDatacenterSpec{},
	}

	// To manipulate time and requeue, we use fakeclient here instead of envtest
	purgeSchedule := &medusav1alpha1.MedusaBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-purge-schedule",
			Namespace: "test-ns",
		},
		Spec: medusav1alpha1.MedusaBackupScheduleSpec{
			CronSchedule: "* * * * *",
			BackupSpec: medusav1alpha1.MedusaBackupJobSpec{
				CassandraDatacenter: "dc1",
			},
			OperationType: "purge",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithObjects(purgeSchedule, &dc).
		WithStatusSubresource(purgeSchedule).
		WithScheme(scheme.Scheme).
		Build()

	nsName := types.NamespacedName{
		Name:      purgeSchedule.Name,
		Namespace: purgeSchedule.Namespace,
	}

	r := &MedusaBackupScheduleReconciler{
		Client: fakeClient,
		Scheme: scheme.Scheme,
		Clock:  fClock,
	}

	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nsName})
	require.NoError(err)
	require.True(res.RequeueAfter > 0)

	fClock.currentTime = fClock.currentTime.Add(1 * time.Minute).Add(1 * time.Second)

	_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nsName})
	require.NoError(err)
	require.True(res.RequeueAfter > 0)

	// We should have a backup now..
	purgeTasks := medusav1alpha1.MedusaTaskList{}
	err = fakeClient.List(context.TODO(), &purgeTasks)
	require.NoError(err)
	require.Equal(1, len(purgeTasks.Items))

	// Ensure the backup object is created correctly
	backup := purgeTasks.Items[0]
	require.Equal(purgeSchedule.Spec.BackupSpec.CassandraDatacenter, backup.Spec.CassandraDatacenter)

	// Verify the Status of the BackupSchedule is modified and the object is requeued
	backupScheduleLive := &medusav1alpha1.MedusaBackupSchedule{}
	err = fakeClient.Get(context.TODO(), nsName, backupScheduleLive)
	require.NoError(err)

	require.Equal(fClock.currentTime, backupScheduleLive.Status.LastExecution.Time.UTC())
	require.Equal(time.Time{}.Add(2*time.Minute), backupScheduleLive.Status.NextSchedule.Time.UTC())

	// Test that next invocation also works
	fClock.currentTime = fClock.currentTime.Add(1 * time.Minute)

	_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nsName})
	require.NoError(err)
	require.True(res.RequeueAfter > 0)

	// We should not have more than 1, since we never set the previous one as finished
	purgeTasks = medusav1alpha1.MedusaTaskList{}
	err = fakeClient.List(context.TODO(), &purgeTasks)
	require.NoError(err)
	require.Equal(1, len(purgeTasks.Items))

	// Mark the first one as finished and try again
	backup.Status.FinishTime = metav1.NewTime(fClock.currentTime)
	require.NoError(fakeClient.Update(context.TODO(), &backup))

	purgeTasks = medusav1alpha1.MedusaTaskList{}
	err = fakeClient.List(context.TODO(), &purgeTasks)
	require.NoError(err)
	require.Equal(1, len(purgeTasks.Items))

	_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nsName})
	require.NoError(err)
	require.True(res.RequeueAfter > 0)

	purgeTasks = medusav1alpha1.MedusaTaskList{}
	err = fakeClient.List(context.TODO(), &purgeTasks)
	require.NoError(err)
	require.Equal(2, len(purgeTasks.Items))

	for _, backup := range purgeTasks.Items {
		backup.Status.FinishTime = metav1.NewTime(fClock.currentTime)
		require.NoError(fakeClient.Update(context.TODO(), &backup))
	}

	// Verify that invocating again without reaching the next time does not generate another backup
	// or modify the Status
	backupScheduleLive = &medusav1alpha1.MedusaBackupSchedule{}
	err = fakeClient.Get(context.TODO(), nsName, backupScheduleLive)
	require.NoError(err)

	previousExecutionTime := backupScheduleLive.Status.LastExecution
	fClock.currentTime = fClock.currentTime.Add(30 * time.Second)
	_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nsName})
	require.NoError(err)
	require.True(res.RequeueAfter > 0)

	purgeTasks = medusav1alpha1.MedusaTaskList{}
	err = fakeClient.List(context.TODO(), &purgeTasks)
	require.NoError(err)
	require.Equal(2, len(purgeTasks.Items))

	backupScheduleLive = &medusav1alpha1.MedusaBackupSchedule{}
	err = fakeClient.Get(context.TODO(), nsName, backupScheduleLive)
	require.NoError(err)
	require.Equal(previousExecutionTime, backupScheduleLive.Status.LastExecution)

	// Set to disabled and verify that the backups aren't scheduled anymore - but the status is updated
	backupScheduleLive.Spec.Disabled = true
	err = fakeClient.Update(context.TODO(), backupScheduleLive)
	require.NoError(err)

	fClock.currentTime = fClock.currentTime.Add(1 * time.Minute)

	_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nsName})
	require.NoError(err)
	require.True(res.RequeueAfter > 0)

	purgeTasks = medusav1alpha1.MedusaTaskList{}
	err = fakeClient.List(context.TODO(), &purgeTasks)
	require.NoError(err)
	require.Equal(2, len(purgeTasks.Items)) // No new items were created

	backupScheduleLive = &medusav1alpha1.MedusaBackupSchedule{}
	err = fakeClient.Get(context.TODO(), nsName, backupScheduleLive)
	require.NoError(err)
	require.True(previousExecutionTime.Before(&backupScheduleLive.Status.LastExecution)) // Status time is still updated
}

func TestSchedulerParseError(t *testing.T) {
	require := require.New(t)

	// To manipulate time and requeue, we use fakeclient here instead of envtest
	backupSchedule := &medusav1alpha1.MedusaBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-schedule",
			Namespace: "test-ns",
		},
		Spec: medusav1alpha1.MedusaBackupScheduleSpec{
			CronSchedule: "* * *",
			BackupSpec: medusav1alpha1.MedusaBackupJobSpec{
				CassandraDatacenter: "dc1",
				Type:                "differential",
			},
		},
	}
	err := medusav1alpha1.AddToScheme(scheme.Scheme)
	require.NoError(err)

	fakeClient := fake.NewClientBuilder().
		WithRuntimeObjects(backupSchedule).
		WithScheme(scheme.Scheme).
		Build()

	fClock := &FakeClock{}

	r := &MedusaBackupScheduleReconciler{
		Client: fakeClient,
		Scheme: scheme.Scheme,
		Clock:  fClock,
	}

	nsName := types.NamespacedName{
		Name:      backupSchedule.Name,
		Namespace: backupSchedule.Namespace,
	}

	_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nsName})
	require.Error(err)
}
