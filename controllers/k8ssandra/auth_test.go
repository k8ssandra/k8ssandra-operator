package k8ssandra

import (
	"context"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reaper"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

// createSingleDcClusterNoAuth verifies that it is possible to create an unauthenticated cluster with one DC and with
// Reaper and Stargate.
func createSingleDcClusterNoAuth(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "cluster1",
		},
		Spec: api.K8ssandraClusterSpec{
			Auth: pointer.BoolPtr(false),
			Cassandra: &api.CassandraClusterTemplate{
				Datacenters: []api.CassandraDatacenterTemplate{{
					Meta:          api.EmbeddedObjectMeta{Name: "dc1"},
					K8sContext:    "cluster-1",
					Size:          1,
					ServerVersion: "3.11.10",
					StorageConfig: &cassdcapi.StorageConfig{
						CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
							StorageClassName: &defaultStorageClass,
						},
					},
				}},
			},
			Stargate: &stargateapi.StargateClusterTemplate{Size: 1},
			Reaper:   &reaperapi.ReaperClusterTemplate{},
		},
	}

	err := f.Client.Create(ctx, kc)
	require.NoError(t, err, "failed to create K8ssandraCluster")

	kcKey := framework.ClusterKey{K8sContext: "cluster-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: kc.Name}}
	dcKey := framework.ClusterKey{K8sContext: "cluster-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	reaperKey := framework.ClusterKey{K8sContext: "cluster-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "cluster1-dc1-reaper"}}
	stargateKey := framework.ClusterKey{K8sContext: "cluster-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "cluster1-dc1-stargate"}}

	verifyFinalizerAdded(ctx, t, f, kcKey.NamespacedName)
	verifySuperuserSecretCreated(ctx, t, f, kc)
	verifySecretNotCreated(ctx, t, f, kc.Namespace, reaper.DefaultUserSecretName(kc.Name))
	verifySecretNotCreated(ctx, t, f, kc.Namespace, reaper.DefaultJmxUserSecretName(kc.Name))
	verifyReplicatedSecretReconciled(ctx, t, f, kc)
	verifySystemReplicationAnnotationSet(ctx, t, f, kc)

	t.Log("check that the datacenter was created")
	require.Eventually(t, f.DatacenterExists(ctx, dcKey), timeout, interval)

	t.Log("update dc status to ready")
	err = f.SetDatacenterStatusReady(ctx, dcKey)
	require.NoError(t, err, "failed to set dc status ready")

	t.Log("check that stargate is created")
	require.Eventually(t, f.StargateExists(ctx, stargateKey), timeout, interval)

	t.Log("update stargate status to ready")
	err = f.SetStargateStatusReady(ctx, stargateKey)
	require.NoError(t, err, "failed to set stargate status ready")

	t.Log("check that reaper is created")
	require.Eventually(t, f.ReaperExists(ctx, reaperKey), timeout, interval)

	t.Log("update reaper status to ready")
	err = f.SetReaperStatusReady(ctx, reaperKey)
	require.NoError(t, err, "failed to set Stargate status ready")

	withDatacenter := f.NewWithDatacenter(ctx, dcKey)

	t.Log("check that authentication is disabled in DC")
	require.Eventually(t, withDatacenter(func(dc *cassdcapi.CassandraDatacenter) bool {
		// there should be no JMX init container
		if dc.Spec.PodTemplateSpec != nil {
			for _, container := range dc.Spec.PodTemplateSpec.Spec.InitContainers {
				if container.Name == cassandra.JmxInitContainer {
					return false
				}
			}
		}
		// the config should have JMX auth disabled
		return assert.Contains(t, string(dc.Spec.Config), "-Dcom.sun.management.jmxremote.authenticate=false")
	}), timeout, interval)

	t.Log("check that remote JMX is enabled")
	require.Eventually(t, withDatacenter(func(dc *cassdcapi.CassandraDatacenter) bool {
		if dc.Spec.PodTemplateSpec != nil {
			for _, container := range dc.Spec.PodTemplateSpec.Spec.Containers {
				if container.Name == reconciliation.CassandraContainerName {
					for _, envVar := range container.Env {
						if envVar.Name == "LOCAL_JMX" {
							return envVar.Value == "no"
						}
					}
				}
			}
		}
		return false
	}), timeout, interval)

	withStargate := f.NewWithStargate(ctx, stargateKey)
	withReaper := f.NewWithReaper(ctx, reaperKey)

	t.Log("check that authentication is disabled in Stargate CRD")
	require.Eventually(t, withStargate(func(sg *stargateapi.Stargate) bool {
		return !sg.Spec.IsAuthEnabled()
	}), timeout, interval)

	t.Log("check that authentication is disabled in Reaper CRD")
	require.Eventually(t, withReaper(func(r *reaperapi.Reaper) bool {
		return r.Spec.CassandraUserSecretRef == corev1.LocalObjectReference{} &&
			r.Spec.JmxUserSecretRef == corev1.LocalObjectReference{}
	}), timeout, interval)

	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})
	require.NoError(t, err, "failed to delete K8ssandraCluster")
	f.AssertObjectDoesNotExist(ctx, t, dcKey, &cassdcapi.CassandraDatacenter{}, timeout, interval)
	f.AssertObjectDoesNotExist(ctx, t, stargateKey, &stargateapi.Stargate{}, timeout, interval)
	f.AssertObjectDoesNotExist(ctx, t, reaperKey, &reaperapi.Reaper{}, timeout, interval)
}

// createSingleDcClusterAuth verifies that it is possible to create an authenticated cluster with one DC and with
// Reaper and Stargate.
func createSingleDcClusterAuth(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "cluster1",
		},
		Spec: api.K8ssandraClusterSpec{
			Auth: pointer.BoolPtr(true),
			Cassandra: &api.CassandraClusterTemplate{
				Datacenters: []api.CassandraDatacenterTemplate{{
					Meta:          api.EmbeddedObjectMeta{Name: "dc1"},
					K8sContext:    "cluster-1",
					Size:          1,
					ServerVersion: "3.11.10",
					StorageConfig: &cassdcapi.StorageConfig{
						CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
							StorageClassName: &defaultStorageClass,
						},
					},
				}},
			},
			Stargate: &stargateapi.StargateClusterTemplate{Size: 1},
			Reaper:   &reaperapi.ReaperClusterTemplate{},
		},
	}

	err := f.Client.Create(ctx, kc)
	require.NoError(t, err, "failed to create K8ssandraCluster")

	kcKey := framework.ClusterKey{K8sContext: "cluster-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: kc.Name}}
	dcKey := framework.ClusterKey{K8sContext: "cluster-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	reaperKey := framework.ClusterKey{K8sContext: "cluster-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "cluster1-dc1-reaper"}}
	stargateKey := framework.ClusterKey{K8sContext: "cluster-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "cluster1-dc1-stargate"}}

	verifyFinalizerAdded(ctx, t, f, kcKey.NamespacedName)
	verifySuperuserSecretCreated(ctx, t, f, kc)
	verifySecretCreated(ctx, t, f, kc.Namespace, reaper.DefaultUserSecretName(kc.Name))
	verifySecretCreated(ctx, t, f, kc.Namespace, reaper.DefaultJmxUserSecretName(kc.Name))
	verifyReplicatedSecretReconciled(ctx, t, f, kc)
	verifySystemReplicationAnnotationSet(ctx, t, f, kc)

	t.Log("check that the datacenter was created")
	require.Eventually(t, f.DatacenterExists(ctx, dcKey), timeout, interval)

	t.Log("update dc status to ready")
	err = f.SetDatacenterStatusReady(ctx, dcKey)
	require.NoError(t, err, "failed to set dc status ready")

	t.Log("check that stargate is created")
	require.Eventually(t, f.StargateExists(ctx, stargateKey), timeout, interval)

	t.Log("update stargate status to ready")
	err = f.SetStargateStatusReady(ctx, stargateKey)
	require.NoError(t, err, "failed to set stargate status ready")

	t.Log("check that reaper is created")
	require.Eventually(t, f.ReaperExists(ctx, reaperKey), timeout, interval)

	t.Log("update reaper status to ready")
	err = f.SetReaperStatusReady(ctx, reaperKey)
	require.NoError(t, err, "failed to set Stargate status ready")

	withDatacenter := f.NewWithDatacenter(ctx, dcKey)

	t.Log("check that authentication is enabled in DC")
	require.Eventually(t, withDatacenter(func(dc *cassdcapi.CassandraDatacenter) bool {
		// there should be a JMX init container with 4 env vars
		if dc.Spec.PodTemplateSpec != nil {
			for _, container := range dc.Spec.PodTemplateSpec.Spec.InitContainers {
				if container.Name == cassandra.JmxInitContainer {
					if container.Env[0].Name == "SUPERUSER_JMX_USERNAME" &&
						container.Env[1].Name == "SUPERUSER_JMX_PASSWORD" &&
						container.Env[2].Name == "REAPER_JMX_USERNAME" &&
						container.Env[3].Name == "REAPER_JMX_PASSWORD" &&
						container.Args[2] == "echo \"$SUPERUSER_JMX_USERNAME $SUPERUSER_JMX_PASSWORD\" >> /config/jmxremote.password && "+
							"echo \"$REAPER_JMX_USERNAME $REAPER_JMX_PASSWORD\" >> /config/jmxremote.password" {
						break
					}
				}
				return false
			}
			// the config should have JMX auth enabled
			return assert.Contains(t, string(dc.Spec.Config), "-Dcom.sun.management.jmxremote.authenticate=true")
		}
		return false
	}), timeout, interval)

	t.Log("check that remote JMX is enabled")
	require.Eventually(t, withDatacenter(func(dc *cassdcapi.CassandraDatacenter) bool {
		if dc.Spec.PodTemplateSpec != nil {
			for _, container := range dc.Spec.PodTemplateSpec.Spec.Containers {
				if container.Name == reconciliation.CassandraContainerName {
					for _, envVar := range container.Env {
						if envVar.Name == "LOCAL_JMX" {
							return envVar.Value == "no"
						}
					}
				}
			}
		}
		return false
	}), timeout, interval)

	withStargate := f.NewWithStargate(ctx, stargateKey)
	withReaper := f.NewWithReaper(ctx, reaperKey)

	t.Log("check that authentication is enabled in Stargate CRD")
	require.Eventually(t, withStargate(func(sg *stargateapi.Stargate) bool {
		return sg.Spec.IsAuthEnabled()
	}), timeout, interval)

	t.Log("check that authentication is enabled in Reaper CRD")
	require.Eventually(t, withReaper(func(r *reaperapi.Reaper) bool {
		return r.Spec.CassandraUserSecretRef == corev1.LocalObjectReference{Name: reaper.DefaultUserSecretName("cluster1")} &&
			r.Spec.JmxUserSecretRef == corev1.LocalObjectReference{Name: reaper.DefaultJmxUserSecretName("cluster1")}
	}), timeout, interval)

	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})
	require.NoError(t, err, "failed to delete K8ssandraCluster")
	f.AssertObjectDoesNotExist(ctx, t, dcKey, &cassdcapi.CassandraDatacenter{}, timeout, interval)
	f.AssertObjectDoesNotExist(ctx, t, stargateKey, &stargateapi.Stargate{}, timeout, interval)
	f.AssertObjectDoesNotExist(ctx, t, reaperKey, &reaperapi.Reaper{}, timeout, interval)
}
