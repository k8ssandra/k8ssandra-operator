package medusa

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"testing"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func testBackupDatacenter(t *testing.T, namespace string, testClient client.Client) {
	assert := assert.New(t)
	require := require.New(t)

	ctx := context.Background()

	backupName := "test-backup"

	dcKey := types.NamespacedName{
		Name:      TestCassandraDatacenterName,
		Namespace: namespace,
	}

	dc := &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dcKey.Name,
			Namespace:   dcKey.Namespace,
			Annotations: map[string]string{},
		},
		Spec: cassdcapi.CassandraDatacenterSpec{
			ClusterName:   "test-dc",
			ServerType:    "cassandra",
			ServerVersion: "3.11.7",
			Size:          3,
			StorageConfig: cassdcapi.StorageConfig{
				CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
					VolumeName: "data",
				},
			},
			Racks: []cassdcapi.Rack{
				{
					Name: "rack1",
				},
			},
			PodTemplateSpec: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{},
					InitContainers: []corev1.Container{
						{
							Name: "medusa-restore",
							Env: []corev1.EnvVar{
								{
									Name:  "MEDUSA_MODE",
									Value: "RESTORE",
								},
							},
						},
					},
				},
			},
		},
	}

	t.Logf("creating datacenter key %s", dcKey)
	t.Logf("creating datacenter %s", dc.Name)

	err := testClient.Create(ctx, dc)
	assert.NoError(err, "failed to create datacenter")

	t.Logf("creating datacenter service")

	dcServiceKey := types.NamespacedName{Namespace: dcKey.Namespace, Name: dc.GetAllPodsServiceName()}
	dcService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: dcServiceKey.Namespace,
			Name:      dcServiceKey.Name,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				cassdcapi.ClusterLabel: dc.Spec.ClusterName,
			},
			Ports: []corev1.ServicePort{
				{
					Name: "cql",
					Port: 9042,
				},
			},
		},
	}
	err = testClient.Create(ctx, dcService)
	assert.NoError(err, "failed to create datacenter service %s", dcServiceKey)

	t.Log("creating datacenter pods")
	createDatacenterPods(t, ctx, dc, testClient)

	t.Log("make the datacenter ready")
	patch := client.MergeFrom(dc.DeepCopy())
	dc.Status.CassandraOperatorProgress = cassdcapi.ProgressReady
	dc.Status.Conditions = []cassdcapi.DatacenterCondition{
		{
			Status: corev1.ConditionTrue,
			Type:   cassdcapi.DatacenterReady,
		},
	}
	err = testClient.Status().Patch(context.Background(), dc, patch)
	assert.NoError(err, "failed to make datacenter ready")

	t.Log("creating CassandraBackup")
	backupKey := types.NamespacedName{Namespace: namespace, Name: backupName}
	backup := &api.CassandraBackup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      backupName,
		},
		Spec: api.CassandraBackupSpec{
			Name:                backupName,
			CassandraDatacenter: dc.Name,
		},
	}

	err = testClient.Create(ctx, backup)
	assert.NoError(err, "failed to create CassandraBackup")

	t.Log("verify that the backups are started")
	require.Eventually(func() bool {
		updated := &api.CassandraBackup{}
		err := testClient.Get(context.Background(), backupKey, updated)
		if err != nil {
			return false
		}
		return !updated.Status.StartTime.IsZero()
	}, timeout, interval)

	t.Log("verify that the CassandraDatacenter spec is added to the backup status")
	require.Eventually(func() bool {
		updated := &api.CassandraBackup{}
		err := testClient.Get(context.Background(), backupKey, updated)
		if err != nil {
			return false
		}

		return updated.Status.CassdcTemplateSpec.Spec.ClusterName == dc.Spec.ClusterName &&
			updated.Status.CassdcTemplateSpec.Spec.Size == dc.Spec.Size &&
			updated.Status.CassdcTemplateSpec.Spec.ServerVersion == dc.Spec.ServerVersion &&
			updated.Status.CassdcTemplateSpec.Spec.ServerImage == dc.Spec.ServerImage &&
			reflect.DeepEqual(updated.Status.CassdcTemplateSpec.Spec.Config, dc.Spec.Config) &&
			updated.Status.CassdcTemplateSpec.Spec.ManagementApiAuth == dc.Spec.ManagementApiAuth &&
			reflect.DeepEqual(updated.Status.CassdcTemplateSpec.Spec.Resources, dc.Spec.Resources) &&
			reflect.DeepEqual(updated.Status.CassdcTemplateSpec.Spec.SystemLoggerResources, dc.Spec.SystemLoggerResources) &&
			reflect.DeepEqual(updated.Status.CassdcTemplateSpec.Spec.ConfigBuilderResources, dc.Spec.ConfigBuilderResources) &&
			reflect.DeepEqual(updated.Status.CassdcTemplateSpec.Spec.Racks, dc.Spec.Racks) &&
			reflect.DeepEqual(updated.Status.CassdcTemplateSpec.Spec.StorageConfig, dc.Spec.StorageConfig) &&
			updated.Status.CassdcTemplateSpec.Spec.Stopped == dc.Spec.Stopped &&
			updated.Status.CassdcTemplateSpec.Spec.ConfigBuilderImage == dc.Spec.ConfigBuilderImage &&
			updated.Status.CassdcTemplateSpec.Spec.AllowMultipleNodesPerWorker == dc.Spec.AllowMultipleNodesPerWorker &&
			updated.Status.CassdcTemplateSpec.Spec.ServiceAccount == dc.Spec.ServiceAccount &&
			reflect.DeepEqual(updated.Status.CassdcTemplateSpec.Spec.NodeSelector, dc.Spec.NodeSelector) &&
			// reflect.DeepEqual(updated.Status.CassdcTemplateSpec.Spec.PodTemplateSpec, cassdc.Spec.PodTemplateSpec) &&
			reflect.DeepEqual(updated.Status.CassdcTemplateSpec.Spec.Users, dc.Spec.Users) &&
			reflect.DeepEqual(updated.Status.CassdcTemplateSpec.Spec.AdditionalSeeds, dc.Spec.AdditionalSeeds)
	}, timeout, interval)

	t.Log("verify the backup finished")
	require.Eventually(func() bool {
		updated := &api.CassandraBackup{}
		err := testClient.Get(context.Background(), backupKey, updated)
		if err != nil {
			return false
		}
		return len(updated.Status.Finished) == 3 && len(updated.Status.InProgress) == 0
	}, timeout, interval)

	t.Log("verify that medusa gRPC clients are invoked")
	assert.Equal(medusaClientFactory.GetRequestedBackups(), map[string][]string{
		fmt.Sprintf("%s:%d", getPodIpAddress(0), backupSidecarPort): {backupName},
		fmt.Sprintf("%s:%d", getPodIpAddress(1), backupSidecarPort): {backupName},
		fmt.Sprintf("%s:%d", getPodIpAddress(2), backupSidecarPort): {backupName},
	})
}

func createDatacenterPods(t *testing.T, ctx context.Context, dc *cassdcapi.CassandraDatacenter, testClient client.Client) {
	for i := int32(0); i < dc.Spec.Size; i++ {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: dc.Namespace,
				Name:      fmt.Sprintf("%s-%d", dc.Spec.ClusterName, i),
				Labels: map[string]string{
					cassdcapi.ClusterLabel:    dc.Spec.ClusterName,
					cassdcapi.DatacenterLabel: dc.Name,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "cassandra",
						Image: "cassandra",
					},
					{
						Name:  backupSidecarName,
						Image: backupSidecarName,
					},
				},
			},
		}
		err := testClient.Create(ctx, pod)
		assert.NoError(t, err, "failed to create datacenter pod")

		patch := client.MergeFrom(pod.DeepCopy())
		pod.Status.PodIP = getPodIpAddress(int(i))

		err = testClient.Status().Patch(ctx, pod, patch)
		assert.NoError(t, err, "failed to patch datacenter pod status")
	}
}

// Creates a fake ip address with the pod's orginal index from the StatefulSet
func getPodIpAddress(index int) string {
	return "192.168.1." + strconv.Itoa(50+index)
}

type fakeMedusaClientFactory struct {
	clientsMutex sync.Mutex
	clients      map[string]*fakeMedusaClient
}

func NewMedusaClientFactory() *fakeMedusaClientFactory {
	return &fakeMedusaClientFactory{clients: make(map[string]*fakeMedusaClient, 0)}
}

func (f *fakeMedusaClientFactory) NewClient(address string) (medusa.Client, error) {
	medusaClient := newFakeMedusaClient()
	f.clientsMutex.Lock()
	f.clients[address] = medusaClient
	f.clientsMutex.Unlock()
	return medusaClient, nil
}

func (f *fakeMedusaClientFactory) GetRequestedBackups() map[string][]string {
	requestedBackups := make(map[string][]string)
	for k, v := range f.clients {
		requestedBackups[k] = v.RequestedBackups
	}
	return requestedBackups
}

type fakeMedusaClient struct {
	RequestedBackups []string
}

func newFakeMedusaClient() *fakeMedusaClient {
	return &fakeMedusaClient{RequestedBackups: make([]string, 0)}
}

func (c *fakeMedusaClient) Close() error {
	return nil
}

func (c *fakeMedusaClient) CreateBackup(ctx context.Context, name string, backupType string) error {
	c.RequestedBackups = append(c.RequestedBackups, name)
	return nil
}

func (c *fakeMedusaClient) GetBackups(ctx context.Context) ([]*medusa.BackupSummary, error) {
	return nil, nil
}

func (c *fakeMedusaClient) BackupStatus(ctx context.Context, name string) (*medusa.BackupStatusResponse, error) {
	return nil, nil
}
