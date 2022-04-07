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

package v1alpha1

import (
	"context"
	"crypto/tls"
	"fmt"
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
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: false,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "..", "config", "webhook")},
		},
	}

	cfg, err := testEnv.Start()
	require.NoError(err)
	require.NotNil(cfg)

	defer cancel()
	defer testEnv.Stop()

	scheme := runtime.NewScheme()
	err = AddToScheme(scheme)
	require.NoError(err)

	err = corev1.AddToScheme(scheme)
	require.NoError(err)

	err = admissionv1.AddToScheme(scheme)
	require.NoError(err)

	err = AddToScheme(scheme)
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
	err = (&K8ssandraCluster{}).SetupWebhookWithManager(mgr, clientCache)
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

	cluster.Spec.Cassandra.DatacenterOptions.StorageConfig = nil
	err := k8sClient.Create(ctx, cluster)
	require.Error(err)

	cluster.Spec.Cassandra.DatacenterOptions.StorageConfig = &v1beta1.StorageConfig{}
	err = k8sClient.Create(ctx, cluster)
	require.NoError(err)

	cluster.Spec.Cassandra.DatacenterOptions.StorageConfig = nil
	cluster.Spec.Cassandra.Datacenters[0].DatacenterOptions.StorageConfig = &v1beta1.StorageConfig{}
	err = k8sClient.Update(ctx, cluster)
	require.NoError(err)

	cluster.Spec.Cassandra.Datacenters = append(cluster.Spec.Cassandra.Datacenters, CassandraDatacenterTemplate{
		K8sContext: "envtest",
		Size:       1,
	})

	err = k8sClient.Update(ctx, cluster)
	require.Error(err)

	cluster.Spec.Cassandra.Datacenters[1].DatacenterOptions.StorageConfig = &v1beta1.StorageConfig{}
	err = k8sClient.Update(ctx, cluster)
	require.NoError(err)
}

func testNumTokens(t *testing.T) {
	require := require.New(t)
	createNamespace(require, "numtokens-namespace")
	cluster := createMinimalClusterObj("numtokens-test", "numtokens-namespace")

	// Create without token definition
	cluster.Spec.Cassandra.DatacenterOptions.CassandraConfig = &CassandraConfig{}
	err := k8sClient.Create(ctx, cluster)
	require.NoError(err)

	tokens := int(256)
	cluster.Spec.Cassandra.DatacenterOptions.CassandraConfig.CassandraYaml.NumTokens = &tokens
	err = k8sClient.Update(ctx, cluster)
	require.Error(err)

	err = k8sClient.Delete(context.TODO(), cluster)
	require.NoError(err)

	// Recreate with tokens
	cluster.ResourceVersion = ""
	err = k8sClient.Create(ctx, cluster)
	require.NoError(err)

	newTokens := int(16)
	cluster.Spec.Cassandra.DatacenterOptions.CassandraConfig.CassandraYaml.NumTokens = &newTokens
	err = k8sClient.Update(ctx, cluster)
	require.Error(err)

	cluster.Spec.Cassandra.DatacenterOptions.CassandraConfig.CassandraYaml.NumTokens = nil
	err = k8sClient.Update(ctx, cluster)
	require.Error(err)
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

func createMinimalClusterObj(name, namespace string) *K8ssandraCluster {
	return &K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: K8ssandraClusterSpec{
			Cassandra: &CassandraClusterTemplate{
				DatacenterOptions: DatacenterOptions{
					StorageConfig: &v1beta1.StorageConfig{},
				},
				Datacenters: []CassandraDatacenterTemplate{
					{
						K8sContext: "envtest",
						Size:       1,
					},
				},
			},
		},
	}
}
