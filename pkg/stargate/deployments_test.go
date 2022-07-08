package stargate

import (
	"k8s.io/utils/pointer"
	"strings"
	"testing"

	"github.com/k8ssandra/k8ssandra-operator/pkg/images"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/encryption"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	namespace = "namespace1"
)

var (
	dc = &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "dc1",
		},
		Spec: cassdcapi.CassandraDatacenterSpec{
			ServerVersion: "3.11.11",
			ClusterName:   "cluster1",
		},
	}
	stargate = &api.Stargate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "s1",
		},
		Spec: api.StargateSpec{
			DatacenterRef: corev1.LocalObjectReference{Name: dc.Name},
			StargateDatacenterTemplate: api.StargateDatacenterTemplate{
				StargateClusterTemplate: api.StargateClusterTemplate{Size: 1},
			},
			CassandraEncryption: &api.CassandraEncryption{
				ServerEncryptionStores: &encryption.Stores{
					KeystoreSecretRef: corev1.LocalObjectReference{
						Name: "server-keystore-secret",
					},
					TruststoreSecretRef: corev1.LocalObjectReference{
						Name: "server-truststore-secret",
					},
				},
				ClientEncryptionStores: &encryption.Stores{
					KeystoreSecretRef: corev1.LocalObjectReference{
						Name: "client-keystore-secret",
					},
					TruststoreSecretRef: corev1.LocalObjectReference{
						Name: "client-truststore-secret",
					},
				},
			},
		},
	}
)

func TestNewDeployments(t *testing.T) {
	t.Run("Default rack single replica", testNewDeploymentsDefaultRackSingleReplica)
	t.Run("Single rack many replicas", testNewDeploymentsSingleRackManyReplicas)
	t.Run("Many racks many replicas", testNewDeploymentsManyRacksManyReplicas)
	t.Run("Many racks custom affinity dc", testNewDeploymentsManyRacksCustomAffinityDc)
	t.Run("Many racks custom affinity stargate", testNewDeploymentsManyRacksCustomAffinityStargate)
	t.Run("Many racks few replicas", testNewDeploymentsManyRacksFewReplicas)
	t.Run("CassandraConfigMap", testNewDeploymentsCassandraConfigMap)
	t.Run("Custom images", testImages)
	t.Run("Encryption", testNewDeploymentsEncryption)
	t.Run("Authentication", testNewDeploymentsAuthentication)
}

func testNewDeploymentsDefaultRackSingleReplica(t *testing.T) {

	deployments := NewDeployments(stargate, dc)
	require.Len(t, deployments, 1)
	require.Contains(t, deployments, "cluster1-dc1-default-stargate-deployment")
	deployment := deployments["cluster1-dc1-default-stargate-deployment"]

	assert.Equal(t, "cluster1-dc1-default-stargate-deployment", deployment.Name)
	assert.Equal(t, namespace, deployment.Namespace)
	assert.Contains(t, deployment.Labels, api.StargateLabel)
	assert.Equal(t, "s1", deployment.Labels[api.StargateLabel])

	assert.EqualValues(t, 1, *deployment.Spec.Replicas)

	assert.Contains(t, deployment.Spec.Selector.MatchLabels, api.StargateDeploymentLabel)
	assert.Equal(t, "cluster1-dc1-default-stargate-deployment", deployment.Spec.Selector.MatchLabels[api.StargateDeploymentLabel])

	assert.Contains(t, deployment.Spec.Template.Labels, api.StargateLabel)
	assert.Equal(t, "s1", deployment.Spec.Template.Labels[api.StargateLabel])
	assert.Contains(t, deployment.Spec.Template.Labels, api.StargateDeploymentLabel)
	assert.Equal(t, "cluster1-dc1-default-stargate-deployment", deployment.Spec.Template.Labels[api.StargateDeploymentLabel])

	assert.Equal(t, "default", deployment.Spec.Template.Spec.ServiceAccountName)
	assert.Equal(t, affinityForRack(dc, "default"), deployment.Spec.Template.Spec.Affinity)
	assert.Nil(t, deployment.Spec.Template.Spec.Tolerations)

	container := findContainer(&deployment, deployment.Name)
	require.NotNil(t, container, "failed to find stargate container")

	assert.Equal(t, "docker.io/stargateio/stargate-3_11:v"+DefaultVersion, container.Image)
	assert.Equal(t, corev1.PullIfNotPresent, container.ImagePullPolicy)

	assert.EqualValues(t, resource.MustParse("200m"), container.Resources.Requests[corev1.ResourceCPU])
	assert.EqualValues(t, resource.MustParse("512Mi"), container.Resources.Requests[corev1.ResourceMemory])

	assert.EqualValues(t, resource.MustParse("1000m"), container.Resources.Limits[corev1.ResourceCPU])
	assert.EqualValues(t, resource.MustParse("1024Mi"), container.Resources.Limits[corev1.ResourceMemory])

	assert.EqualValues(t, 10, container.LivenessProbe.TimeoutSeconds)
	assert.EqualValues(t, 30, container.LivenessProbe.InitialDelaySeconds)
	assert.EqualValues(t, 5, container.LivenessProbe.FailureThreshold)
	assert.Equal(t, "/checker/liveness", container.LivenessProbe.ProbeHandler.HTTPGet.Path)
	assert.Equal(t, "health", container.LivenessProbe.ProbeHandler.HTTPGet.Port.String())

	assert.EqualValues(t, 10, container.ReadinessProbe.TimeoutSeconds)
	assert.EqualValues(t, 30, container.ReadinessProbe.InitialDelaySeconds)
	assert.EqualValues(t, 5, container.ReadinessProbe.FailureThreshold)
	assert.Equal(t, "/checker/readiness", container.ReadinessProbe.ProbeHandler.HTTPGet.Path)
	assert.Equal(t, "health", container.ReadinessProbe.ProbeHandler.HTTPGet.Port.String())

	clusterVersion := utils.FindEnvVarInContainer(container, "CLUSTER_VERSION")
	require.NotNil(t, clusterVersion, "failed to find CLUSTER_VERSION env var")
	assert.Equal(t, "3.11", clusterVersion.Value)

	clusterName := utils.FindEnvVarInContainer(container, "CLUSTER_NAME")
	require.NotNil(t, clusterName, "failed to find CLUSTER_NAME env var")
	assert.Equal(t, "cluster1", clusterName.Value)

	datacenterName := utils.FindEnvVarInContainer(container, "DATACENTER_NAME")
	require.NotNil(t, datacenterName, "failed to find DATACENTER_NAME env var")
	assert.Equal(t, "dc1", datacenterName.Value)

	rackName := utils.FindEnvVarInContainer(container, "RACK_NAME")
	require.NotNil(t, rackName, "failed to find RACK_NAME env var")
	assert.Equal(t, "default", rackName.Value)

	seed := utils.FindEnvVarInContainer(container, "SEED")
	require.NotNil(t, seed, "failed to find SEED env var")
	assert.Equal(t, "cluster1-seed-service.namespace1.svc.cluster.local", seed.Value)

	javaOpts := utils.FindEnvVarInContainer(container, "JAVA_OPTS")
	require.NotNil(t, javaOpts, "failed to find JAVA_OPTS env var")
	assert.Contains(t, javaOpts.Value, "-XX:+CrashOnOutOfMemoryError")
	assert.Contains(t, javaOpts.Value, "-Xms268435456")
	assert.Contains(t, javaOpts.Value, "-Xmx268435456")

	volumeMount := findVolumeMount(container, "cassandra-config")
	require.NotNil(t, volumeMount)

	volume := findVolume(&deployment, "cassandra-config")
	require.NotNil(t, volume)
}

func testNewDeploymentsSingleRackManyReplicas(t *testing.T) {

	dc := dc.DeepCopy()
	dc.Spec.Size = 3
	dc.Spec.Racks = []cassdcapi.Rack{{Name: "rack1"}}

	stargate := stargate.DeepCopy()
	stargate.Spec.Size = 3

	deployments := NewDeployments(stargate, dc)
	require.Len(t, deployments, 1)
	require.Contains(t, deployments, "cluster1-dc1-rack1-stargate-deployment")
	deployment := deployments["cluster1-dc1-rack1-stargate-deployment"]

	assert.Equal(t, "cluster1-dc1-rack1-stargate-deployment", deployment.Name)
	assert.Equal(t, namespace, deployment.Namespace)
	assert.Contains(t, deployment.Labels, api.StargateLabel)
	assert.Equal(t, "s1", deployment.Labels[api.StargateLabel])

	assert.EqualValues(t, 3, *deployment.Spec.Replicas)

	assert.Contains(t, deployment.Spec.Selector.MatchLabels, api.StargateDeploymentLabel)
	assert.Equal(t, "cluster1-dc1-rack1-stargate-deployment", deployment.Spec.Selector.MatchLabels[api.StargateDeploymentLabel])

	assert.Contains(t, deployment.Spec.Template.Labels, api.StargateLabel)
	assert.Equal(t, "s1", deployment.Spec.Template.Labels[api.StargateLabel])
	assert.Contains(t, deployment.Spec.Template.Labels, api.StargateDeploymentLabel)
	assert.Equal(t, "cluster1-dc1-rack1-stargate-deployment", deployment.Spec.Template.Labels[api.StargateDeploymentLabel])

	assert.Equal(t, affinityForRack(dc, "rack1"), deployment.Spec.Template.Spec.Affinity)
	assert.Nil(t, deployment.Spec.Template.Spec.Tolerations)

	container := findContainer(&deployment, deployment.Name)
	require.NotNil(t, container, "failed to find stargate container")

	rackName := utils.FindEnvVarInContainer(container, "RACK_NAME")
	require.NotNil(t, rackName, "failed to find RACK_NAME env var")
	assert.Equal(t, "rack1", rackName.Value)

	seed := utils.FindEnvVarInContainer(container, "SEED")
	require.NotNil(t, seed, "failed to find SEED env var")
	assert.Equal(t, "cluster1-seed-service.namespace1.svc.cluster.local", seed.Value)

}

func testNewDeploymentsManyRacksManyReplicas(t *testing.T) {

	dc := dc.DeepCopy()
	dc.Spec.Size = 9
	dc.Spec.Racks = []cassdcapi.Rack{
		{Name: "rack1"},
		{Name: "rack2"},
		{Name: "rack3"},
	}
	stargate := stargate.DeepCopy()
	stargate.Spec.Size = 8

	deployments := NewDeployments(stargate, dc)

	require.Len(t, deployments, 3)
	require.Contains(t, deployments, "cluster1-dc1-rack1-stargate-deployment")
	require.Contains(t, deployments, "cluster1-dc1-rack2-stargate-deployment")
	require.Contains(t, deployments, "cluster1-dc1-rack3-stargate-deployment")

	deployment1 := deployments["cluster1-dc1-rack1-stargate-deployment"]
	assert.Equal(t, "cluster1-dc1-rack1-stargate-deployment", deployment1.Name)
	assert.EqualValues(t, 3, *deployment1.Spec.Replicas)
	assert.Contains(t, deployment1.Spec.Selector.MatchLabels, api.StargateDeploymentLabel)
	assert.Equal(t, "cluster1-dc1-rack1-stargate-deployment", deployment1.Spec.Selector.MatchLabels[api.StargateDeploymentLabel])
	assert.Contains(t, deployment1.Spec.Template.Labels, api.StargateLabel)
	assert.Equal(t, "s1", deployment1.Spec.Template.Labels[api.StargateLabel])
	assert.Contains(t, deployment1.Spec.Template.Labels, api.StargateDeploymentLabel)
	assert.Equal(t, "cluster1-dc1-rack1-stargate-deployment", deployment1.Spec.Template.Labels[api.StargateDeploymentLabel])
	assert.Equal(t, affinityForRack(dc, "rack1"), deployment1.Spec.Template.Spec.Affinity)
	assert.Nil(t, deployment1.Spec.Template.Spec.NodeSelector)
	assert.Nil(t, deployment1.Spec.Template.Spec.Tolerations)
	container1 := findContainer(&deployment1, deployment1.Name)
	require.NotNil(t, container1, "failed to find stargate container")
	rackName1 := utils.FindEnvVarInContainer(container1, "RACK_NAME")
	require.NotNil(t, rackName1, "failed to find RACK_NAME env var")
	assert.Equal(t, "rack1", rackName1.Value)

	deployment2 := deployments["cluster1-dc1-rack2-stargate-deployment"]
	assert.Equal(t, "cluster1-dc1-rack2-stargate-deployment", deployment2.Name)
	assert.EqualValues(t, 3, *deployment2.Spec.Replicas)
	assert.Contains(t, deployment2.Spec.Selector.MatchLabels, api.StargateDeploymentLabel)
	assert.Equal(t, "cluster1-dc1-rack2-stargate-deployment", deployment2.Spec.Selector.MatchLabels[api.StargateDeploymentLabel])
	assert.Contains(t, deployment2.Spec.Template.Labels, api.StargateLabel)
	assert.Equal(t, "s1", deployment2.Spec.Template.Labels[api.StargateLabel])
	assert.Contains(t, deployment2.Spec.Template.Labels, api.StargateDeploymentLabel)
	assert.Equal(t, "cluster1-dc1-rack2-stargate-deployment", deployment2.Spec.Template.Labels[api.StargateDeploymentLabel])
	assert.Equal(t, affinityForRack(dc, "rack2"), deployment2.Spec.Template.Spec.Affinity)
	assert.Nil(t, deployment1.Spec.Template.Spec.NodeSelector)
	assert.Nil(t, deployment1.Spec.Template.Spec.Tolerations)
	container2 := findContainer(&deployment2, deployment2.Name)
	require.NotNil(t, container2, "failed to find stargate container")
	rackName2 := utils.FindEnvVarInContainer(container2, "RACK_NAME")
	require.NotNil(t, rackName2, "failed to find RACK_NAME env var")
	assert.Equal(t, "rack2", rackName2.Value)

	deployment3 := deployments["cluster1-dc1-rack3-stargate-deployment"]
	assert.Equal(t, "cluster1-dc1-rack3-stargate-deployment", deployment3.Name)
	assert.EqualValues(t, 2, *deployment3.Spec.Replicas)
	assert.Contains(t, deployment3.Spec.Selector.MatchLabels, api.StargateDeploymentLabel)
	assert.Equal(t, "cluster1-dc1-rack3-stargate-deployment", deployment3.Spec.Selector.MatchLabels[api.StargateDeploymentLabel])
	assert.Contains(t, deployment3.Spec.Template.Labels, api.StargateLabel)
	assert.Equal(t, "s1", deployment3.Spec.Template.Labels[api.StargateLabel])
	assert.Contains(t, deployment3.Spec.Template.Labels, api.StargateDeploymentLabel)
	assert.Equal(t, "cluster1-dc1-rack3-stargate-deployment", deployment3.Spec.Template.Labels[api.StargateDeploymentLabel])
	assert.Equal(t, affinityForRack(dc, "rack3"), deployment3.Spec.Template.Spec.Affinity)
	assert.Nil(t, deployment1.Spec.Template.Spec.NodeSelector)
	assert.Nil(t, deployment1.Spec.Template.Spec.Tolerations)
	container3 := findContainer(&deployment3, deployment3.Name)
	require.NotNil(t, container3, "failed to find stargate container")
	rackName3 := utils.FindEnvVarInContainer(container3, "RACK_NAME")
	require.NotNil(t, rackName3, "failed to find RACK_NAME env var")
	assert.Equal(t, "rack3", rackName3.Value)
}

func testNewDeploymentsManyRacksCustomAffinityDc(t *testing.T) {

	dc := dc.DeepCopy()
	dc.Spec.Size = 9
	//goland:noinspection GoDeprecation
	dc.Spec.Racks = []cassdcapi.Rack{
		{Name: "rack1", NodeAffinityLabels: map[string]string{"rack1label": "value1"}},
		{Name: "rack2", Zone: "zone2"},
		{Name: "rack3"},
	}
	dc.Spec.NodeAffinityLabels = map[string]string{"dc1label": "value1"}
	dc.Spec.NodeSelector = map[string]string{"selectorKey1": "selectorValue1"}
	dc.Spec.Tolerations = []corev1.Toleration{{
		Key:      "key1",
		Operator: "in",
		Value:    "value1",
	}}

	stargate := stargate.DeepCopy()
	stargate.Spec.Size = 8

	deployments := NewDeployments(stargate, dc)
	require.Len(t, deployments, 3)
	require.Contains(t, deployments, "cluster1-dc1-rack1-stargate-deployment")
	require.Contains(t, deployments, "cluster1-dc1-rack2-stargate-deployment")
	require.Contains(t, deployments, "cluster1-dc1-rack3-stargate-deployment")

	deployment1 := deployments["cluster1-dc1-rack1-stargate-deployment"]
	assert.Equal(t, "cluster1-dc1-rack1-stargate-deployment", deployment1.Name)
	assert.EqualValues(t, 3, *deployment1.Spec.Replicas)
	assert.Contains(t, deployment1.Spec.Selector.MatchLabels, api.StargateDeploymentLabel)
	assert.Equal(t, "cluster1-dc1-rack1-stargate-deployment", deployment1.Spec.Selector.MatchLabels[api.StargateDeploymentLabel])
	assert.Contains(t, deployment1.Spec.Template.Labels, api.StargateLabel)
	assert.Equal(t, "s1", deployment1.Spec.Template.Labels[api.StargateLabel])
	assert.Contains(t, deployment1.Spec.Template.Labels, api.StargateDeploymentLabel)
	assert.Equal(t, "cluster1-dc1-rack1-stargate-deployment", deployment1.Spec.Template.Labels[api.StargateDeploymentLabel])
	assert.Equal(t, affinityForRack(dc, "rack1"), deployment1.Spec.Template.Spec.Affinity)
	assert.Equal(t, dc.Spec.NodeSelector, deployment1.Spec.Template.Spec.NodeSelector)
	assert.Equal(t, dc.Spec.Tolerations, deployment1.Spec.Template.Spec.Tolerations)
	container1 := findContainer(&deployment1, deployment1.Name)
	require.NotNil(t, container1, "failed to find stargate container")
	rackName1 := utils.FindEnvVarInContainer(container1, "RACK_NAME")
	require.NotNil(t, rackName1, "failed to find RACK_NAME env var")
	assert.Equal(t, "rack1", rackName1.Value)

	deployment2 := deployments["cluster1-dc1-rack2-stargate-deployment"]
	assert.Equal(t, "cluster1-dc1-rack2-stargate-deployment", deployment2.Name)
	assert.EqualValues(t, 3, *deployment2.Spec.Replicas)
	assert.Contains(t, deployment2.Spec.Selector.MatchLabels, api.StargateDeploymentLabel)
	assert.Equal(t, "cluster1-dc1-rack2-stargate-deployment", deployment2.Spec.Selector.MatchLabels[api.StargateDeploymentLabel])
	assert.Contains(t, deployment2.Spec.Template.Labels, api.StargateLabel)
	assert.Equal(t, "s1", deployment2.Spec.Template.Labels[api.StargateLabel])
	assert.Contains(t, deployment2.Spec.Template.Labels, api.StargateDeploymentLabel)
	assert.Equal(t, "cluster1-dc1-rack2-stargate-deployment", deployment2.Spec.Template.Labels[api.StargateDeploymentLabel])
	assert.Equal(t, affinityForRack(dc, "rack2"), deployment2.Spec.Template.Spec.Affinity)
	assert.Equal(t, dc.Spec.NodeSelector, deployment2.Spec.Template.Spec.NodeSelector)
	assert.Equal(t, dc.Spec.Tolerations, deployment2.Spec.Template.Spec.Tolerations)
	container2 := findContainer(&deployment2, deployment2.Name)
	require.NotNil(t, container2, "failed to find stargate container")
	rackName2 := utils.FindEnvVarInContainer(container2, "RACK_NAME")
	require.NotNil(t, rackName2, "failed to find RACK_NAME env var")
	assert.Equal(t, "rack2", rackName2.Value)

	deployment3 := deployments["cluster1-dc1-rack3-stargate-deployment"]
	assert.Equal(t, "cluster1-dc1-rack3-stargate-deployment", deployment3.Name)
	assert.EqualValues(t, 2, *deployment3.Spec.Replicas)
	assert.Contains(t, deployment3.Spec.Selector.MatchLabels, api.StargateDeploymentLabel)
	assert.Equal(t, "cluster1-dc1-rack3-stargate-deployment", deployment3.Spec.Selector.MatchLabels[api.StargateDeploymentLabel])
	assert.Contains(t, deployment3.Spec.Template.Labels, api.StargateLabel)
	assert.Equal(t, "s1", deployment3.Spec.Template.Labels[api.StargateLabel])
	assert.Contains(t, deployment3.Spec.Template.Labels, api.StargateDeploymentLabel)
	assert.Equal(t, "cluster1-dc1-rack3-stargate-deployment", deployment3.Spec.Template.Labels[api.StargateDeploymentLabel])
	assert.Equal(t, affinityForRack(dc, "rack3"), deployment3.Spec.Template.Spec.Affinity)
	assert.Equal(t, dc.Spec.NodeSelector, deployment3.Spec.Template.Spec.NodeSelector)
	assert.Equal(t, dc.Spec.Tolerations, deployment3.Spec.Template.Spec.Tolerations)
	container3 := findContainer(&deployment3, deployment3.Name)
	require.NotNil(t, container3, "failed to find stargate container")
	rackName3 := utils.FindEnvVarInContainer(container3, "RACK_NAME")
	require.NotNil(t, rackName3, "failed to find RACK_NAME env var")
	assert.Equal(t, "rack3", rackName3.Value)
}

func testNewDeploymentsManyRacksCustomAffinityStargate(t *testing.T) {

	dc := dc.DeepCopy()
	dc.Spec.Size = 9
	//goland:noinspection GoDeprecation
	dc.Spec.Racks = []cassdcapi.Rack{
		{Name: "rack1", NodeAffinityLabels: map[string]string{"rack1label": "value1"}},
		{Name: "rack2", Zone: "zone2"},
		{Name: "rack3"},
	}
	dc.Spec.NodeAffinityLabels = map[string]string{"dc1label": "value1"}
	dc.Spec.NodeSelector = map[string]string{"selectorKey1": "selectorValue1"}
	dc.Spec.Tolerations = []corev1.Toleration{{
		Key:      "key1",
		Operator: "in",
		Value:    "value1",
	}}

	stargate := stargate.DeepCopy()
	stargate.Spec.Size = 8
	stargate.Spec.NodeSelector = map[string]string{"selectorKey2": "selectorValue2"}
	stargate.Spec.Tolerations = []corev1.Toleration{{
		Key:      "key2",
		Operator: "in",
		Value:    "value2",
	}}
	stargate.Spec.Racks = []api.StargateRackTemplate{{
		Name: "rack3",
		StargateTemplate: api.StargateTemplate{
			NodeSelector: map[string]string{"selectorKey3": "selectorValue3"},
			Tolerations: []corev1.Toleration{{
				Key:      "key3",
				Operator: "in",
				Value:    "value3",
			}},
			Affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{{
							MatchExpressions: []corev1.NodeSelectorRequirement{{
								Key:      "nodeLabel",
								Operator: "in",
								Values:   []string{"node1"},
							}},
						}},
					},
				},
			},
		},
	}}

	deployments := NewDeployments(stargate, dc)
	require.Len(t, deployments, 3)
	require.Contains(t, deployments, "cluster1-dc1-rack1-stargate-deployment")
	require.Contains(t, deployments, "cluster1-dc1-rack2-stargate-deployment")
	require.Contains(t, deployments, "cluster1-dc1-rack3-stargate-deployment")

	deployment1 := deployments["cluster1-dc1-rack1-stargate-deployment"]
	assert.Equal(t, "cluster1-dc1-rack1-stargate-deployment", deployment1.Name)
	assert.EqualValues(t, 3, *deployment1.Spec.Replicas)
	assert.Contains(t, deployment1.Spec.Selector.MatchLabels, api.StargateDeploymentLabel)
	assert.Equal(t, "cluster1-dc1-rack1-stargate-deployment", deployment1.Spec.Selector.MatchLabels[api.StargateDeploymentLabel])
	assert.Contains(t, deployment1.Spec.Template.Labels, api.StargateLabel)
	assert.Equal(t, "s1", deployment1.Spec.Template.Labels[api.StargateLabel])
	assert.Contains(t, deployment1.Spec.Template.Labels, api.StargateDeploymentLabel)
	assert.Equal(t, "cluster1-dc1-rack1-stargate-deployment", deployment1.Spec.Template.Labels[api.StargateDeploymentLabel])
	assert.Equal(t, affinityForRack(dc, "rack1"), deployment1.Spec.Template.Spec.Affinity)
	assert.Equal(t, stargate.Spec.NodeSelector, deployment1.Spec.Template.Spec.NodeSelector)
	assert.Equal(t, stargate.Spec.Tolerations, deployment1.Spec.Template.Spec.Tolerations)
	container1 := findContainer(&deployment1, deployment1.Name)
	require.NotNil(t, container1, "failed to find stargate container")
	rackName1 := utils.FindEnvVarInContainer(container1, "RACK_NAME")
	require.NotNil(t, rackName1, "failed to find RACK_NAME env var")
	assert.Equal(t, "rack1", rackName1.Value)

	deployment2 := deployments["cluster1-dc1-rack2-stargate-deployment"]
	assert.Equal(t, "cluster1-dc1-rack2-stargate-deployment", deployment2.Name)
	assert.EqualValues(t, 3, *deployment2.Spec.Replicas)
	assert.Contains(t, deployment2.Spec.Selector.MatchLabels, api.StargateDeploymentLabel)
	assert.Equal(t, "cluster1-dc1-rack2-stargate-deployment", deployment2.Spec.Selector.MatchLabels[api.StargateDeploymentLabel])
	assert.Contains(t, deployment2.Spec.Template.Labels, api.StargateLabel)
	assert.Equal(t, "s1", deployment2.Spec.Template.Labels[api.StargateLabel])
	assert.Contains(t, deployment2.Spec.Template.Labels, api.StargateDeploymentLabel)
	assert.Equal(t, "cluster1-dc1-rack2-stargate-deployment", deployment2.Spec.Template.Labels[api.StargateDeploymentLabel])
	assert.Equal(t, affinityForRack(dc, "rack2"), deployment2.Spec.Template.Spec.Affinity)
	assert.Equal(t, stargate.Spec.NodeSelector, deployment2.Spec.Template.Spec.NodeSelector)
	assert.Equal(t, stargate.Spec.Tolerations, deployment2.Spec.Template.Spec.Tolerations)
	container2 := findContainer(&deployment2, deployment2.Name)
	require.NotNil(t, container2, "failed to find stargate container")
	rackName2 := utils.FindEnvVarInContainer(container2, "RACK_NAME")
	require.NotNil(t, rackName2, "failed to find RACK_NAME env var")
	assert.Equal(t, "rack2", rackName2.Value)

	deployment3 := deployments["cluster1-dc1-rack3-stargate-deployment"]
	assert.Equal(t, "cluster1-dc1-rack3-stargate-deployment", deployment3.Name)
	assert.EqualValues(t, 2, *deployment3.Spec.Replicas)
	assert.Contains(t, deployment3.Spec.Selector.MatchLabels, api.StargateDeploymentLabel)
	assert.Equal(t, "cluster1-dc1-rack3-stargate-deployment", deployment3.Spec.Selector.MatchLabels[api.StargateDeploymentLabel])
	assert.Contains(t, deployment3.Spec.Template.Labels, api.StargateLabel)
	assert.Equal(t, "s1", deployment3.Spec.Template.Labels[api.StargateLabel])
	assert.Contains(t, deployment3.Spec.Template.Labels, api.StargateDeploymentLabel)
	assert.Equal(t, "cluster1-dc1-rack3-stargate-deployment", deployment3.Spec.Template.Labels[api.StargateDeploymentLabel])
	assert.Equal(t, stargate.Spec.Racks[0].Affinity, deployment3.Spec.Template.Spec.Affinity)
	assert.Equal(t, stargate.Spec.Racks[0].NodeSelector, deployment3.Spec.Template.Spec.NodeSelector)
	assert.Equal(t, stargate.Spec.Racks[0].Tolerations, deployment3.Spec.Template.Spec.Tolerations)
	container3 := findContainer(&deployment3, deployment3.Name)
	require.NotNil(t, container3, "failed to find stargate container")
	rackName3 := utils.FindEnvVarInContainer(container3, "RACK_NAME")
	require.NotNil(t, rackName3, "failed to find RACK_NAME env var")
	assert.Equal(t, "rack3", rackName3.Value)
}

func testNewDeploymentsManyRacksFewReplicas(t *testing.T) {

	dc := dc.DeepCopy()
	dc.Spec.Size = 9
	dc.Spec.Racks = []cassdcapi.Rack{
		{Name: "rack1"},
		{Name: "rack2"},
		{Name: "rack3"},
	}

	stargate := stargate.DeepCopy()
	stargate.Spec.Size = 2 // rack3 will get no deployment

	deployments := NewDeployments(stargate, dc)
	require.Len(t, deployments, 2)
	require.Contains(t, deployments, "cluster1-dc1-rack1-stargate-deployment")
	require.Contains(t, deployments, "cluster1-dc1-rack2-stargate-deployment")

	deployment1 := deployments["cluster1-dc1-rack1-stargate-deployment"]
	assert.Equal(t, "cluster1-dc1-rack1-stargate-deployment", deployment1.Name)
	assert.EqualValues(t, 1, *deployment1.Spec.Replicas)

	deployment2 := deployments["cluster1-dc1-rack2-stargate-deployment"]
	assert.Equal(t, "cluster1-dc1-rack2-stargate-deployment", deployment2.Name)
	assert.EqualValues(t, 1, *deployment2.Spec.Replicas)
}

func testNewDeploymentsCassandraConfigMap(t *testing.T) {
	configMapName := "cassandra-config"
	generatedConfigMapName := "cluster1-dc1-cassandra-config"

	stargate := stargate.DeepCopy()
	stargate.Spec.CassandraConfigMapRef = &corev1.LocalObjectReference{Name: configMapName}

	deployments := NewDeployments(stargate, dc)
	require.Len(t, deployments, 1)
	deployment := deployments["cluster1-dc1-default-stargate-deployment"]

	container := findContainer(&deployment, deployment.Name)
	require.NotNil(t, container, "failed to find stargate container")

	envVar := utils.FindEnvVarInContainer(container, "JAVA_OPTS")
	require.NotNil(t, envVar, "failed to find JAVA_OPTS env var")
	assert.True(t, strings.Contains(envVar.Value, "-Dstargate.unsafe.cassandra_config_path="+cassandraConfigPath))

	volumeMount := findVolumeMount(container, "cassandra-config")
	require.NotNil(t, volumeMount, "failed to find cassandra-config volume mount")
	assert.Equal(t, "/config", volumeMount.MountPath)

	volume := findVolume(&deployment, "cassandra-config")
	require.NotNil(t, volume, "failed to find cassandra-config volume")
	expected := corev1.Volume{
		Name: "cassandra-config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: generatedConfigMapName},
			},
		},
	}
	assert.Equal(t, expected, *volume, "cassandra-config volume does not match expected value")
}

func testNewDeploymentsEncryption(t *testing.T) {

	stargate := stargate.DeepCopy()

	deployments := NewDeployments(stargate, dc)
	require.Len(t, deployments, 1)
	deployment := deployments["cluster1-dc1-default-stargate-deployment"]

	container := findContainer(&deployment, deployment.Name)
	require.NotNil(t, container, "failed to find stargate container")

	/* envVar := utils.FindEnvVarInContainer(container, "JAVA_OPTS")
	require.NotNil(t, envVar, "failed to find JAVA_OPTS env var")
	assert.True(t, strings.Contains(envVar.Value, "-Dstargate.unsafe.cassandra_config_path="+cassandraConfigPath)) */

	volumeMount := findVolumeMount(container, "server-keystore")
	require.NotNil(t, volumeMount, "failed to find server keystore volume mount")
	assert.Equal(t, "/mnt/server-keystore", volumeMount.MountPath)

	volumeMount = findVolumeMount(container, "server-truststore")
	require.NotNil(t, volumeMount, "failed to find server truststore volume mount")
	assert.Equal(t, "/mnt/server-truststore", volumeMount.MountPath)

	volumeMount = findVolumeMount(container, "client-keystore")
	require.NotNil(t, volumeMount, "failed to find client keystore volume mount")
	assert.Equal(t, "/mnt/client-keystore", volumeMount.MountPath)

	volumeMount = findVolumeMount(container, "client-truststore")
	require.NotNil(t, volumeMount, "failed to find client truststore volume mount")
	assert.Equal(t, "/mnt/client-truststore", volumeMount.MountPath)

	volume := findVolume(&deployment, "client-keystore")
	require.NotNil(t, volume, "failed to find client-keystore volume")
	expectedKeystoreVolume := corev1.Volume{
		Name: "client-keystore",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "client-keystore-secret",
				Items: []corev1.KeyToPath{
					{
						Key:  string(encryption.StoreNameKeystore),
						Path: string(encryption.StoreNameKeystore),
					},
				},
			},
		},
	}
	assert.Equal(t, expectedKeystoreVolume, *volume, "client keystore volume does not match expected value")

	volume = findVolume(&deployment, "client-truststore")
	require.NotNil(t, volume, "failed to find client-truststore volume")
	expectedTruststoreVolume := corev1.Volume{
		Name: "client-truststore",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "client-truststore-secret",
				Items: []corev1.KeyToPath{
					{
						Key:  string(encryption.StoreNameTruststore),
						Path: string(encryption.StoreNameTruststore),
					},
				},
			},
		},
	}
	assert.Equal(t, expectedTruststoreVolume, *volume, "client truststore volume does not match expected value")

	volume = findVolume(&deployment, "server-keystore")
	require.NotNil(t, volume, "failed to find server-keystore volume")
	expectedKeystoreVolume = corev1.Volume{
		Name: "server-keystore",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "server-keystore-secret",
				Items: []corev1.KeyToPath{
					{
						Key:  string(encryption.StoreNameKeystore),
						Path: string(encryption.StoreNameKeystore),
					},
				},
			},
		},
	}
	assert.Equal(t, expectedKeystoreVolume, *volume, "server keystore volume does not match expected value")

	volume = findVolume(&deployment, "server-truststore")
	require.NotNil(t, volume, "failed to find server-truststore volume")
	expectedTruststoreVolume = corev1.Volume{
		Name: "server-truststore",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "server-truststore-secret",
				Items: []corev1.KeyToPath{
					{
						Key:  string(encryption.StoreNameTruststore),
						Path: string(encryption.StoreNameTruststore),
					},
				},
			},
		},
	}
	assert.Equal(t, expectedTruststoreVolume, *volume, "server truststore volume does not match expected value")
}

func testNewDeploymentsAuthentication(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		sg := stargate.DeepCopy()
		sg.Spec.Auth = pointer.Bool(false)
		deployments := NewDeployments(sg, dc)
		require.Len(t, deployments, 1)
		deployment := deployments["cluster1-dc1-default-stargate-deployment"]
		javaOpts := utils.FindEnvVarInContainer(&deployment.Spec.Template.Spec.Containers[0], "JAVA_OPTS")
		require.NotNil(t, javaOpts, "failed to find JAVA_OPTS env var")
		assert.NotContains(t, javaOpts.Value, "-Dstargate.auth_id")
	})
	t.Run("table-based", func(t *testing.T) {
		sg := stargate.DeepCopy()
		sg.Spec.Auth = pointer.Bool(true)
		sg.Spec.AuthOptions = &api.AuthOptions{
			ApiAuthMethod:   "Table",
			TokenTtlSeconds: 123,
		}
		deployments := NewDeployments(sg, dc)
		require.Len(t, deployments, 1)
		deployment := deployments["cluster1-dc1-default-stargate-deployment"]
		javaOpts := utils.FindEnvVarInContainer(&deployment.Spec.Template.Spec.Containers[0], "JAVA_OPTS")
		require.NotNil(t, javaOpts, "failed to find JAVA_OPTS env var")
		assert.Contains(t, javaOpts.Value, "-Dstargate.auth_id=AuthTableBasedService")
		assert.Contains(t, javaOpts.Value, "-Dstargate.auth_tokenttl=123")
	})
	t.Run("JWT-based", func(t *testing.T) {
		sg := stargate.DeepCopy()
		sg.Spec.Auth = pointer.Bool(true)
		sg.Spec.AuthOptions = &api.AuthOptions{
			ApiAuthMethod:  "JWT",
			JwtProviderUrl: "https://auth.example.com/auth/realms/stargate/protocol/openid-connect/token",
		}
		deployments := NewDeployments(sg, dc)
		require.Len(t, deployments, 1)
		deployment := deployments["cluster1-dc1-default-stargate-deployment"]
		javaOpts := utils.FindEnvVarInContainer(&deployment.Spec.Template.Spec.Containers[0], "JAVA_OPTS")
		require.NotNil(t, javaOpts, "failed to find JAVA_OPTS env var")
		assert.Contains(t, javaOpts.Value, "-Dstargate.auth_id=AuthJwtService")
		assert.Contains(t, javaOpts.Value, "-Dstargate.auth.jwt_provider_url=https://auth.example.com/auth/realms/stargate/protocol/openid-connect/token")
	})

}

func testImages(t *testing.T) {
	// Note: a nil image is normally not possible due to the kubebuilder marker on the CRD spec
	t.Run("nil image 3", func(t *testing.T) {
		stargate := stargate.DeepCopy()
		stargate.Spec.ContainerImage = nil
		deployments := NewDeployments(stargate, dc)
		require.Len(t, deployments, 1)
		deployment := deployments["cluster1-dc1-default-stargate-deployment"]
		assert.Equal(t, defaultImage3.String(), deployment.Spec.Template.Spec.Containers[0].Image)
		assert.Equal(t, corev1.PullIfNotPresent, deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy)
		assert.Empty(t, deployment.Spec.Template.Spec.ImagePullSecrets)
	})
	t.Run("nil image 4", func(t *testing.T) {
		stargate := stargate.DeepCopy()
		stargate.Spec.ContainerImage = nil
		dc := dc.DeepCopy()
		dc.Spec.ServerVersion = "4.0.1"
		deployments := NewDeployments(stargate, dc)
		require.Len(t, deployments, 1)
		deployment := deployments["cluster1-dc1-default-stargate-deployment"]
		assert.Equal(t, defaultImage4.String(), deployment.Spec.Template.Spec.Containers[0].Image)
		assert.Equal(t, corev1.PullIfNotPresent, deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy)
		assert.Empty(t, deployment.Spec.Template.Spec.ImagePullSecrets)
	})
	t.Run("default image 3", func(t *testing.T) {
		stargate := stargate.DeepCopy()
		stargate.Spec.ContainerImage = &images.Image{
			Repository: "stargateio",
			Tag:        "v" + DefaultVersion,
		}
		deployments := NewDeployments(stargate, dc)
		require.Len(t, deployments, 1)
		deployment := deployments["cluster1-dc1-default-stargate-deployment"]
		assert.Equal(t, defaultImage3.String(), deployment.Spec.Template.Spec.Containers[0].Image)
		assert.Equal(t, corev1.PullIfNotPresent, deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy)
		assert.Empty(t, deployment.Spec.Template.Spec.ImagePullSecrets)
	})
	t.Run("default image 4", func(t *testing.T) {
		stargate := stargate.DeepCopy()
		stargate.Spec.ContainerImage = &images.Image{
			Repository: "stargateio",
			Tag:        "v" + DefaultVersion,
		}
		dc := dc.DeepCopy()
		dc.Spec.ServerVersion = "4.0.1"
		deployments := NewDeployments(stargate, dc)
		require.Len(t, deployments, 1)
		deployment := deployments["cluster1-dc1-default-stargate-deployment"]
		assert.Equal(t, defaultImage4.String(), deployment.Spec.Template.Spec.Containers[0].Image)
		assert.Equal(t, corev1.PullIfNotPresent, deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy)
		assert.Empty(t, deployment.Spec.Template.Spec.ImagePullSecrets)
	})
	t.Run("custom image 3", func(t *testing.T) {
		stargate := stargate.DeepCopy()
		image := &images.Image{
			Repository:    "my-custom-repo",
			Tag:           "latest",
			PullSecretRef: &corev1.LocalObjectReference{Name: "my-secret"},
		}
		stargate.Spec.ContainerImage = image
		deployments := NewDeployments(stargate, dc)
		require.Len(t, deployments, 1)
		deployment := deployments["cluster1-dc1-default-stargate-deployment"]
		assert.Equal(t, "docker.io/my-custom-repo/stargate-3_11:latest", deployment.Spec.Template.Spec.Containers[0].Image)
		assert.Equal(t, corev1.PullAlways, deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy)
		assert.Contains(t, deployment.Spec.Template.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: "my-secret"})
		assert.Len(t, deployment.Spec.Template.Spec.ImagePullSecrets, 1)
	})
	t.Run("custom image 4", func(t *testing.T) {
		stargate := stargate.DeepCopy()
		image := &images.Image{
			Repository:    "my-custom-repo",
			Tag:           "latest",
			PullSecretRef: &corev1.LocalObjectReference{Name: "my-secret"},
		}
		stargate.Spec.ContainerImage = image
		dc := dc.DeepCopy()
		dc.Spec.ServerVersion = "4.0.1"
		deployments := NewDeployments(stargate, dc)
		require.Len(t, deployments, 1)
		deployment := deployments["cluster1-dc1-default-stargate-deployment"]
		assert.Equal(t, "docker.io/my-custom-repo/stargate-4_0:latest", deployment.Spec.Template.Spec.Containers[0].Image)
		assert.Equal(t, corev1.PullAlways, deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy)
		assert.Contains(t, deployment.Spec.Template.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: "my-secret"})
		assert.Len(t, deployment.Spec.Template.Spec.ImagePullSecrets, 1)
	})
}

func findContainer(deployment *appsv1.Deployment, name string) *corev1.Container {
	for _, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name == name {
			return &c
		}
	}
	return nil
}

func findVolumeMount(container *corev1.Container, name string) *corev1.VolumeMount {
	for _, v := range container.VolumeMounts {
		if v.Name == name {
			return &v
		}
	}
	return nil
}

func findVolume(deployment *appsv1.Deployment, name string) *corev1.Volume {
	for _, v := range deployment.Spec.Template.Spec.Volumes {
		if v.Name == name {
			return &v
		}
	}
	return nil
}

func affinityForRack(dc *cassdcapi.CassandraDatacenter, rackName string) *corev1.Affinity {
	return &corev1.Affinity{
		NodeAffinity:    computeNodeAffinity(dc, rackName),
		PodAntiAffinity: computePodAntiAffinity(false, dc, rackName),
	}
}
