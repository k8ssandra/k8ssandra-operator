/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Webhooks tests must sit in test/webhooks instead of apis/k8ssandra/v1alpha1 where you'd normally expect them, because they need to utilise test
// functions from pkg/test which imports apis, which leads to a cycle in the imports.

package webhooks

import (
	"context"
	"crypto/tls"
	"fmt"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	testpkg "github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"net"
	"path/filepath"
	"testing"
	"time"

	logrusr "github.com/bombsimon/logrusr/v2"
	"github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"

	//+kubebuilder:scaffold:imports
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
)

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc

func TestWebhook(t *testing.T) {
	require := require.New(t)
	ctx, cancel = context.WithCancel(context.TODO())

	logrusLog := logrus.New()
	log := logrusr.New(logrusLog)
	logf.SetLogger(log)

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join( "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: false,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "config", "webhook")},
		},
	}

	cfg, err := testEnv.Start()
	require.NoError(err)
	require.NotNil(cfg)

	defer cancel()
	defer testEnv.Stop()

	scheme := runtime.NewScheme()
	err = k8ssandraapi.AddToScheme(scheme)
	require.NoError(err)

	err = corev1.AddToScheme(scheme)
	require.NoError(err)

	err = admissionv1.AddToScheme(scheme)
	require.NoError(err)

	err = k8ssandraapi.AddToScheme(scheme)
	require.NoError(err)

	err = reaperapi.AddToScheme(scheme)
	require.NoError(err)

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	require.NoError(err)
	require.NotNil(k8sClient)

	// start webhook server using Manager
	webhookInstallOptions := &testEnv.WebhookInstallOptions
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme,
		Host:               webhookInstallOptions.LocalServingHost,
		Port:               webhookInstallOptions.LocalServingPort,
		CertDir:            webhookInstallOptions.LocalServingCertDir,
		LeaderElection:     false,
		MetricsBindAddress: "0",
	})
	require.NoError(err)

	clientCache := clientcache.New(k8sClient, k8sClient, scheme)
	clientCache.AddClient("envtest", k8sClient)
	err = (&k8ssandraapi.K8ssandraCluster{}).SetupWebhookWithManager(mgr, clientCache)
	require.NoError(err)

	//+kubebuilder:scaffold:webhook

	go func() {
		err = mgr.Start(ctx)
		require.NoError(err)
	}()

	// wait for the webhook server to get ready
	dialer := &net.Dialer{Timeout: time.Second}
	addrPort := fmt.Sprintf("%s:%d", webhookInstallOptions.LocalServingHost, webhookInstallOptions.LocalServingPort)
	require.Eventually(func() bool {
		conn, err := tls.DialWithDialer(dialer, "tcp", addrPort, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}, 2*time.Second, 300*time.Millisecond)

	t.Run("ContextValidation", testContextValidation)
	t.Run("ReaperKeyspaceValidation", testReaperKeyspaceValidation)
	t.Run("StorageConfigValidation", testStorageConfigValidation)
	t.Run("NumTokensValidation", testNumTokens)
	t.Run("TelemetryValidation", testTelemetryValidation)
}

func testContextValidation(t *testing.T) {
	require := require.New(t)
	createNamespace(require, "create-namespace")
	cluster := createMinimalClusterObj("create-test", "create-namespace")

	err := k8sClient.Create(ctx, cluster)
	require.NoError(err)

	// Verify incorrect K8sContext is not allowed
	cluster.Spec.Cassandra.Datacenters[0].K8sContext = "wrong"
	err = k8sClient.Update(ctx, cluster)
	require.Error(err)
}

func testReaperKeyspaceValidation(t *testing.T) {
	require := require.New(t)
	createNamespace(require, "update-namespace")
	cluster := createMinimalClusterObj("update-test", "update-namespace")

	cluster.Spec.Reaper = &reaperapi.ReaperClusterTemplate{
		ReaperTemplate: reaperapi.ReaperTemplate{
			Keyspace: "original",
		},
	}

	err := k8sClient.Create(ctx, cluster)
	require.NoError(err)

	cluster.Spec.Reaper.ReaperTemplate.Keyspace = "modified"
	err = k8sClient.Update(ctx, cluster)
	require.Error(err)
}

func testStorageConfigValidation(t *testing.T) {
	require := require.New(t)
	createNamespace(require, "storage-namespace")
	cluster := createMinimalClusterObj("storage-test", "storage-namespace")

	cluster.Spec.Cassandra.StorageConfig = nil
	err := k8sClient.Create(ctx, cluster)
	require.Error(err)

	cluster.Spec.Cassandra.StorageConfig = &v1beta1.StorageConfig{}
	err = k8sClient.Create(ctx, cluster)
	require.NoError(err)

	cluster.Spec.Cassandra.StorageConfig = nil
	cluster.Spec.Cassandra.Datacenters[0].StorageConfig = &v1beta1.StorageConfig{}
	err = k8sClient.Update(ctx, cluster)
	require.NoError(err)

	cluster.Spec.Cassandra.Datacenters = append(cluster.Spec.Cassandra.Datacenters, k8ssandraapi.CassandraDatacenterTemplate{
		K8sContext: "envtest",
		Size:       1,
	})

	err = k8sClient.Update(ctx, cluster)
	require.Error(err)

	cluster.Spec.Cassandra.Datacenters[1].StorageConfig = &v1beta1.StorageConfig{}
	err = k8sClient.Update(ctx, cluster)
	require.NoError(err)
}

func testNumTokens(t *testing.T) {
	require := require.New(t)
	createNamespace(require, "numtokens-namespace")
	cluster := createMinimalClusterObj("numtokens-test", "numtokens-namespace")

	// Create without token definition
	cluster.Spec.Cassandra.CassandraConfig = &k8ssandraapi.CassandraConfig{}
	err := k8sClient.Create(ctx, cluster)
	require.NoError(err)

	tokens := int(256)
	cluster.Spec.Cassandra.CassandraConfig.CassandraYaml.NumTokens = &tokens
	err = k8sClient.Update(ctx, cluster)
	require.Error(err)

	err = k8sClient.Delete(context.TODO(), cluster)
	require.NoError(err)

	// Recreate with tokens
	cluster.ResourceVersion = ""
	err = k8sClient.Create(ctx, cluster)
	require.NoError(err)

	newTokens := int(16)
	cluster.Spec.Cassandra.CassandraConfig.CassandraYaml.NumTokens = &newTokens
	err = k8sClient.Update(ctx, cluster)
	require.Error(err)

	cluster.Spec.Cassandra.CassandraConfig.CassandraYaml.NumTokens = nil
	err = k8sClient.Update(ctx, cluster)
	require.Error(err)
}

func testTelemetryValidation(t *testing.T) {
	promCache := testpkg.MockClientCache{}
	kc := testpkg.NewK8ssandraCluster("test-cluster", "test-namespace")
	telemetryEnabled := &telemetryapi.TelemetrySpec{
		Prometheus: &telemetryapi.PrometheusTelemetrySpec{
			Enabled: true,
			CommonLabels: map[string]string{"thisLabel": "maybe", "thatLabel": "definitely"},
		},
	}
	telemetryDisabled := &telemetryapi.TelemetrySpec{
		Prometheus: &telemetryapi.PrometheusTelemetrySpec{
			Enabled: false,
		},
	}
	// Cases where Prometheus is installed.
	err := k8ssandraapi.TelemetrySpecsAreValid(&kc, promCache)
	require.NoError(t, err, "unexpected error when validating telemetry spec on prom cluster WITH NO telemetry, no DCs")
	kc.Spec.Cassandra.Telemetry = telemetryEnabled
	err = k8ssandraapi.TelemetrySpecsAreValid(&kc, promCache)
	require.NoError(t, err, "unexpected error when validating telemetry spec on prom cluster WITH cass telemetry, no DCs")
	kc.Spec.Stargate = &v1alpha1.StargateClusterTemplate{
		Size: 1,
		StargateTemplate: v1alpha1.StargateTemplate{
			Telemetry: telemetryEnabled,
		},
	}
	err = k8ssandraapi.TelemetrySpecsAreValid(&kc, promCache)
	require.NoError(t, err, "unexpected error when validating telemetry spec on prom cluster WITH cass, stargate telemetry, no DCs")
	kc.Spec.Cassandra.Datacenters = []k8ssandraapi.CassandraDatacenterTemplate{
		{
			Telemetry: telemetryDisabled,
			Stargate: &v1alpha1.StargateDatacenterTemplate{
				StargateClusterTemplate: v1alpha1.StargateClusterTemplate{
					StargateTemplate: v1alpha1.StargateTemplate{
						Telemetry: telemetryEnabled,
					},
				},
			},
		},
		{
			K8sContext: "context2",
			Telemetry: telemetryEnabled,
			Stargate: &v1alpha1.StargateDatacenterTemplate{
				StargateClusterTemplate: v1alpha1.StargateClusterTemplate{
					StargateTemplate: v1alpha1.StargateTemplate{
						Telemetry: telemetryDisabled,
					},
				},
			},
		},
	}
	err = k8ssandraapi.TelemetrySpecsAreValid(&kc, promCache)
	require.NoError(t, err, "unexpected error when validating telemetry spec on prom cluster with MIXED cass, stargate telemetry, 2 DCs")

	// Cases where Prometheus is NOT installed
	noPromCache := testpkg.MockClientCache{
		PromInstalled: map[string]bool{"":false, "context2": false},
	}
	kc = testpkg.NewK8ssandraCluster("test-cluster", "test-namespace")
	err = k8ssandraapi.TelemetrySpecsAreValid(&kc, noPromCache)
	require.NoError(t, err, "unexpected error when validating telemetry spec on NON-PROM cluster with telemetry DISABLED")
	kc.Spec.Cassandra.Telemetry = telemetryEnabled
	err = k8ssandraapi.TelemetrySpecsAreValid(&kc, noPromCache)
	require.Error(t, err,"did not get expected error when trying to enable cass telemetry on a cluster with no prom installed")
	kc.Spec.Cassandra.Telemetry = nil
	kc.Spec.Stargate.Telemetry = telemetryEnabled
	err = k8ssandraapi.TelemetrySpecsAreValid(&kc, noPromCache)
	require.Error(t, err,"did not get expected error when trying to enable stargate telemetry on a cluster with no prom installed")
}

func createNamespace(require *require.Assertions, namespace string) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	err := k8sClient.Create(ctx, ns)
	require.NoError(err)
}

func createMinimalClusterObj(name, namespace string) *k8ssandraapi.K8ssandraCluster {
	return &k8ssandraapi.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: k8ssandraapi.K8ssandraClusterSpec{
			Cassandra: &k8ssandraapi.CassandraClusterTemplate{
				StorageConfig: &v1beta1.StorageConfig{},
				Datacenters: []k8ssandraapi.CassandraDatacenterTemplate{
					{
						K8sContext: "envtest",
						Size:       1,
					},
				},
			},
		},
	}
}
