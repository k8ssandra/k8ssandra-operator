package reconciliation

import (
	"context"
	"testing"
	"time"

	testutils "github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	ctx           = context.Background()
	requeueDelay  = time.Second * 3
	desiredObject = corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"key": "value",
		},
	}
	currentObj = corev1.ConfigMap{}
)

func Test_Reconcile_ObjectCreateSuccess(t *testing.T) {
	fakeClient := testutils.NewFakeClientWRestMapper() // Reset the Client
	res := ReconcileObject[corev1.ConfigMap](desiredObject, ctx, fakeClient, requeueDelay)
	test := corev1.ConfigMap{}
	test.DeepCopyObject()

	assert.True(t, res.IsDone())
	// actualCm := &corev1.ConfigMap{}
	// err := fakeClient.Get(ctx, types.NamespacedName{Name: desiredObject.Name, Namespace: desiredObject.Namespace}, actualCm)
	// assert.NoError(t, err)
}

// func Test_Reconcile_ObjectCreateFailed(t *testing.T) {
// 	dc := testutils.NewCassandraDatacenter("test-dc", "test-namespace")
// 	Cfg.RemoteClient = testutils.NewCreateFailingFakeClient() // Reset the Client
// 	recRes := Cfg.ReconcileTelemetryAgentConfig(&dc)
// 	assert.True(t, recRes.IsError())
// }

// func Test_Reconcile_ObjectUpdateSuccess(t *testing.T) {
// 	dc := testutils.NewCassandraDatacenter("test-dc", "test-namespace")
// 	Cfg.RemoteClient = testutils.NewFakeClientWRestMapper() // Reset the Client
// 	// Create an initial ConfigMap with the same name.
// 	initialCm, err := Cfg.GetTelemetryAgentConfigMap()
// 	if err != nil {
// 		assert.Fail(t, "couldn't create ConfigMap")
// 	}
// 	initialCm.Annotations = make(map[string]string)
// 	initialCm.Annotations[k8ssandraapi.ResourceHashAnnotation] = "gobbledegook"
// 	initialCm.Data = map[string]string{"gobbledegook": "gobbledegook"}
// 	if err := Cfg.RemoteClient.Create(Cfg.Ctx, initialCm); err != nil {
// 		assert.Fail(t, "could not create initial ConfigMap")
// 	}
// 	// Launch reconciliation.
// 	Cfg.TelemetrySpec = getExampleTelemetrySpec()
// 	recRes := Cfg.ReconcileTelemetryAgentConfig(&dc)
// 	assert.True(t, recRes.IsRequeue())
// 	// After the update we should see the expected ConfigMap
// 	afterUpdateCM := &corev1.ConfigMap{}
// 	err = Cfg.RemoteClient.Get(Cfg.Ctx,
// 		types.NamespacedName{
// 			Name:      Cfg.Kluster.Name + "-" + Cfg.DcName + "-metrics-agent-config",
// 			Namespace: Cfg.DcNamespace},
// 		afterUpdateCM)
// 	assert.NoError(t, err)

// 	expectedCm := getExpectedConfigMap()
// 	assert.NoError(t, err)
// 	assert.Equal(t, expectedCm.Data["metric-collector.yaml"], afterUpdateCM.Data["metric-collector.yaml"])
// 	assert.Equal(t, expectedCm.Name, afterUpdateCM.Name)
// 	assert.Equal(t, expectedCm.Namespace, afterUpdateCM.Namespace)
// }

// func Test_Reconcile_ObjectUpdateDone(t *testing.T) {
// 	dc := testutils.NewCassandraDatacenter("test-dc", "test-namespace")
// 	Cfg.RemoteClient = testutils.NewFakeClientWRestMapper() // Reset the Client
// 	// Launch reconciliation.
// 	recRes := Cfg.ReconcileTelemetryAgentConfig(&dc)
// 	assert.True(t, recRes.IsRequeue())
// 	// After the update we should see the expected ConfigMap
// 	afterUpdateCM := &corev1.ConfigMap{}
// 	err := Cfg.RemoteClient.Get(Cfg.Ctx,
// 		types.NamespacedName{
// 			Name:      Cfg.Kluster.Name + "-" + Cfg.DcName + "-metrics-agent-config",
// 			Namespace: Cfg.DcNamespace,
// 		},
// 		afterUpdateCM)
// 	assert.NoError(t, err)
// 	// If we reconcile again, we should move into the Done state.
// 	recRes = Cfg.ReconcileTelemetryAgentConfig(&dc)
// 	assert.True(t, recRes.IsDone())
// }
