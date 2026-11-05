package v1alpha1

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestMedusaRestoreMappingPitrJSON(t *testing.T) {
	inPlace := true
	m := MedusaRestoreMapping{
		InPlace: &inPlace,
		Pitr: &PitrRestoreBlock{
			TargetTimestamp: 1700000000,
			NodeBackups:     map[string]string{"node1": "backup1"},
		},
	}
	b, err := json.Marshal(m)
	require.NoError(t, err)
	require.JSONEq(t, `{"in_place":true,"pitr":{"target_timestamp":1700000000,"node_backups":{"node1":"backup1"}}}`, string(b))

	m.Pitr = nil
	b, err = json.Marshal(m)
	require.NoError(t, err)
	require.JSONEq(t, `{"in_place":true}`, string(b))
}

// K8ssandraCluster is used as unstructured: importing apis/k8ssandra here would be an import cycle.
func newK8ssandraCluster(namespace, name string, pitr map[string]interface{}) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{
			"cassandra": map[string]interface{}{
				"serverVersion": "4.0.1",
				"datacenters":   []interface{}{map[string]interface{}{"metadata": map[string]interface{}{"name": "dc1"}, "size": int64(1)}},
			},
			"medusa": map[string]interface{}{
				"storageProperties": map[string]interface{}{"storageProvider": "google_storage", "bucketName": "b"},
				"pitr":              pitr,
			},
		},
	}}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: "k8ssandra.io", Version: "v1alpha1", Kind: "K8ssandraCluster"})
	u.SetNamespace(namespace)
	u.SetName(name)
	return u
}

// TestPitrSchema uses its own envtest without webhooks, so only the CRD schema is exercised.
func TestPitrSchema(t *testing.T) {
	require := require.New(t)
	env := &envtest.Environment{CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "config", "crd", "bases")}}
	cfg, err := env.Start()
	require.NoError(err)
	defer func() { require.NoError(env.Stop()) }()

	scheme := runtime.NewScheme()
	require.NoError(AddToScheme(scheme))
	require.NoError(corev1.AddToScheme(scheme))
	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	require.NoError(err)
	require.NoError(k8sClient.Create(context.TODO(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "pitrtest"}}))

	restore := &MedusaRestoreJob{
		ObjectMeta: metav1.ObjectMeta{Name: "pitr-restore", Namespace: "pitrtest"},
		Spec:       MedusaRestoreJobSpec{TargetTimestamp: "2026-01-01T00:00:00Z", CassandraDatacenter: "dc1"},
	}
	require.NoError(k8sClient.Create(context.TODO(), restore))

	valid := newK8ssandraCluster("pitrtest", "valid", map[string]interface{}{
		"enabled": true, "archiveIntervalSeconds": int64(30), "spoolDir": "/x",
	})
	require.NoError(k8sClient.Create(context.TODO(), valid))

	invalid := newK8ssandraCluster("pitrtest", "invalid", map[string]interface{}{
		"enabled": true, "archiveIntervalSeconds": int64(0),
	})
	err = k8sClient.Create(context.TODO(), invalid)
	require.ErrorContains(err, "archiveIntervalSeconds")
}
