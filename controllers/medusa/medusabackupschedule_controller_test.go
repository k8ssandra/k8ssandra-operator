package medusa

import (
	"context"
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

	// To manipulate time and requeue, we use fakeclient here instead of envtest
	backupSchedule := &medusav1alpha1.MedusaBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-schedule",
			Namespace: "test-ns",
		},
		Spec: medusav1alpha1.MedusaBackupScheduleSpec{
			CronSchedule: "* * * * *",
			BackupSpec: medusav1alpha1.CassandraBackupSpec{
				Name:                "test-backup",
				CassandraDatacenter: "dc1",
				Type:                "differential",
			},
		},
	}
	medusav1alpha1.AddToScheme(scheme.Scheme)

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

	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nsName})
	require.NoError(err)
	require.True(res.RequeueAfter > 0)

	fClock.currentTime = fClock.currentTime.Add(1 * time.Minute).Add(1 * time.Second)

	_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nsName})
	require.NoError(err)
	require.True(res.RequeueAfter > 0)

	// We should have a backup now..
	backupRequests := medusav1alpha1.CassandraBackupList{}
	err = fakeClient.List(context.TODO(), &backupRequests)
	require.NoError(err)
	require.Equal(1, len(backupRequests.Items))

	// Ensure the backup object is created correctly
	backup := backupRequests.Items[0]
	require.Equal(backupSchedule.Spec.BackupSpec.CassandraDatacenter, backup.Spec.CassandraDatacenter)
	require.Equal(backupSchedule.Spec.BackupSpec.Name, backup.Spec.Name)
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

	backupRequests = medusav1alpha1.CassandraBackupList{}
	err = fakeClient.List(context.TODO(), &backupRequests)
	require.NoError(err)
	require.Equal(2, len(backupRequests.Items))

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

	backupRequests = medusav1alpha1.CassandraBackupList{}
	err = fakeClient.List(context.TODO(), &backupRequests)
	require.NoError(err)
	require.Equal(2, len(backupRequests.Items))

	backupScheduleLive = &medusav1alpha1.MedusaBackupSchedule{}
	err = fakeClient.Get(context.TODO(), nsName, backupScheduleLive)
	require.NoError(err)
	require.Equal(previousExecutionTime, backupScheduleLive.Status.LastExecution)
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
			BackupSpec: medusav1alpha1.CassandraBackupSpec{
				Name:                "test-backup",
				CassandraDatacenter: "dc1",
				Type:                "differential",
			},
		},
	}
	medusav1alpha1.AddToScheme(scheme.Scheme)

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

	_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nsName})
	require.Error(err)
}
