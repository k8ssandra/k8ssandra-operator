package reaper

import (
	"context"
	"testing"
	"time"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	"github.com/k8ssandra/k8ssandra-operator/pkg/mocks"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reaper"
	testutils "github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	reaperName              = "test-reaper"
	cassandraClusterName    = "test-cluster"
	cassandraDatacenterName = "test-dc"

	timeout  = time.Second * 5
	interval = time.Millisecond * 250
)

var currentTest *testing.T

func TestReaper(t *testing.T) {
	ctx := testutils.TestSetup(t)
	ctx, cancel := context.WithCancel(ctx)
	testEnv := &testutils.TestEnv{}
	err := testEnv.Start(ctx, t, func(mgr manager.Manager) error {
		err := (&ReaperReconciler{
			ReconcilerConfig: config.InitConfig(),
			Client:           mgr.GetClient(),
			Scheme:           mgr.GetScheme(),
			NewManager:       newMockManager,
		}).SetupWithManager(mgr)
		return err
	})
	if err != nil {
		t.Fatalf("failed to start test environment: %s", err)
	}

	defer testEnv.Stop(t)
	defer cancel()

	t.Run("CreateReaper", reaperControllerTest(ctx, testEnv, testCreateReaper))
	t.Run("CreateReaperWithExistingObjects", reaperControllerTest(ctx, testEnv, testCreateReaperWithExistingObjects))
	t.Run("CreateReaperWithAutoSchedulingEnabled", reaperControllerTest(ctx, testEnv, testCreateReaperWithAutoSchedulingEnabled))
	t.Run("CreateReaperWithAuthEnabled", reaperControllerTest(ctx, testEnv, testCreateReaperWithAuthEnabled))
}

func newMockManager() reaper.Manager {
	m := new(mocks.ReaperManager)
	m.On("Connect", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	m.On("AddClusterToReaper", mock.Anything, mock.Anything).Return(nil)
	m.On("VerifyClusterIsConfigured", mock.Anything, mock.Anything).Return(true, nil)
	m.Test(currentTest)
	return m
}

func reaperControllerTest(ctx context.Context, env *testutils.TestEnv, test func(t *testing.T, ctx context.Context, k8sClient client.Client, testNamespace string)) func(t *testing.T) {
	return func(t *testing.T) {
		testNamespace := "ns-" + rand.String(6)
		beforeTest(t, ctx, env.TestClient, testNamespace)
		test(t, ctx, env.TestClient, testNamespace)
	}
}

func beforeTest(t *testing.T, ctx context.Context, k8sClient client.Client, testNamespace string) {
	currentTest = t
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}
	err := k8sClient.Create(ctx, ns)
	require.NoError(t, err)

	testDc := &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      cassandraDatacenterName,
		},
		Spec: cassdcapi.CassandraDatacenterSpec{
			ClusterName:   cassandraClusterName,
			ServerType:    "cassandra",
			ServerVersion: "3.11.7",
			Size:          3,
		},
	}
	err = k8sClient.Create(ctx, testDc)
	require.NoError(t, err)

	patchCassdc := client.MergeFrom(testDc.DeepCopy())
	testDc.Status.CassandraOperatorProgress = cassdcapi.ProgressReady
	testDc.Status.Conditions = []cassdcapi.DatacenterCondition{{
		Status: corev1.ConditionTrue,
		Type:   cassdcapi.DatacenterReady,
	}}

	err = k8sClient.Status().Patch(ctx, testDc, patchCassdc)
	require.NoError(t, err)

	cassdcKey := types.NamespacedName{Namespace: testNamespace, Name: cassandraDatacenterName}
	cassdc := &cassdcapi.CassandraDatacenter{}
	assert.Eventually(t, func() bool {
		err := k8sClient.Get(ctx, cassdcKey, cassdc)
		if err != nil {
			return false
		}
		return cassdc.Status.CassandraOperatorProgress == cassdcapi.ProgressReady
	}, timeout, interval)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cassdc-pod1",
			Namespace: testNamespace,
			Labels: map[string]string{
				cassdcapi.ClusterLabel:    cassandraClusterName,
				cassdcapi.DatacenterLabel: cassandraDatacenterName,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "cassandra",
				Image: "k8ssandra/cassandra-nothere:latest",
			}},
		},
	}

	err = k8sClient.Create(ctx, pod)
	require.NoError(t, err)

	podIP := "127.0.0.1"

	patchPod := client.MergeFrom(pod.DeepCopy())
	pod.Status = corev1.PodStatus{
		PodIP:  podIP,
		PodIPs: []corev1.PodIP{{IP: podIP}}}
	err = k8sClient.Status().Patch(ctx, pod, patchPod)
	require.NoError(t, err)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-dc-test-dc-all-pods-service",
			Namespace: testNamespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{Name: "mgmt-api-http", Port: int32(8080)}},
			Selector: map[string]string{
				cassdcapi.ClusterLabel:    cassandraClusterName,
				cassdcapi.DatacenterLabel: cassandraDatacenterName,
			},
		},
	}
	err = k8sClient.Create(ctx, service)
	require.NoError(t, err)
}

func testCreateReaper(t *testing.T, ctx context.Context, k8sClient client.Client, testNamespace string) {
	rpr := newReaper(testNamespace)
	err := k8sClient.Create(ctx, rpr)
	require.NoError(t, err)

	t.Log("check that the service is created")
	serviceKey := types.NamespacedName{Namespace: testNamespace, Name: reaper.GetServiceName(rpr.Name)}
	service := &corev1.Service{}

	require.Eventually(t, func() bool {
		return k8sClient.Get(ctx, serviceKey, service) == nil
	}, timeout, interval, "service creation check failed")

	assert.Len(t, service.OwnerReferences, 1, "service owner reference not set")
	assert.Equal(t, rpr.UID, service.OwnerReferences[0].UID, "service owner reference has wrong uid")

	t.Log("check that the deployment is created")
	deploymentKey := types.NamespacedName{Namespace: testNamespace, Name: reaperName}
	deployment := &appsv1.Deployment{}

	require.Eventually(t, func() bool {
		return k8sClient.Get(ctx, deploymentKey, deployment) == nil
	}, timeout, interval, "deployment creation check failed")

	assert.Len(t, deployment.OwnerReferences, 1, "deployment owner reference not set")
	assert.Equal(t, rpr.UID, deployment.OwnerReferences[0].UID, "deployment owner reference has wrong uid")

	// init container should be a default image and thus should not contain the latest tag; pull policy should be the
	// default one (IfNotPresent)
	assert.Equal(t, "docker.io/thelastpickle/cassandra-reaper:"+reaper.DefaultVersion, deployment.Spec.Template.Spec.InitContainers[0].Image)
	assert.Equal(t, corev1.PullIfNotPresent, deployment.Spec.Template.Spec.InitContainers[0].ImagePullPolicy)

	// main container is a custom image where the tag isn't specified, so it should default to latest, and pull policy
	// to Always.
	assert.Equal(t, "docker.io/thelastpickle/cassandra-reaper-custom:latest", deployment.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, corev1.PullAlways, deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy)
	// one secret should have been collected, from the main container image
	assert.Equal(t, []corev1.LocalObjectReference{{Name: "main-secret"}}, deployment.Spec.Template.Spec.ImagePullSecrets)

	t.Log("update deployment to be ready")
	patchDeploymentStatus(t, ctx, deployment, 1, 1, k8sClient)

	verifyReaperReady(t, ctx, k8sClient, testNamespace)

	// Now simulate the Reaper app entering a state in which its readiness probe fails. This
	// should cause the deployment to have its status updated. The Reaper object's .Status.Ready
	// field should subsequently be updated.
	t.Log("update deployment to be not ready")
	patchDeploymentStatus(t, ctx, deployment, 1, 0, k8sClient)

	reaperKey := types.NamespacedName{Namespace: testNamespace, Name: reaperName}
	updatedReaper := &reaperapi.Reaper{}
	require.Eventually(t, func() bool {
		err := k8sClient.Get(ctx, reaperKey, updatedReaper)
		if err != nil {
			return false
		}
		return updatedReaper.Status.IsReady()
	}, timeout, interval, "reaper status should have been updated")
}

// The purpose of this testutils is to cover code paths where an object, e.g., the
// deployment already exists. This could happen after a failed reconciliation and
// the request gets requeued.
func testCreateReaperWithExistingObjects(t *testing.T, ctx context.Context, k8sClient client.Client, testNamespace string) {

	t.Log("create the service")
	serviceKey := types.NamespacedName{Namespace: testNamespace, Name: reaper.GetServiceName(reaperName)}
	// We can use a fake service here with only the required properties set. Since the service already
	// exists, the reconciler should continue its work. There are unit tests to verify that the service
	// is created as expected.
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: serviceKey.Namespace,
			Name:      serviceKey.Name,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name:     "fake-port",
				Protocol: corev1.ProtocolTCP,
				Port:     8888,
			},
			}},
	}
	err := k8sClient.Create(ctx, service)
	require.NoError(t, err)

	t.Log("create the deployment")
	// We can use a fake deployment here with only the required properties set. Since the deployment
	// already exists, the reconciler will just check that it is ready. There are unit tests to
	// verify that the deployment is created as expected.
	labels := map[string]string{
		reaperapi.ReaperLabel:       reaperName,
		k8ssandraapi.ManagedByLabel: k8ssandraapi.NameLabelValue,
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      reaperName,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      k8ssandraapi.ManagedByLabel,
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{k8ssandraapi.NameLabelValue},
					},
					{
						Key:      reaperapi.ReaperLabel,
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{reaperName},
					},
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "fake-deployment",
						Image: "fake-deployment:test",
					}},
				},
			},
		},
	}
	err = k8sClient.Create(ctx, deployment)
	require.NoError(t, err)

	// We need to mock the deployment being ready in order for Reaper status to be updated
	t.Log("update deployment to be ready")
	patchDeploymentStatus(t, ctx, deployment, 1, 1, k8sClient)

	t.Log("create the Reaper object")
	rpr := newReaper(testNamespace)
	err = k8sClient.Create(ctx, rpr)
	require.NoError(t, err)

	verifyReaperReady(t, ctx, k8sClient, testNamespace)
}

func testCreateReaperWithAutoSchedulingEnabled(t *testing.T, ctx context.Context, k8sClient client.Client, testNamespace string) {
	t.Log("create the Reaper object")
	rpr := newReaper(testNamespace)
	rpr.Spec.AutoScheduling = reaperapi.AutoScheduling{
		Enabled: true,
	}
	err := k8sClient.Create(ctx, rpr)
	require.NoError(t, err)

	t.Log("check that the deployment is created")
	deploymentKey := types.NamespacedName{Namespace: testNamespace, Name: reaperName}
	deployment := &appsv1.Deployment{}

	require.Eventually(t, func() bool {
		return k8sClient.Get(ctx, deploymentKey, deployment) == nil
	}, timeout, interval, "deployment creation check failed")

	assert.Len(t, deployment.Spec.Template.Spec.Containers, 1)

	autoSchedulingEnabled := false
	for _, env := range deployment.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "REAPER_AUTO_SCHEDULING_ENABLED" && env.Value == "true" {
			autoSchedulingEnabled = true
		}
	}
	assert.True(t, autoSchedulingEnabled)
}

func testCreateReaperWithAuthEnabled(t *testing.T, ctx context.Context, k8sClient client.Client, testNamespace string) {
	t.Log("creating reaper secrets")
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "top-secret-cass",
		},
		Data: map[string][]byte{
			"username": []byte("bond"),
			"password": []byte("james"),
		},
	}
	err := k8sClient.Create(ctx, &secret)
	require.NoError(t, err)
	secret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "top-secret-jmx",
		},
		Data: map[string][]byte{
			"username": []byte("bond"),
			"password": []byte("james"),
		},
	}
	err = k8sClient.Create(ctx, &secret)
	require.NoError(t, err)
	secret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "top-secret-ui",
		},
		Data: map[string][]byte{
			"username": []byte("bond"),
			"password": []byte("james"),
		},
	}
	err = k8sClient.Create(ctx, &secret)
	require.NoError(t, err)

	t.Log("create the Reaper object and modify it")
	rpr := newReaper(testNamespace)
	rpr.Spec.CassandraUserSecretRef.Name = "top-secret-cass"
	rpr.Spec.JmxUserSecretRef.Name = "top-secret-jmx"
	rpr.Spec.UiUserSecretRef.Name = "top-secret-ui"
	err = k8sClient.Create(ctx, rpr)
	require.NoError(t, err)

	t.Log("check that the deployment is created")
	deploymentKey := types.NamespacedName{Namespace: testNamespace, Name: reaperName}
	deployment := &appsv1.Deployment{}

	require.Eventually(t, func() bool {
		return k8sClient.Get(ctx, deploymentKey, deployment) == nil
	}, timeout, interval, "deployment creation check failed")

	t.Log("verify the deployment has CassAuth EnvVars")
	envVars := deployment.Spec.Template.Spec.Containers[0].Env

	assert.True(t, envVarExists(envVars, "REAPER_CASS_AUTH_USERNAME"), "Cassandra auth username env var not found")
	assert.True(t, envVarSecretHasName(envVars, "REAPER_CASS_AUTH_USERNAME", "top-secret-cass"), "Cassandra auth username env var secret name not found")
	assert.True(t, envVarSecretHasKey(envVars, "REAPER_CASS_AUTH_USERNAME", "username"), "Cassandra auth username env var secret key not found")

	assert.True(t, envVarExists(envVars, "REAPER_CASS_AUTH_PASSWORD"), "Cassandra auth password env var not found")
	assert.True(t, envVarSecretHasName(envVars, "REAPER_CASS_AUTH_PASSWORD", "top-secret-cass"), "Cassandra auth password env var secret name not found")
	assert.True(t, envVarSecretHasKey(envVars, "REAPER_CASS_AUTH_PASSWORD", "password"), "Cassandra auth password env var secret key not found")

	assert.True(t, envVarExists(envVars, "REAPER_CASS_AUTH_ENABLED"), "Cassandra auth enabled env var not found")
	assert.True(t, envVarHasValue(envVars, "REAPER_CASS_AUTH_ENABLED", "true"), "Cassandra auth enabled env var is not set to true")

	assert.True(t, envVarExists(envVars, "REAPER_JMX_AUTH_USERNAME"), "Cassandra auth username env var not found")
	assert.True(t, envVarSecretHasName(envVars, "REAPER_JMX_AUTH_USERNAME", "top-secret-jmx"), "Cassandra jmx username env var secret name not found")
	assert.True(t, envVarSecretHasKey(envVars, "REAPER_JMX_AUTH_USERNAME", "username"), "Cassandra jmx username env var secret key not found")

	assert.True(t, envVarExists(envVars, "REAPER_JMX_AUTH_PASSWORD"), "Cassandra auth password env var not found")
	assert.True(t, envVarSecretHasName(envVars, "REAPER_JMX_AUTH_PASSWORD", "top-secret-jmx"), "Cassandra jmx password env var secret name not found")
	assert.True(t, envVarSecretHasKey(envVars, "REAPER_JMX_AUTH_PASSWORD", "password"), "Cassandra jmx password env var secret key not found")

	assert.True(t, envVarExists(envVars, "REAPER_AUTH_ENABLED"), "Reaper auth enabled env var not found")
	assert.True(t, envVarHasValue(envVars, "REAPER_AUTH_ENABLED", "true"), "Reaper auth enabled env var is not set to true")

	assert.True(t, envVarExists(envVars, "REAPER_AUTH_USER"), "Reaper auth user env var not found")
	assert.True(t, envVarSecretHasName(envVars, "REAPER_AUTH_USER", "top-secret-ui"), "Reaper auth user env var secret name not found")
	assert.True(t, envVarSecretHasKey(envVars, "REAPER_AUTH_USER", "username"), "Cassandra auth user env var secret key not found")

	assert.True(t, envVarExists(envVars, "REAPER_AUTH_PASSWORD"), "Reaper auth password env var not found")
	assert.True(t, envVarSecretHasName(envVars, "REAPER_AUTH_PASSWORD", "top-secret-ui"), "Reaper auth password env var secret name not found")
	assert.True(t, envVarSecretHasKey(envVars, "REAPER_AUTH_PASSWORD", "password"), "Cassandra auth password env var secret key not found")
}

// Check if env var exists
func envVarExists(envVars []corev1.EnvVar, name string) bool {
	for _, envVar := range envVars {
		if envVar.Name == name {
			return true
		}
	}
	return false
}

// Check if env var exists
func envVarHasValue(envVars []corev1.EnvVar, name string, value string) bool {
	for _, envVar := range envVars {
		if envVar.Name == name && envVar.Value == value {
			return true
		}
	}
	return false
}

// Check if env var points to the right secret name
func envVarSecretHasName(envVars []corev1.EnvVar, name string, secretName string) bool {
	for _, envVar := range envVars {
		if envVar.Name == name {
			return envVar.ValueFrom.SecretKeyRef.LocalObjectReference.Name == secretName
		}
	}
	return false
}

// Check if env var points to the right secret key
func envVarSecretHasKey(envVars []corev1.EnvVar, name string, secretKey string) bool {
	for _, envVar := range envVars {
		if envVar.Name == name {
			return envVar.ValueFrom.SecretKeyRef.Key == secretKey
		}
	}
	return false
}

func newReaper(namespace string) *reaperapi.Reaper {
	return &reaperapi.Reaper{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      reaperName,
		},
		Spec: reaperapi.ReaperSpec{
			ReaperTemplate: reaperapi.ReaperTemplate{
				// custom image for the main container, but default image for the init container
				ContainerImage: &images.Image{
					Name:          "cassandra-reaper-custom",
					PullSecretRef: &corev1.LocalObjectReference{Name: "main-secret"},
				},
			},
			DatacenterRef: reaperapi.CassandraDatacenterRef{
				Name: cassandraDatacenterName,
			},
		},
	}
}

func verifyReaperReady(t *testing.T, ctx context.Context, k8sClient client.Client, testNamespace string) {
	t.Log("check that the reaper is ready")
	reaperKey := types.NamespacedName{Namespace: testNamespace, Name: reaperName}
	require.Eventually(t, func() bool {
		updatedReaper := &reaperapi.Reaper{}
		if err := k8sClient.Get(ctx, reaperKey, updatedReaper); err != nil {
			return false
		}
		return updatedReaper.Status.IsReady()
	}, timeout, interval)
}

func patchDeploymentStatus(t *testing.T, ctx context.Context, deployment *appsv1.Deployment, replicas, readyReplicas int32, k8sClient client.Client) {
	deploymentPatch := client.MergeFrom(deployment.DeepCopy())
	deployment.Status.Replicas = replicas
	deployment.Status.ReadyReplicas = readyReplicas
	err := k8sClient.Status().Patch(ctx, deployment, deploymentPatch)
	require.NoError(t, err)
}
