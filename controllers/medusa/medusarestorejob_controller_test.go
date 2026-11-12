package medusa

import (
	"context"
	"encoding/json"
	goerrors "errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ss "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	"github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	restoredBackupName = "backup2"
)

// createRestoreTestCluster creates a K8ssandraCluster with Medusa and gRPC encryption, waits for its
// datacenter dc1 (Cassandra name real-dc1), marks it ready and creates its pods.
func createRestoreTestCluster(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) (*k8ss.K8ssandraCluster, framework.ClusterKey, *cassdcapi.CassandraDatacenter) {
	require := require.New(t)
	err := f.Client.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(namespace))
	require.NoError(err)
	k8sCtx0 := f.DataPlaneContexts[0]

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "medusa-client-secret",
		},
		Data: map[string][]byte{
			"ca.crt": []byte(`-----BEGIN CERTIFICATE-----
-----END CERTIFICATE-----`),
			"tls.crt": []byte(`-----BEGIN CERTIFICATE-----`),
			"tls.key": []byte(`-----BEGIN CERTIFICATE-----
-----END CERTIFICATE-----`),
		},
	}
	secretKey := framework.NewClusterKey(k8sCtx0, namespace, "medusa-client-secret")
	require.NoError(f.Create(ctx, secretKey, secret), "failed to create medusa-client-secret")
	t.Logf("Created secret: %s/%s", secret.Namespace, secret.Name)

	// The user secrets are refreshed at the end of the restore, so they must exist in the data plane.
	// The superuser secret is created later, to check that a failed refresh keeps the restore unfinished.
	require.NoError(createUserSecret(ctx, f, framework.NewClusterKey(k8sCtx0, namespace, cassandraUserSecret)), "failed to create Medusa user secret")

	kc := &k8ss.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test",
		},
		Spec: k8ss.K8ssandraClusterSpec{
			Cassandra: &k8ss.CassandraClusterTemplate{
				Datacenters: []k8ss.CassandraDatacenterTemplate{
					{
						Meta: k8ss.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: k8sCtx0,
						Size:       3,
						DatacenterOptions: k8ss.DatacenterOptions{
							DatacenterName: "real-dc1",
							ServerVersion:  "3.11.14",
							StorageConfig: &cassdcapi.StorageConfig{
								CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
									StorageClassName: &defaultStorageClass,
								},
							},
						},
					},
				},
			},
			Medusa: &api.MedusaClusterTemplate{
				ContainerImage: &images.Image{
					Repository: medusaImageRepo,
				},
				StorageProperties: api.Storage{
					StorageProvider: "s3_compatible",
					BucketName:      "not-real",
					StorageSecretRef: corev1.LocalObjectReference{
						Name: cassandraUserSecret,
					},
				},
				ServiceProperties: api.Service{
					GrpcPort: 4567,
					Encryption: &api.GRPCEncryption{
						ClientSecretName: "medusa-client-secret",
						ServerSecretName: "medusa-server-secret",
					},
				},
				CassandraUserSecretRef: corev1.LocalObjectReference{
					Name: cassandraUserSecret,
				},
			},
		},
	}

	t.Log("Creating k8ssandracluster with Medusa")
	require.NoError(f.Client.Create(ctx, kc), "failed to create K8ssandraCluster")

	reconcileReplicatedSecret(ctx, t, f, kc)
	t.Log("check that dc1 was created")
	dc1Key := framework.NewClusterKey(k8sCtx0, namespace, "dc1")
	require.Eventually(f.DatacenterExists(ctx, dc1Key), timeout, interval)

	t.Log("update datacenter status to scaling up")
	err = f.PatchDatacenterStatus(ctx, dc1Key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterScalingUp,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
		})
	})
	require.NoError(err, "failed to patch datacenter status")

	kcKey := framework.ClusterKey{K8sContext: k8sCtx0, NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test"}}

	t.Log("check that the K8ssandraCluster status is updated")
	require.Eventually(func() bool {
		kc := &k8ss.K8ssandraCluster{}
		err = f.Client.Get(ctx, kcKey.NamespacedName, kc)

		if err != nil {
			t.Logf("failed to get K8ssandraCluster: %v", err)
			return false
		}

		if len(kc.Status.Datacenters) == 0 {
			return false
		}

		k8ssandraStatus, found := kc.Status.Datacenters[dc1Key.Name]
		if !found {
			t.Logf("status for datacenter %s not found", dc1Key)
			return false
		}

		condition := findDatacenterScalingUpCondition(k8ssandraStatus.Cassandra)
		return condition != nil && condition.Status == corev1.ConditionTrue
	}, timeout, interval, "timed out waiting for K8ssandraCluster status update")

	t.Log("update dc1 status to ready")
	err = f.PatchDatacenterStatus(ctx, dc1Key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.Status.CassandraOperatorProgress = cassdcapi.ProgressReady
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
		})
	})
	require.NoError(err, "failed to update dc1 status to ready")

	dc1 := &cassdcapi.CassandraDatacenter{}
	require.NoError(f.Get(ctx, dc1Key, dc1))
	require.True(metav1.HasAnnotation(dc1.ObjectMeta, medusa.MedusaClientSecretNameAnnotation))
	require.Equal("medusa-client-secret", dc1.Annotations[medusa.MedusaClientSecretNameAnnotation])

	err = createCassandraPods(t, f, ctx, dc1Key, dc1)
	require.NoError(err)

	return kc, dc1Key, dc1
}

func testMedusaRestoreDatacenter(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	k8sCtx0 := f.DataPlaneContexts[0]
	kc, dc1Key, dc1 := createRestoreTestCluster(t, ctx, f, namespace)

	t.Log("creating MedusaBackup")
	backup := &api.MedusaBackup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      restoredBackupName,
		},
		Spec: api.MedusaBackupSpec{
			CassandraDatacenter: dc1.Name,
		},
	}

	backupKey := framework.NewClusterKey(dc1Key.K8sContext, dc1Key.Namespace, restoredBackupName)
	err := f.Create(ctx, backupKey, backup)
	require.NoError(err, "failed to create CassandraBackup")

	patch := client.MergeFrom(backup.DeepCopy())
	backup.Status.FinishTime = metav1.Now()
	backup.Status.FinishedNodes = dc1.Spec.Size
	backup.Status.TotalNodes = dc1.Spec.Size
	backup.Status.Nodes = []*api.MedusaBackupNode{
		{
			Datacenter: "real-dc1",
			Rack:       "default",
		},
		{
			Datacenter: "real-dc1",
			Rack:       "default",
		},
		{
			Datacenter: "real-dc1",
			Rack:       "default",
		},
	}

	err = f.PatchStatus(ctx, backup, patch, backupKey)
	require.NoError(err, "failed to patch MedusaBackup")

	restore := &api.MedusaRestoreJob{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test-restore",
		},
		Spec: api.MedusaRestoreJobSpec{
			Backup:              restoredBackupName,
			CassandraDatacenter: dc1.Name,
		},
	}

	restoreKey := framework.NewClusterKey(dc1Key.K8sContext, dc1Key.Namespace, restore.Name)
	err = f.Create(ctx, restoreKey, restore)
	require.NoError(err, "failed to create MedusaRestoreJob")

	withDc1 := f.NewWithDatacenter(ctx, dc1Key)

	t.Log("check that the datacenter is set to be stopped")
	require.Eventually(withDc1(func(dc *cassdcapi.CassandraDatacenter) bool {
		return dc.Spec.Stopped == true
	}), timeout*5, interval, "timed out waiting for CassandraDatacenter stopped flag to be set")

	t.Log("delete datacenter pods to simulate shutdown")
	err = f.DeleteAllOf(ctx, dc1Key.K8sContext, &corev1.Pod{}, client.InNamespace(namespace), client.MatchingLabels{cassdcapi.DatacenterLabel: "dc1"})
	require.NoError(err, "failed to delete datacenter pods")

	restore = &api.MedusaRestoreJob{}
	err = f.Get(ctx, restoreKey, restore)
	require.NoError(err, "failed to get MedusaRestoreJob")

	dcStoppedTime := restore.Status.StartTime.Add(1 * time.Second)

	t.Log("set datacenter status to stopped")
	err = f.PatchDatacenterStatus(ctx, dc1Key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterStopped,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(dcStoppedTime),
		})
	})
	require.NoError(err, "failed to update datacenter status with stopped condition")

	t.Log("check that the datacenter podTemplateSpec is updated")
	require.Eventually(withDc1(func(dc *cassdcapi.CassandraDatacenter) bool {
		restoreContainer := findRestoreContainer(dc.Spec.PodTemplateSpec.Spec.InitContainers)
		if restoreContainer == nil {
			t.Log("restore container not found")
			return false
		}

		envVar := utils.FindEnvVar(restoreContainer.Env, "BACKUP_NAME")
		if envVar == nil || envVar.Value != restoredBackupName {
			t.Logf("backup name not found in restore container: %v", restoreContainer.Env)
			return false
		}

		envVar = utils.FindEnvVar(restoreContainer.Env, "RESTORE_MAPPING")
		t.Logf("restore mapping: %v", envVar)
		if envVar == nil || envVar.Value == "" {
			t.Logf("restore mapping not foun d in restore container: %v", restoreContainer.Env)
			return false
		}

		envVar = utils.FindEnvVar(restoreContainer.Env, "RESTORE_KEY")
		t.Logf("restore key: %v", envVar)
		return envVar != nil
	}), timeout, interval, "timed out waiting for CassandraDatacenter PodTemplateSpec update")

	restore = &api.MedusaRestoreJob{}
	err = f.Get(ctx, restoreKey, restore)
	require.NoError(err, "failed to get MedusaRestoreJob")

	// In addition to checking Updating condition, the restore controller also checks the
	// PodTemplateSpec of the StatefulSets to make sure the update has been pushed down.
	// Note that this test does **not** verify the StatefulSet check. cass-operator creates
	// the StatefulSets. While we could create the StatefulSets in this test, it will be
	// easier/better to verify the StatefulSet checks in unit and e2e tests.
	t.Log("set datacenter status to updated")
	err = f.PatchDatacenterStatus(ctx, dc1Key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterUpdating,
			Status:             corev1.ConditionFalse,
			LastTransitionTime: metav1.NewTime(restore.Status.DatacenterStopped.Add(time.Second * 1)),
		})
	})
	require.NoError(err, "failed to update datacenter status with updating condition")

	dc := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc1Key, dc)
	require.NoError(err)

	restore = &api.MedusaRestoreJob{}
	err = f.Get(ctx, restoreKey, restore)
	require.NoError(err)

	t.Log("check datacenter restarted")
	require.Eventually(withDc1(func(dc *cassdcapi.CassandraDatacenter) bool {
		return !dc.Spec.Stopped
	}), timeout, interval)

	t.Log("set datacenter status to ready")
	err = f.PatchDatacenterStatus(ctx, dc1Key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.Status.CassandraOperatorProgress = cassdcapi.ProgressReady
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(dcStoppedTime.Add(time.Second * 2)),
		})
	})

	require.NoError(err)

	t.Log("check that the restore does not finish while the superuser secret cannot be refreshed")
	require.Never(func() bool {
		restore := &api.MedusaRestoreJob{}
		if err := f.Get(ctx, restoreKey, restore); err != nil {
			return false
		}
		return !restore.Status.FinishTime.IsZero()
	}, 1*time.Second, interval, "the restore finished before the user secrets were refreshed")

	superuserSecretKey := framework.NewClusterKey(k8sCtx0, namespace, "test-superuser")
	require.NoError(createUserSecret(ctx, f, superuserSecretKey), "failed to create superuser secret")

	t.Log("check restore status finish time set")
	require.Eventually(func() bool {
		restore := &api.MedusaRestoreJob{}
		err := f.Get(ctx, restoreKey, restore)
		if err != nil {
			return false
		}

		return !restore.Status.FinishTime.IsZero()
	}, timeout*2, interval)

	restore = &api.MedusaRestoreJob{}
	require.NoError(f.Get(ctx, restoreKey, restore))

	t.Log("check that the superuser secret was refreshed")
	superuserSecret := &corev1.Secret{}
	require.NoError(f.Get(ctx, superuserSecretKey, superuserSecret))
	require.Equal(restore.Status.StartTime.String(), superuserSecret.Annotations[k8ss.RefreshAnnotation])

	testFinishedRestoreJobDoesNotModifyDatacenter(t, ctx, f, dc1Key, restoreKey, backupKey, restore)

	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: dc.Namespace, Name: kc.Name}, timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")
}

// testFinishedRestoreJobDoesNotModifyDatacenter simulates what happens to a finished restore job on an
// operator restart: the datacenter has drifted since the restore (the restore env vars were removed by a
// later spec push and the datacenter was stopped) and the job is reconciled again. The datacenter must
// be left untouched.
func testFinishedRestoreJobDoesNotModifyDatacenter(t *testing.T, ctx context.Context, f *framework.Framework, dcKey, restoreKey, backupKey framework.ClusterKey, finishedRestore *api.MedusaRestoreJob) {
	require := require.New(t)

	t.Log("remove the restore env vars from the datacenter and stop it")
	dc := &cassdcapi.CassandraDatacenter{}
	require.NoError(f.Get(ctx, dcKey, dc))
	patch := client.MergeFrom(dc.DeepCopy())
	restoreContainer := &dc.Spec.PodTemplateSpec.Spec.InitContainers[findContainerIndex(dc.Spec.PodTemplateSpec.Spec.InitContainers, restoreContainerName)]
	env := make([]corev1.EnvVar, 0, len(restoreContainer.Env))
	for _, e := range restoreContainer.Env {
		if e.Name != backupNameEnvVar && e.Name != restoreKeyEnvVar && e.Name != restoreMappingEnvVar {
			env = append(env, e)
		}
	}
	restoreContainer.Env = env
	dc.Spec.Stopped = true
	require.NoError(f.Patch(ctx, dc, patch, dcKey))

	require.NoError(f.Get(ctx, dcKey, dc))
	expectedSpec := dc.Spec.DeepCopy()
	expectedResourceVersion := dc.ResourceVersion

	remoteClient, err := f.GetClient(dcKey.K8sContext)
	require.NoError(err)
	reconciler := &MedusaRestoreJobReconciler{
		ReconcilerConfig: &config.ReconcilerConfig{DefaultDelay: 50 * time.Millisecond, LongDelay: 150 * time.Millisecond},
		Client:           remoteClient,
		Scheme:           scheme.Scheme,
		ClientFactory:    NewMedusaClientRestoreFactory(),
		ImageRegistry:    getTestImageRegistry(),
	}

	t.Log("reconcile the finished restore job again")
	for i := 0; i < 3; i++ {
		res, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: restoreKey.NamespacedName})
		require.NoError(err)
		require.Equal(ctrl.Result{}, res)
	}

	t.Log("check that the datacenter was not modified")
	require.Never(func() bool {
		current := &cassdcapi.CassandraDatacenter{}
		if err := f.Get(ctx, dcKey, current); err != nil {
			t.Logf("failed to get datacenter: %v", err)
			return true
		}
		return current.ResourceVersion != expectedResourceVersion
	}, 1*time.Second, interval, "the finished restore job modified the datacenter")

	require.NoError(f.Get(ctx, dcKey, dc))
	require.Equal(*expectedSpec, dc.Spec)
	require.True(dc.Spec.Stopped, "the finished restore job started the datacenter")
	restoreContainer = findRestoreContainer(dc.Spec.PodTemplateSpec.Spec.InitContainers)
	require.NotNil(restoreContainer)
	require.Nil(utils.FindEnvVar(restoreContainer.Env, backupNameEnvVar))
	require.Nil(utils.FindEnvVar(restoreContainer.Env, restoreKeyEnvVar))
	require.Nil(utils.FindEnvVar(restoreContainer.Env, restoreMappingEnvVar))

	t.Log("check that the restore job status was not modified")
	restore := &api.MedusaRestoreJob{}
	require.NoError(f.Get(ctx, restoreKey, restore))
	require.Equal(finishedRestore.Status, restore.Status)

	t.Log("purge the MedusaBackup and check that reconciling the finished restore job does not fail")
	require.NoError(f.Delete(ctx, backupKey, &api.MedusaBackup{ObjectMeta: metav1.ObjectMeta{Namespace: backupKey.Namespace, Name: backupKey.Name}}))
	require.Eventually(func() bool {
		return errors.IsNotFound(f.Get(ctx, backupKey, &api.MedusaBackup{}))
	}, timeout, interval)
	res, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: restoreKey.NamespacedName})
	require.NoError(err)
	require.Equal(ctrl.Result{}, res)
}

func createUserSecret(ctx context.Context, f *framework.Framework, key framework.ClusterKey) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
		},
		Data: map[string][]byte{
			"username": []byte(key.Name),
			"password": []byte(key.Name),
		},
	}
	return f.Create(ctx, key, secret)
}

func findContainerIndex(containers []corev1.Container, name string) int {
	for i, c := range containers {
		if c.Name == name {
			return i
		}
	}
	return -1
}

func createCassandraPods(t *testing.T, f *framework.Framework, ctx context.Context, dcKey framework.ClusterKey, dc *cassdcapi.CassandraDatacenter) error {
	dcServiceKey := framework.NewClusterKey(dcKey.K8sContext, dcKey.Namespace, dc.GetAllPodsServiceName())
	dcService := &corev1.Service{}
	if err := f.Get(ctx, dcServiceKey, dcService); err != nil {
		if errors.IsNotFound(err) {
			dcService = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: dcServiceKey.Namespace,
					Name:      dcServiceKey.Name,
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						cassdcapi.ClusterLabel: cassdcapi.CleanLabelValue(dc.Spec.ClusterName),
					},
					Ports: []corev1.ServicePort{
						{
							Name: "cql",
							Port: 9042,
						},
					},
				},
			}

			err := f.Create(ctx, dcServiceKey, dcService)
			if err != nil {
				return err
			}
		} else {
			t.Errorf("failed to get service %s: %v", dcServiceKey, err)
			return err
		}
	}
	createDatacenterPods(t, f, ctx, dcKey, dc)
	return nil
}

func testValidationErrorStopsRestore(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	require.NoError(f.Client.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(namespace)))
	k8sCtx0 := f.DataPlaneContexts[0]

	kc := &k8ss.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test",
		},
		Spec: k8ss.K8ssandraClusterSpec{
			Cassandra: &k8ss.CassandraClusterTemplate{
				Datacenters: []k8ss.CassandraDatacenterTemplate{
					{
						Meta: k8ss.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: k8sCtx0,
						Size:       3,
						DatacenterOptions: k8ss.DatacenterOptions{
							DatacenterName: "real-dc1",
							ServerVersion:  "3.11.14",
							StorageConfig: &cassdcapi.StorageConfig{
								CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
									StorageClassName: &defaultStorageClass,
								},
							},
						},
					},
				},
			},
			Medusa: &api.MedusaClusterTemplate{
				ContainerImage: &images.Image{
					Repository: medusaImageRepo,
				},
				StorageProperties: api.Storage{
					StorageProvider: "s3_compatible",
					BucketName:      "not-real",
					StorageSecretRef: corev1.LocalObjectReference{
						Name: cassandraUserSecret,
					},
				},
				CassandraUserSecretRef: corev1.LocalObjectReference{
					Name: cassandraUserSecret,
				},
			},
		},
	}

	t.Log("Creating k8ssandracluster with Medusa")
	err := f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8ssandraCluster")

	reconcileReplicatedSecret(ctx, t, f, kc)
	t.Log("check that dc1 was created")
	dc1Key := framework.NewClusterKey(f.DataPlaneContexts[0], namespace, "dc1")
	require.Eventually(f.DatacenterExists(ctx, dc1Key), timeout, interval)

	t.Log("update datacenter status to scaling up")
	err = f.PatchDatacenterStatus(ctx, dc1Key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterScalingUp,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
		})
	})
	require.NoError(err, "failed to patch datacenter status")

	kcKey := framework.ClusterKey{K8sContext: k8sCtx0, NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test"}}

	t.Log("check that the K8ssandraCluster status is updated")
	require.Eventually(func() bool {
		kc := &k8ss.K8ssandraCluster{}
		err = f.Client.Get(ctx, kcKey.NamespacedName, kc)

		if err != nil {
			t.Logf("failed to get K8ssandraCluster: %v", err)
			return false
		}

		if len(kc.Status.Datacenters) == 0 {
			return false
		}

		k8ssandraStatus, found := kc.Status.Datacenters[dc1Key.Name]
		if !found {
			t.Logf("status for datacenter %s not found", dc1Key)
			return false
		}

		condition := findDatacenterScalingUpCondition(k8ssandraStatus.Cassandra)
		return condition != nil && condition.Status == corev1.ConditionTrue
	}, timeout, interval, "timed out waiting for K8ssandraCluster status update")

	dc1 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc1Key, dc1)

	t.Log("update dc1 status to ready")
	err = f.PatchDatacenterStatus(ctx, dc1Key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.Status.CassandraOperatorProgress = cassdcapi.ProgressReady
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
		})
	})
	require.NoError(err, "failed to update dc1 status to ready")

	t.Log("creating MedusaBackup")
	backup := &api.MedusaBackup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      restoredBackupName,
		},
		Spec: api.MedusaBackupSpec{
			CassandraDatacenter: dc1.Name,
		},
	}

	backupKey := framework.NewClusterKey(dc1Key.K8sContext, dc1Key.Namespace, restoredBackupName)
	err = f.Create(ctx, backupKey, backup)
	require.NoError(err, "failed to create CassandraBackup")

	patch := client.MergeFrom(backup.DeepCopy())
	backup.Status.FinishTime = metav1.Now()
	backup.Status.FinishedNodes = dc1.Spec.Size - 1
	backup.Status.TotalNodes = dc1.Spec.Size
	backup.Status.Nodes = []*api.MedusaBackupNode{
		{
			Datacenter: "dc1",
			Rack:       "default",
		},
		{
			Datacenter: "dc1",
			Rack:       "default",
		},
		{
			Datacenter: "dc1",
			Rack:       "default",
		},
	}

	err = f.PatchStatus(ctx, backup, patch, backupKey)
	require.NoError(err, "failed to patch MedusaBackup")

	restore := &api.MedusaRestoreJob{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test-restore",
		},
		Spec: api.MedusaRestoreJobSpec{
			Backup:              restoredBackupName,
			CassandraDatacenter: dc1.Name,
		},
	}

	restoreKey := framework.NewClusterKey(dc1Key.K8sContext, dc1Key.Namespace, restore.Name)
	err = f.Create(ctx, restoreKey, restore)
	require.NoError(err, "failed to create MedusaRestoreJob")

	t.Log("check restore status set to failed")
	require.Eventually(func() bool {
		restore := &api.MedusaRestoreJob{}
		err := f.Get(ctx, restoreKey, restore)
		if err != nil {
			return false
		}

		return restore.Status.Message != ""
	}, timeout, interval)

	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}, timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")
}

func findRestoreContainer(containers []corev1.Container) *corev1.Container {
	for _, container := range containers {
		if container.Name == restoreContainerName {
			return &container
		}
	}
	return nil
}

func TestValidateBackupForRestore(t *testing.T) {
	assert := assert.New(t)

	createBackup := func() *api.MedusaBackup {
		return &api.MedusaBackup{
			Spec: api.MedusaBackupSpec{},
			Status: api.MedusaBackupStatus{
				FinishTime:    metav1.Now(),
				TotalNodes:    3,
				FinishedNodes: 3,
				Nodes: []*api.MedusaBackupNode{
					{
						Datacenter: "dc1",
						Rack:       "r1",
					},
					{
						Datacenter: "dc1",
						Rack:       "r2",
					},
					{
						Datacenter: "dc1",
						Rack:       "r3",
					},
				},
			},
		}
	}

	createCassDc := func() *cassdcapi.CassandraDatacenter {
		return &cassdcapi.CassandraDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Name: "dc1",
			},
			Spec: cassdcapi.CassandraDatacenterSpec{
				Size: 3,
				Racks: []cassdcapi.Rack{
					{
						Name: "r1",
					},
					{
						Name: "r2",
					},
					{
						Name: "r3",
					},
				},
			},
		}
	}

	assert.NoError(validateBackupForRestore(createBackup(), createCassDc()))

	backup := createBackup()
	backup.Status.FinishedNodes = 2
	assert.Error(validateBackupForRestore(backup, createCassDc()))

	backup = createBackup()
	backup.Status.Nodes[0].Datacenter = "dc2"
	assert.Error(validateBackupForRestore(backup, createCassDc()))

	cassdc := createCassDc()
	cassdc.Spec.Racks = nil
	assert.Error(validateBackupForRestore(createBackup(), cassdc))

	backup = createBackup()
	backup.Status.Nodes[0].Rack = "default"
	backup.Status.Nodes[1].Rack = "default"
	backup.Status.Nodes[2].Rack = "default"
	assert.NoError(validateBackupForRestore(backup, cassdc))

	cassdc = createCassDc()
	cassdc.Spec.Size = 6
	assert.Error(validateBackupForRestore(createBackup(), cassdc))
}

type fakeMedusaRestoreClientFactory struct {
	clientsMutex sync.Mutex
	clients      map[string]*fakeMedusaRestoreClient

	// Responses shared by every client of the factory. A test sets them before it creates the restore job.
	backups      []*medusa.BackupSummary
	backupsErr   error
	pitrResponse *medusa.PreparePitrRestoreResponse
	pitrErr      error

	// The datacenter and targetTimestamp arguments of every PreparePitrRestore call, oldest first.
	pitrDatacenters []string
	pitrTimestamps  []string
}

// pitrTestTimestamp is the spec.targetTimestamp of the PITR test jobs. It reaches Medusa and the restore
// mapping unchanged.
const pitrTestTimestamp = "2024-07-15T12:30:00Z"

// NewMedusaClientRestoreFactory returns a factory whose clients report one successful backup of nodes node1 to
// node3 in rack "default" of datacenter "real-dc1", and accept a PITR restore of those nodes from that backup.
func NewMedusaClientRestoreFactory() *fakeMedusaRestoreClientFactory {
	return &fakeMedusaRestoreClientFactory{
		clients: make(map[string]*fakeMedusaRestoreClient, 0),
		backups: []*medusa.BackupSummary{
			{
				BackupName: restoredBackupName,
				StartTime:  0,
				FinishTime: 10,
				Status:     *medusa.StatusType_SUCCESS.Enum(),
				Nodes: []*medusa.BackupNode{
					{
						Host: "node1", Datacenter: "real-dc1", Rack: "default",
					},
					{
						Host: "node2", Datacenter: "real-dc1", Rack: "default",
					},
					{
						Host: "node3", Datacenter: "real-dc1", Rack: "default",
					},
				},
			},
		},
		pitrResponse: &medusa.PreparePitrRestoreResponse{
			Status:          medusa.StatusType_SUCCESS,
			NodeBaseBackups: map[string]string{"node1": restoredBackupName, "node2": restoredBackupName, "node3": restoredBackupName},
		},
	}
}

func (f *fakeMedusaRestoreClientFactory) NewClient(address string) (medusa.Client, error) {
	f.clientsMutex.Lock()
	defer f.clientsMutex.Unlock()
	_, ok := f.clients[address]
	if !ok {
		f.clients[address] = &fakeMedusaRestoreClient{factory: f}
	}
	return f.clients[address], nil
}

func (f *fakeMedusaRestoreClientFactory) NewClientWithTLS(address string, secret *corev1.Secret) (medusa.Client, error) {
	return f.NewClient(address)
}

type fakeMedusaRestoreClient struct {
	factory *fakeMedusaRestoreClientFactory
}

func (c *fakeMedusaRestoreClient) Close() error {
	return nil
}

func (c *fakeMedusaRestoreClient) CreateBackup(ctx context.Context, name string, backupType string) (*medusa.BackupResponse, error) {
	return nil, nil
}

func (c *fakeMedusaRestoreClient) GetBackups(ctx context.Context) ([]*medusa.BackupSummary, error) {
	return c.factory.backups, c.factory.backupsErr
}

func (c *fakeMedusaRestoreClient) PurgeBackups(ctx context.Context) (*medusa.PurgeBackupsResponse, error) {
	response := &medusa.PurgeBackupsResponse{
		NbBackupsPurged:           2,
		NbObjectsPurged:           10,
		TotalObjectsWithinGcGrace: 0,
		TotalPurgedSize:           1000,
	}
	return response, nil
}

func (c *fakeMedusaRestoreClient) BackupStatus(ctx context.Context, name string) (*medusa.BackupStatusResponse, error) {
	return nil, nil
}

func (c *fakeMedusaRestoreClient) PrepareRestore(ctx context.Context, datacenter, backupName, restoreKey string) (*medusa.PrepareRestoreResponse, error) {
	return nil, nil
}

func (c *fakeMedusaRestoreClient) PreparePitrRestore(ctx context.Context, datacenter, targetTimestamp string) (*medusa.PreparePitrRestoreResponse, error) {
	c.factory.clientsMutex.Lock()
	c.factory.pitrDatacenters = append(c.factory.pitrDatacenters, datacenter)
	c.factory.pitrTimestamps = append(c.factory.pitrTimestamps, targetTimestamp)
	c.factory.clientsMutex.Unlock()
	return c.factory.pitrResponse, c.factory.pitrErr
}

// testMedusaPitrRestoreDatacenter runs a PITR restore through the whole reconcile loop. No MedusaBackup
// exists in the namespace: Medusa picks the base backups. The job finishes only once the datacenter is ready
// again after its restart.
func testMedusaPitrRestoreDatacenter(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	k8sCtx0 := f.DataPlaneContexts[0]
	kc, dc1Key, dc1 := createRestoreTestCluster(t, ctx, f, namespace)
	require.NoError(createUserSecret(ctx, f, framework.NewClusterKey(k8sCtx0, namespace, "test-superuser")), "failed to create superuser secret")

	restore := &api.MedusaRestoreJob{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test-pitr-restore",
		},
		Spec: api.MedusaRestoreJobSpec{
			TargetTimestamp:     pitrTestTimestamp,
			CassandraDatacenter: dc1.Name,
		},
	}
	restoreKey := framework.NewClusterKey(dc1Key.K8sContext, dc1Key.Namespace, restore.Name)
	require.NoError(f.Create(ctx, restoreKey, restore), "failed to create MedusaRestoreJob")

	withDc1 := f.NewWithDatacenter(ctx, dc1Key)

	t.Log("check that the datacenter is set to be stopped")
	require.Eventually(withDc1(func(dc *cassdcapi.CassandraDatacenter) bool {
		return dc.Spec.Stopped
	}), timeout*5, interval, "timed out waiting for CassandraDatacenter stopped flag to be set")

	t.Log("check that Medusa chose the base backups before the datacenter was stopped")
	restore = &api.MedusaRestoreJob{}
	require.NoError(f.Get(ctx, restoreKey, restore))
	require.True(restore.Status.RestorePrepared)
	require.Equal(false, *restore.Status.RestoreMapping.InPlace)
	require.Len(restore.Status.RestoreMapping.HostMap, int(dc1.Spec.Size))
	require.NotNil(restore.Status.RestoreMapping.Pitr)
	require.Equal(pitrTestTimestamp, restore.Status.RestoreMapping.Pitr.TargetTimestamp)
	require.Equal(map[string]string{"node1": restoredBackupName, "node2": restoredBackupName, "node3": restoredBackupName}, restore.Status.RestoreMapping.Pitr.NodeBackups)
	expectedMapping, err := json.Marshal(restore.Status.RestoreMapping)
	require.NoError(err)

	t.Log("delete datacenter pods to simulate shutdown")
	require.NoError(f.DeleteAllOf(ctx, dc1Key.K8sContext, &corev1.Pod{}, client.InNamespace(namespace), client.MatchingLabels{cassdcapi.DatacenterLabel: "dc1"}), "failed to delete datacenter pods")

	dcStoppedTime := restore.Status.StartTime.Add(1 * time.Second)

	t.Log("set datacenter status to stopped and not ready")
	err = f.PatchDatacenterStatus(ctx, dc1Key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterStopped,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(dcStoppedTime),
		})
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterReady,
			Status:             corev1.ConditionFalse,
			LastTransitionTime: metav1.NewTime(dcStoppedTime),
		})
	})
	require.NoError(err, "failed to update datacenter status with stopped condition")

	t.Log("check that the datacenter podTemplateSpec is updated for a PITR restore")
	require.Eventually(withDc1(func(dc *cassdcapi.CassandraDatacenter) bool {
		restoreContainer := findRestoreContainer(dc.Spec.PodTemplateSpec.Spec.InitContainers)
		if restoreContainer == nil {
			return false
		}
		return utils.ContainerHasEnvVar(restoreContainer, backupNameEnvVar, "") &&
			utils.ContainerHasEnvVar(restoreContainer, restoreMappingEnvVar, string(expectedMapping)) &&
			utils.ContainerHasEnvVar(restoreContainer, restoreKeyEnvVar, restore.Status.RestoreKey)
	}), timeout, interval, "timed out waiting for CassandraDatacenter PodTemplateSpec update")

	restore = &api.MedusaRestoreJob{}
	require.NoError(f.Get(ctx, restoreKey, restore))

	t.Log("set datacenter status to updated")
	err = f.PatchDatacenterStatus(ctx, dc1Key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterUpdating,
			Status:             corev1.ConditionFalse,
			LastTransitionTime: metav1.NewTime(restore.Status.DatacenterStopped.Add(time.Second * 1)),
		})
	})
	require.NoError(err, "failed to update datacenter status with updating condition")

	t.Log("check datacenter restarted")
	require.Eventually(withDc1(func(dc *cassdcapi.CassandraDatacenter) bool {
		return !dc.Spec.Stopped
	}), timeout, interval)

	restoreFinished := func() bool {
		restore := &api.MedusaRestoreJob{}
		if err := f.Get(ctx, restoreKey, restore); err != nil {
			return false
		}
		return !restore.Status.FinishTime.IsZero()
	}

	t.Log("check that the restore does not finish while the datacenter is not ready")
	require.Never(restoreFinished, 1*time.Second, interval, "the restore finished before the datacenter was ready")

	t.Log("set datacenter status to ready")
	err = f.PatchDatacenterStatus(ctx, dc1Key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.Status.CassandraOperatorProgress = cassdcapi.ProgressReady
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(dcStoppedTime.Add(time.Second * 2)),
		})
	})
	require.NoError(err)

	t.Log("check restore status finish time set")
	require.Eventually(restoreFinished, timeout*2, interval)
	restore = &api.MedusaRestoreJob{}
	require.NoError(f.Get(ctx, restoreKey, restore))
	require.Empty(restore.Status.Message)

	require.NoError(f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}, timeout, interval), "failed to delete K8ssandraCluster")
}

// newPitrTestDC returns a two-rack datacenter "dc1" of cluster "test" whose Cassandra name is "real-dc1", with
// a medusa-restore initContainer that still carries the BACKUP_NAME of an earlier backup restore.
func newPitrTestDC() *cassdcapi.CassandraDatacenter {
	return &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{Name: "dc1", Namespace: "test", UID: "dc1"},
		Spec: cassdcapi.CassandraDatacenterSpec{
			ClusterName:    "test",
			DatacenterName: "real-dc1",
			Size:           2,
			Racks:          []cassdcapi.Rack{{Name: "r1"}, {Name: "r2"}},
			PodTemplateSpec: &corev1.PodTemplateSpec{Spec: corev1.PodSpec{InitContainers: []corev1.Container{{
				Name: restoreContainerName,
				Env:  []corev1.EnvVar{{Name: backupNameEnvVar, Value: "old-backup"}},
			}}}},
		},
	}
}

// newPitrTestJob returns a PITR restore job for dc that already has its owner reference.
func newPitrTestJob(dc *cassdcapi.CassandraDatacenter) *api.MedusaRestoreJob {
	return &api.MedusaRestoreJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "pitr-restore",
			Namespace:       dc.Namespace,
			OwnerReferences: []metav1.OwnerReference{{APIVersion: cassdcapi.GroupVersion.String(), Kind: "CassandraDatacenter", Name: dc.Name, UID: dc.UID}},
		},
		Spec: api.MedusaRestoreJobSpec{TargetTimestamp: pitrTestTimestamp, CassandraDatacenter: dc.Name},
	}
}

func newPitrTestMapping() api.MedusaRestoreMapping {
	return api.MedusaRestoreMapping{
		InPlace: ptr.To(true),
		HostMap: map[string]api.MappingSource{"test-dc1-r1-sts-0": {Source: []string{"test-dc1-r1-sts-0"}}},
		Pitr:    &api.PitrRestoreBlock{TargetTimestamp: pitrTestTimestamp, NodeBackups: map[string]string{"test-dc1-r1-sts-0": "b1"}},
	}
}

func newPitrTestScheme(t *testing.T) *runtime.Scheme {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, cassdcapi.AddToScheme(scheme))
	require.NoError(t, api.AddToScheme(scheme))
	return scheme
}

// pitrTestBackups returns one backup per rack, each with the single node <prefix>-dc1-<rack>-sts-0 of
// datacenter real-dc1, and the node to backup map that PreparePitrRestore would return for them.
func pitrTestBackups(prefix string, racks ...string) ([]*medusa.BackupSummary, map[string]string) {
	backups := []*medusa.BackupSummary{}
	nodeBackups := map[string]string{}
	for i, rack := range racks {
		host := fmt.Sprintf("%s-dc1-%s-sts-0", prefix, rack)
		name := fmt.Sprintf("b%d", i+1)
		backups = append(backups, &medusa.BackupSummary{BackupName: name, Nodes: []*medusa.BackupNode{{Host: host, Datacenter: "real-dc1", Rack: rack}}})
		nodeBackups[host] = name
	}
	return backups, nodeBackups
}

// newPitrTestReconciler returns a reconciler over a fake client that holds the PITR test datacenter, one pod
// per rack and the PITR job. No MedusaBackup object exists.
func newPitrTestReconciler(t *testing.T, factory *fakeMedusaRestoreClientFactory) (*MedusaRestoreJobReconciler, client.Client, *cassdcapi.CassandraDatacenter, *api.MedusaRestoreJob) {
	dc := newPitrTestDC()
	job := newPitrTestJob(dc)
	objects := []client.Object{dc, job}
	for i, rack := range []string{"r1", "r2"} {
		objects = append(objects, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("test-dc1-%s-sts-0", rack), Namespace: dc.Namespace, Labels: dc.GetDatacenterLabels()},
			Status:     corev1.PodStatus{PodIP: fmt.Sprintf("10.0.0.%d", i+1)},
		})
	}
	scheme := newPitrTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).WithStatusSubresource(&api.MedusaRestoreJob{}).Build()
	r := &MedusaRestoreJobReconciler{Client: c, Scheme: scheme, ReconcilerConfig: &config.ReconcilerConfig{DefaultDelay: time.Second}, ClientFactory: factory}
	return r, c, dc, job
}

func TestReconcilePitrRestore(t *testing.T) {
	inPlaceBackups, inPlaceNodes := pitrTestBackups("test", "r1", "r2")
	outOfPlaceBackups, outOfPlaceNodes := pitrTestBackups("src", "r1", "r2")
	threeRackBackups, threeRackNodes := pitrTestBackups("src", "r1", "r2", "r3")
	success := func(nodes map[string]string) *medusa.PreparePitrRestoreResponse {
		return &medusa.PreparePitrRestoreResponse{Status: medusa.StatusType_SUCCESS, NodeBaseBackups: nodes}
	}

	tests := []struct {
		name       string
		backups    []*medusa.BackupSummary
		backupsErr error
		pitrResp   *medusa.PreparePitrRestoreResponse
		pitrErr    error
		terminal   string            // substring of the failure message; "" means the restore proceeds
		transient  bool              // the reconcile returns a plain error and changes nothing
		inPlace    bool              // expected in_place of a restore that proceeds
		hostMap    map[string]string // expected host_map (target pod to source node) of a restore that proceeds
	}{
		{name: "in place", backups: inPlaceBackups, pitrResp: success(inPlaceNodes), inPlace: true,
			hostMap: map[string]string{"test-dc1-r1-sts-0": "test-dc1-r1-sts-0", "test-dc1-r2-sts-0": "test-dc1-r2-sts-0"}},
		{name: "out of place", backups: outOfPlaceBackups, pitrResp: success(outOfPlaceNodes),
			hostMap: map[string]string{"test-dc1-r1-sts-0": "src-dc1-r1-sts-0", "test-dc1-r2-sts-0": "src-dc1-r2-sts-0"}},
		{name: "Medusa rejects the restore", backups: inPlaceBackups, pitrResp: &medusa.PreparePitrRestoreResponse{Status: medusa.StatusType_FAILED}, terminal: "rejected by Medusa"},
		{name: "Medusa without PITR", pitrErr: status.Error(codes.Unimplemented, "unknown method PreparePitrRestore"), terminal: "does not support PITR"},
		{name: "rack layout does not match", backups: threeRackBackups, pitrResp: success(threeRackNodes), terminal: "racks"},
		{name: "transient PreparePitrRestore error", pitrErr: status.Error(codes.Unavailable, "sidecar not ready"), transient: true},
		{name: "transient GetBackups error", pitrResp: success(inPlaceNodes), backupsErr: fmt.Errorf("storage unavailable"), transient: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			factory := NewMedusaClientRestoreFactory()
			factory.backups, factory.backupsErr, factory.pitrResponse, factory.pitrErr = tt.backups, tt.backupsErr, tt.pitrResp, tt.pitrErr
			r, c, dc, job := newPitrTestReconciler(t, factory)
			req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(job)}

			result, err := r.Reconcile(ctx, req)
			require.NoError(t, c.Get(ctx, req.NamespacedName, job))
			require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(dc), dc))
			require.Equal(t, []string{"real-dc1"}, factory.pitrDatacenters, "PreparePitrRestore must get the Cassandra datacenter name")
			require.Equal(t, []string{pitrTestTimestamp}, factory.pitrTimestamps, "PreparePitrRestore must get the timestamp as written in the spec")

			if tt.transient {
				var terminal *medusa.TerminalError
				require.Error(t, err)
				require.False(t, goerrors.As(err, &terminal), "a transient error must not be terminal: %v", err)
				require.True(t, job.Status.FinishTime.IsZero())
				require.False(t, job.Status.RestorePrepared)
				require.False(t, dc.Spec.Stopped)
				return
			}

			if tt.terminal != "" {
				require.NoError(t, err)
				require.Equal(t, ctrl.Result{}, result)
				require.False(t, job.Status.FinishTime.IsZero())
				require.Contains(t, job.Status.Message, tt.terminal)
				require.False(t, job.Status.RestorePrepared)
				require.Nil(t, job.Status.RestoreMapping.Pitr)
				require.False(t, dc.Spec.Stopped)

				t.Log("a failed job is not reconciled again")
				result, err = r.Reconcile(ctx, req)
				require.NoError(t, err)
				require.Equal(t, ctrl.Result{}, result)
				require.Len(t, factory.pitrDatacenters, 1, "a finished job must not call Medusa")
				return
			}

			require.NoError(t, err)
			require.NotZero(t, result.RequeueAfter)
			require.True(t, job.Status.RestorePrepared)
			require.True(t, job.Status.FinishTime.IsZero())
			mapping := job.Status.RestoreMapping
			require.Equal(t, tt.inPlace, *mapping.InPlace)
			require.Len(t, mapping.HostMap, len(tt.hostMap))
			for target, source := range tt.hostMap {
				require.Equal(t, api.MappingSource{Source: []string{source}}, mapping.HostMap[target])
			}
			require.Equal(t, pitrTestTimestamp, mapping.Pitr.TargetTimestamp, "the target timestamp is the spec value, unchanged")
			require.Equal(t, tt.pitrResp.NodeBaseBackups, mapping.Pitr.NodeBackups, "the node_backups keys stay source node names")

			t.Log("the next reconcile stops the datacenter with no further Medusa call")
			result, err = r.Reconcile(ctx, req)
			require.NoError(t, err)
			require.NotZero(t, result.RequeueAfter)
			require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(dc), dc))
			require.True(t, dc.Spec.Stopped)
			require.Len(t, factory.pitrDatacenters, 1)
		})
	}
}

// TestReconcileInvalidSpecIsTerminal covers clusters where the validating webhook is not running: a job with
// both or neither of spec.backup and spec.targetTimestamp fails for good without a Medusa call.
func TestReconcileInvalidSpecIsTerminal(t *testing.T) {
	for name, spec := range map[string]api.MedusaRestoreJobSpec{
		"both set":    {Backup: "b1", TargetTimestamp: "2024-07-15T12:30:00Z", CassandraDatacenter: "dc1"},
		"neither set": {CassandraDatacenter: "dc1"},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			factory := NewMedusaClientRestoreFactory()
			r, c, dc, job := newPitrTestReconciler(t, factory)
			require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(job), job))
			job.Spec = spec
			require.NoError(t, c.Update(ctx, job))
			req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(job)}

			for i := 0; i < 2; i++ {
				result, err := r.Reconcile(ctx, req)
				require.NoError(t, err)
				require.Equal(t, ctrl.Result{}, result)
			}
			require.NoError(t, c.Get(ctx, req.NamespacedName, job))
			require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(dc), dc))
			require.False(t, job.Status.FinishTime.IsZero())
			require.NotEmpty(t, job.Status.Message)
			require.Empty(t, factory.pitrDatacenters)
			require.False(t, dc.Spec.Stopped)
		})
	}
}

func TestPodTemplateSpecUpdateCompletePitr(t *testing.T) {
	dc := newPitrTestDC()
	dc.SetCondition(cassdcapi.DatacenterCondition{Type: cassdcapi.DatacenterUpdating, Status: corev1.ConditionFalse, LastTransitionTime: metav1.Now()})
	job := newPitrTestJob(dc)
	job.Status.RestoreKey = "key"
	job.Status.DatacenterStopped = metav1.NewTime(time.Now().Add(-time.Minute))
	job.Status.RestoreMapping = newPitrTestMapping()

	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{name: "env vars match", env: map[string]string{backupNameEnvVar: "", restoreKeyEnvVar: "key"}, want: true},
		{name: "BACKUP_NAME not cleared", env: map[string]string{backupNameEnvVar: "old-backup", restoreKeyEnvVar: "key"}},
		{name: "RESTORE_KEY differs", env: map[string]string{backupNameEnvVar: "", restoreKeyEnvVar: "other"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := []corev1.EnvVar{}
			for name, value := range tt.env {
				env = append(env, corev1.EnvVar{Name: name, Value: value})
			}
			sts := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-dc1-r1-sts", Namespace: dc.Namespace, Labels: map[string]string{cassdcapi.ClusterLabel: "test", cassdcapi.DatacenterLabel: "dc1"}},
				Spec:       appsv1.StatefulSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{InitContainers: []corev1.Container{{Name: restoreContainerName, Env: env}}}}},
			}
			scheme := newPitrTestScheme(t)
			r := &MedusaRestoreJobReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(sts).Build(), Scheme: scheme}
			complete, err := r.podTemplateSpecUpdateComplete(context.Background(), &medusa.RestoreRequest{Log: logr.Discard(), RestoreJob: job, Datacenter: dc})
			require.NoError(t, err)
			require.Equal(t, tt.want, complete)
		})
	}
}

func TestUpdateMedusaRestoreInitContainerPitr(t *testing.T) {
	dc := newPitrTestDC()
	job := newPitrTestJob(dc)
	job.Status.RestoreKey = "key"
	job.Status.RestoreMapping = newPitrTestMapping()
	mappingJSON, err := json.Marshal(job.Status.RestoreMapping)
	require.NoError(t, err)

	require.NoError(t, updateMedusaRestoreInitContainer(&medusa.RestoreRequest{Log: logr.Discard(), RestoreJob: job, Datacenter: dc}))

	container := findRestoreContainer(dc.Spec.PodTemplateSpec.Spec.InitContainers)
	require.True(t, utils.ContainerHasEnvVar(container, backupNameEnvVar, ""), "BACKUP_NAME of an earlier backup restore must be cleared")
	require.True(t, utils.ContainerHasEnvVar(container, restoreMappingEnvVar, string(mappingJSON)))
	require.True(t, utils.ContainerHasEnvVar(container, restoreKeyEnvVar, "key"))
}
