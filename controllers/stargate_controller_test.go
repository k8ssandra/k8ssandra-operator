package controllers

import (
	"context"
	"fmt"
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"testing"
)

func testStargate(t *testing.T) {
	req := require.New(t)
	testClient := testClients[fmt.Sprintf(clusterProtoName, 0)]

	namespace := "default"
	ctx := context.Background()

	dc := &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "dc1",
		},
		Spec: cassdcapi.CassandraDatacenterSpec{
			Size:          1,
			ServerVersion: "3.11.10",
			ServerType:    "cassandra",
			ClusterName:   "test",
			StorageConfig: cassdcapi.StorageConfig{
				CassandraDataVolumeClaimSpec: &v1.PersistentVolumeClaimSpec{
					AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				},
			},
		},
	}

	err := testClient.Create(ctx, dc)
	req.NoError(err, "failed to create CassandraDatacenter")

	t.Log("check that the datacenter was created")
	dcKey := types.NamespacedName{Namespace: namespace, Name: "dc1"}

	req.Eventually(func() bool {
		err := testClient.Get(ctx, dcKey, dc)
		return err == nil
	}, timeout, interval)

	stargate := &api.Stargate{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace,
			Name: "dc1-stargate",
		},
		Spec: api.StargateSpec{
			StargateTemplate: api.StargateTemplate{Size: 1},
			DatacenterRef:    "dc1",
		},
	}

	// artificially put the DC in a ready state
	dc.SetCondition(cassdcapi.DatacenterCondition{Type: cassdcapi.DatacenterReady, Status: v1.ConditionTrue, LastTransitionTime: metav1.Now()})
	dc.Status.CassandraOperatorProgress = cassdcapi.ProgressReady
	dc.Status.LastServerNodeStarted = metav1.Now()
	dc.Status.NodeStatuses = cassdcapi.CassandraStatusMap{"node1": cassdcapi.CassandraNodeStatus{HostID: "irrelevant"}}
	dc.Status.NodeReplacements = []string{}
	dc.Status.LastRollingRestart = metav1.Now()
	dc.Status.QuietPeriod = metav1.Now()
	//goland:noinspection GoDeprecation
	dc.Status.SuperUserUpserted = metav1.Now()
	dc.Status.UsersUpserted = metav1.Now()

	err = testClient.Status().Update(ctx, dc)
	req.NoError(err, "failed to update dc")

	err = testClient.Create(ctx, stargate)
	req.NoError(err, "failed to create Stargate")

	t.Log("check that the Stargate resource was created")
	stargateKey := types.NamespacedName{Namespace: namespace, Name: "dc1-stargate"}
	req.Eventually(func() bool {
		err := testClient.Get(ctx, stargateKey, stargate)
		return err == nil && stargate.Status.Progress == api.StargateProgressDeploying
	}, timeout, interval)

	deploymentKey := types.NamespacedName{Namespace: namespace, Name: "test-dc1-stargate-deployment"}
	deployment := &appsv1.Deployment{}
	req.Eventually(func() bool {
		err := testClient.Get(ctx, deploymentKey, deployment)
		return err == nil
	}, timeout, interval)

	t.Log("check that the owner reference is set on the Stargate deployment")
	req.Len(deployment.OwnerReferences, 1, "expected to find 1 owner reference for Stargate deployment")
	req.Equal(stargate.UID, deployment.OwnerReferences[0].UID)

	deployment.Status.Replicas = 1
	deployment.Status.ReadyReplicas = 1
	deployment.Status.AvailableReplicas = 1
	err = testClient.Status().Update(ctx, deployment)
	req.NoError(err, "failed to update deployment")

	serviceKey := types.NamespacedName{Namespace: namespace, Name: "test-dc1-stargate-service"}
	service := &corev1.Service{}
	req.Eventually(func() bool {
		err := testClient.Get(ctx, serviceKey, service)
		return err == nil
	}, timeout, interval)

	t.Log("check that the Stargate resource is fully reconciled")
	req.Eventually(func() bool {
		err := testClient.Get(ctx, stargateKey, stargate)
		return err == nil && stargate.Status.Progress == api.StargateProgressRunning
	}, timeout, interval)

	t.Log("check Stargate condition")
	req.Len(stargate.Status.Conditions, 1, "expected to find 1 condition for Stargate")
	req.Equal(api.StargateReady, stargate.Status.Conditions[0].Type)
	req.Equal(corev1.ConditionTrue, stargate.Status.Conditions[0].Status)
}
