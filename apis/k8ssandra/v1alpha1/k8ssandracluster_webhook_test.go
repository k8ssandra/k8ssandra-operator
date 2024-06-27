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
	"github.com/k8ssandra/k8ssandra-operator/pkg/unstructured"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	//+kubebuilder:scaffold:imports
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	medusaapi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
)

var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc

func TestWebhook(t *testing.T) {
	required := require.New(t)
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
	required.NoError(err)
	required.NotNil(cfg)

	defer cancel()
	defer func(testEnv *envtest.Environment) {
		err := testEnv.Stop()
		if err != nil {
			log.Error(err, "failure to stop test environment")
		}
	}(testEnv)

	scheme := runtime.NewScheme()
	err = AddToScheme(scheme)
	required.NoError(err)

	err = corev1.AddToScheme(scheme)
	required.NoError(err)

	err = admissionv1.AddToScheme(scheme)
	required.NoError(err)

	err = AddToScheme(scheme)
	required.NoError(err)

	err = reaperapi.AddToScheme(scheme)
	required.NoError(err)

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	required.NoError(err)
	required.NotNil(k8sClient)

	// start webhook server using Manager
	webhookInstallOptions := &testEnv.WebhookInstallOptions

	whServer := webhook.NewServer(webhook.Options{
		Port:    webhookInstallOptions.LocalServingPort,
		Host:    webhookInstallOptions.LocalServingHost,
		CertDir: webhookInstallOptions.LocalServingCertDir,
		TLSOpts: []func(*tls.Config){func(config *tls.Config) {}},
	})

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:         scheme,
		WebhookServer:  whServer,
		LeaderElection: false,
		Metrics: server.Options{
			BindAddress: "0",
		},
	})
	required.NoError(err)

	clientCache := clientcache.New(k8sClient, k8sClient, scheme)
	clientCache.AddClient("envtest", k8sClient)
	err = (&K8ssandraCluster{}).SetupWebhookWithManager(mgr, clientCache)
	required.NoError(err)

	//+kubebuilder:scaffold:webhook

	go func() {
		err = mgr.Start(ctx)
		required.NoError(err)
	}()

	// wait for the webhook server to get ready
	dialer := &net.Dialer{Timeout: time.Second}
	addrPort := fmt.Sprintf("%s:%d", webhookInstallOptions.LocalServingHost, webhookInstallOptions.LocalServingPort)
	required.Eventually(func() bool {
		conn, err := tls.DialWithDialer(dialer, "tcp", addrPort, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			return false
		}
		closeErr := conn.Close()
		if closeErr != nil {
			log.Error(closeErr, "failed to close connection")
		}
		return true
	}, 2*time.Second, 300*time.Millisecond)

	t.Run("ContextValidation", testContextValidation)
	t.Run("ReaperKeyspaceValidation", testReaperKeyspaceValidation)
	t.Run("StorageConfigValidation", testStorageConfigValidation)
	t.Run("NumTokensValidation", testNumTokens)
	t.Run("NumTokensValidationInUpdate", testNumTokensInUpdate)
	t.Run("StsNameTooLong", testStsNameTooLong)
	t.Run("MedusaPrefixMissing", testMedusaPrefixMissing)
	t.Run("InvalidDcName", testInvalidDcName)
	t.Run("MedusaConfigNonLocalNamespace", testMedusaNonLocalNamespace)
}

func testContextValidation(t *testing.T) {
	required := require.New(t)
	createNamespace(required, "create-namespace")
	cluster := createMinimalClusterObj("create-test", "create-namespace")

	err := k8sClient.Create(ctx, cluster)
	required.NoError(err)

	// Verify incorrect K8sContext is not allowed
	cluster.Spec.Cassandra.Datacenters[0].K8sContext = "wrong"
	err = k8sClient.Update(ctx, cluster)
	required.Error(err)
}

func testNumTokensInUpdate(t *testing.T) {
	require := require.New(t)
	createNamespace(require, "numtokensupdate-namespace")
	cluster := createMinimalClusterObj("numtokens-test-update", "numtokensupdate-namespace")
	cluster.Spec.Cassandra.ServerVersion = "3.11.10"
	cluster.Spec.Cassandra.DatacenterOptions.CassandraConfig = &CassandraConfig{}
	err := k8sClient.Create(ctx, cluster)
	require.NoError(err)

	// Now update to 4.1.3
	cluster.Spec.Cassandra.DatacenterOptions.CassandraConfig.CassandraYaml = unstructured.Unstructured{"num_tokens": 256}
	cluster.Spec.Cassandra.ServerVersion = "4.1.3"

	// This should be acceptable change, since 3.11.10 defaulted to 256 and so it is the same value
	err = k8sClient.Update(ctx, cluster)
	require.NoError(err)
}

func testReaperKeyspaceValidation(t *testing.T) {
	required := require.New(t)
	createNamespace(required, "update-namespace")
	cluster := createMinimalClusterObj("update-test", "update-namespace")

	cluster.Spec.Reaper = &reaperapi.ReaperClusterTemplate{
		ReaperTemplate: reaperapi.ReaperTemplate{
			Keyspace: "original",
		},
	}

	err := k8sClient.Create(ctx, cluster)
	required.NoError(err)

	cluster.Spec.Reaper.ReaperTemplate.Keyspace = "modified"
	err = k8sClient.Update(ctx, cluster)
	required.Error(err)
}

func testStorageConfigValidation(t *testing.T) {
	required := require.New(t)
	createNamespace(required, "storage-namespace")
	cluster := createMinimalClusterObj("storage-test", "storage-namespace")
	cluster.Spec.Cassandra.DatacenterOptions.ServerVersion = "3.11.10"

	cluster.Spec.Cassandra.DatacenterOptions.StorageConfig = nil
	err := k8sClient.Create(ctx, cluster)
	required.Error(err)

	cluster.Spec.Cassandra.DatacenterOptions.StorageConfig = &v1beta1.StorageConfig{}
	err = k8sClient.Create(ctx, cluster)
	required.NoError(err)

	cluster.Spec.Cassandra.DatacenterOptions.StorageConfig = nil
	cluster.Spec.Cassandra.Datacenters[0].DatacenterOptions.StorageConfig = &v1beta1.StorageConfig{}
	err = k8sClient.Update(ctx, cluster)
	required.NoError(err)

	cluster.Spec.Cassandra.Datacenters = append(cluster.Spec.Cassandra.Datacenters, CassandraDatacenterTemplate{
		Meta:       EmbeddedObjectMeta{Name: "dc2"},
		K8sContext: "envtest",
		Size:       1,
	})

	err = k8sClient.Update(ctx, cluster)
	required.Error(err)

	cluster.Spec.Cassandra.Datacenters[1].DatacenterOptions.StorageConfig = &v1beta1.StorageConfig{}
	err = k8sClient.Update(ctx, cluster)
	required.NoError(err)
}

func testNumTokens(t *testing.T) {
	required := require.New(t)

	createNamespace(required, "numtokens-namespace")
	cluster := createMinimalClusterObj("numtokens-test", "numtokens-namespace")

	// Create without token definition
	cluster.Spec.Cassandra.DatacenterOptions.CassandraConfig = &CassandraConfig{}
	err := k8sClient.Create(ctx, cluster)
	required.NoError(err)

	tokens := 256
	cluster.Spec.Cassandra.DatacenterOptions.CassandraConfig.CassandraYaml = unstructured.Unstructured{"num_tokens": tokens}
	err = k8sClient.Update(ctx, cluster)
	required.Error(err)

	err = k8sClient.Delete(context.TODO(), cluster)
	required.NoError(err)

	// Recreate with tokens
	cluster.ResourceVersion = ""
	err = k8sClient.Create(ctx, cluster)
	required.NoError(err)

	newTokens := 16
	cluster.Spec.Cassandra.DatacenterOptions.CassandraConfig.CassandraYaml["num_tokens"] = newTokens
	err = k8sClient.Update(ctx, cluster)
	required.Error(err)

	delete(cluster.Spec.Cassandra.DatacenterOptions.CassandraConfig.CassandraYaml, "num_tokens")
	err = k8sClient.Update(ctx, cluster)
	required.Error(err)

	// Num_token update validations
	tokens = 11
	newTokens = 22

	oldCluster := createClusterObjWithCassandraConfig("numtokens-test-1", "numtokens-namespace-1")
	newCluster := createClusterObjWithCassandraConfig("numtokens-test-2", "numtokens-namespace-2")

	delete(oldCluster.Spec.Cassandra.DatacenterOptions.CassandraConfig.CassandraYaml, "num_tokens")
	newCluster.Spec.Cassandra.DatacenterOptions.CassandraConfig.CassandraYaml["num_tokens"] = newTokens

	var oldCassConfig = oldCluster.Spec.Cassandra.DatacenterOptions.CassandraConfig
	var newCassConfig = newCluster.Spec.Cassandra.DatacenterOptions.CassandraConfig

	// Handle new num_token value different from previously specified as nil
	required.NotEqual(oldCassConfig.CassandraYaml["num_tokens"], newCassConfig.CassandraYaml["num_tokens"])
	var _, errorWhenNew = (*newCluster).ValidateUpdate(oldCluster)
	required.Error(errorWhenNew, "expected error having new num_token value different from previous specified as nil")

	oldCluster.Spec.Cassandra.DatacenterOptions.CassandraConfig.CassandraYaml["num_tokens"] = tokens
	newCluster.Spec.Cassandra.DatacenterOptions.CassandraConfig.CassandraYaml["num_tokens"] = newTokens

	oldCassConfig = oldCluster.Spec.Cassandra.DatacenterOptions.CassandraConfig
	newCassConfig = newCluster.Spec.Cassandra.DatacenterOptions.CassandraConfig

	// Handle new num_token value different from previously specified as an actual value
	required.NotEqual(oldCassConfig.CassandraYaml["num_tokens"], newCassConfig.CassandraYaml["num_tokens"])
	_, errorWhenNew = (*newCluster).ValidateUpdate(oldCluster)
	required.Error(errorWhenNew, "expected error having new num_token value different from previous specified")

	// Handle new num_token not specified when previously specified
	oldCassConfig.CassandraYaml["num_tokens"] = tokens
	delete(newCassConfig.CassandraYaml, "num_tokens")

	var _, errorWhenNil = (*newCluster).ValidateUpdate(oldCluster)
	required.Error(errorWhenNil, "expected error having new num_token value as nil from previous specified")

	oldCassConfig.CassandraYaml["num_tokens"] = tokens
	newCassConfig.CassandraYaml = unstructured.Unstructured{}

	_, errorWhenNil = (*newCluster).ValidateUpdate(oldCluster)
	required.Error(errorWhenNil, "expected error having new num_token value as nil from previous specified")

	oldCassConfig.CassandraYaml["num_tokens"] = tokens
	newCassConfig = &CassandraConfig{}
	_, errorWhenNil = (*newCluster).ValidateUpdate(oldCluster)
	required.Error(errorWhenNil, "expected error having new num_token value as nil from previous specified")

	oldCassConfig.CassandraYaml["num_tokens"] = tokens
	newCassConfig = &CassandraConfig{}
	_, errorWhenNil = (*newCluster).ValidateUpdate(oldCluster)
	required.Error(errorWhenNil, "expected error having new num_token value as nil from previous specified")

	// Expected to be able to update without token change, however changes to other config values are made
	sameNumTokens := 8675309
	diffNumTokens := 42
	intervalInMins := 9035768
	enabled := true

	oldCluster.Spec.Cassandra.DatacenterOptions.CassandraConfig.CassandraYaml["num_tokens"] = sameNumTokens
	newCluster.Spec.Cassandra.DatacenterOptions.CassandraConfig.CassandraYaml["num_tokens"] = sameNumTokens
	newCluster.Spec.Cassandra.DatacenterOptions.CassandraConfig.CassandraYaml["cdc_enabled"] = enabled
	newCluster.Spec.Cassandra.DatacenterOptions.CassandraConfig.CassandraYaml["index_summary_resize_interval_in_minutes"] = intervalInMins

	_, errorOnValidate := (*newCluster).ValidateUpdate(oldCluster)
	required.NoError(errorOnValidate)

	// Expected failure for validation with token change while changes to other config values are being made
	oldCluster.Spec.Cassandra.DatacenterOptions.CassandraConfig.CassandraYaml["num_tokens"] = sameNumTokens
	newCluster.Spec.Cassandra.DatacenterOptions.CassandraConfig.CassandraYaml["num_tokens"] = diffNumTokens
	newCluster.Spec.Cassandra.DatacenterOptions.CassandraConfig.CassandraYaml["cdc_enabled"] = enabled
	newCluster.Spec.Cassandra.DatacenterOptions.CassandraConfig.CassandraYaml["index_summary_resize_interval_in_minutes"] = intervalInMins

	_, errorOnValidate = (*newCluster).ValidateUpdate(oldCluster)
	required.Error(errorOnValidate, "expected error when changing the value of num tokens while also changing other field values")
}

func testStsNameTooLong(t *testing.T) {
	required := require.New(t)
	createNamespace(required, "too-long-namespace")
	cluster := createMinimalClusterObj("create-very-long-cluster-name-which-will-overflow-our-limit", "too-long-namespace")

	err := k8sClient.Create(ctx, cluster)
	required.Error(err)
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

func createClusterObjWithCassandraConfig(name, namespace string) *K8ssandraCluster {
	return &K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: K8ssandraClusterSpec{
			Cassandra: &CassandraClusterTemplate{
				DatacenterOptions: DatacenterOptions{
					CassandraConfig: &CassandraConfig{
						CassandraYaml: unstructured.Unstructured{"num_tokens": nil},
					},
					StorageConfig: &v1beta1.StorageConfig{},
				},

				Datacenters: []CassandraDatacenterTemplate{
					{
						Meta:       EmbeddedObjectMeta{Name: "dc1"},
						K8sContext: "envtest",
						Size:       1,
					},
				},
			},
		},
	}
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
						Meta: EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: "envtest",
						Size:       1,
					},
				},
			},
		},
	}
}

func testMedusaPrefixMissing(t *testing.T) {
	required := require.New(t)
	createNamespace(required, "short-namespace")

	clusterWithoutMedusa := createMinimalClusterObj("without-medusa", "short-namespace")
	err := k8sClient.Create(ctx, clusterWithoutMedusa)
	required.NoError(err)

	clusterWithMedusa := createMinimalClusterObj("with-medusa", "short-namespace")
	clusterWithMedusa.Spec.Medusa = &medusaapi.MedusaClusterTemplate{
		StorageProperties: medusaapi.Storage{
			Prefix: "",
		},
	}
	err = k8sClient.Create(ctx, clusterWithMedusa)
	required.NoError(err)

	clusterWithoutPrefix := createMinimalClusterObj("without-prefix", "short-namespace")
	clusterWithoutPrefix.Spec.Medusa = &medusaapi.MedusaClusterTemplate{
		MedusaConfigurationRef: corev1.ObjectReference{
			Name: "medusa-config",
		},
		StorageProperties: medusaapi.Storage{
			Prefix: "",
		},
	}
	err = k8sClient.Create(ctx, clusterWithoutPrefix)
	required.Error(err)

	clusterWithPrefix := createMinimalClusterObj("with-prefix", "short-namespace")
	clusterWithPrefix.Spec.Medusa = &medusaapi.MedusaClusterTemplate{
		MedusaConfigurationRef: corev1.ObjectReference{
			Name: "medusa-config",
		},
		StorageProperties: medusaapi.Storage{
			Prefix: "some-prefix",
		},
	}
	err = k8sClient.Create(ctx, clusterWithPrefix)
	required.NoError(err)
}

func testInvalidDcName(t *testing.T) {
	required := require.New(t)
	createNamespace(required, "ns")

	clusterWithBadDcName := createMinimalClusterObj("bad-dc-name", "ns")
	clusterWithBadDcName.Spec.Cassandra.Datacenters[0].Meta.Name = "DC1"
	err := k8sClient.Create(ctx, clusterWithBadDcName)
	required.Error(err)
	required.Contains(err.Error(), "invalid DC name")
}

func testMedusaNonLocalNamespace(t *testing.T) {
	required := require.New(t)
	badCluster := createMinimalClusterObj("medusaconfig-nonlocal", "ns")
	badCluster.Spec.Medusa = &medusaapi.MedusaClusterTemplate{
		MedusaConfigurationRef: corev1.ObjectReference{
			Namespace: "nonlocal-ns",
			Name:      "medusa-config",
		},
		StorageProperties: medusaapi.Storage{
			Prefix: "some-prefix",
		},
	}
	err := badCluster.validateK8ssandraCluster()
	required.Error(err)
	required.Contains(err.Error(), "Medusa config must be namespace local")
}

// TestValidateUpdateNumTokens is a unit test for numTokens updates.
func TestValidateUpdateNumTokens(t *testing.T) {
	type config struct {
		globalVersion   string
		dcVersion       string
		globalNumTokens int
		dcNumTokens     int
	}
	type testCase struct {
		oldConfig config
		newConfig config
		expected  error
	}
	testCases := []testCase{
		{
			oldConfig: config{globalVersion: "3.11.10"},
			newConfig: config{globalVersion: "4.1.3"},
			expected:  ErrNumTokens,
		},
		{
			oldConfig: config{globalVersion: "3.11.10"},
			newConfig: config{globalNumTokens: 256},
			expected:  nil,
		},
		{
			oldConfig: config{globalVersion: "3.11.10"},
			newConfig: config{dcNumTokens: 256},
			expected:  nil,
		},
		{
			oldConfig: config{dcVersion: "3.11.10"},
			newConfig: config{globalNumTokens: 256},
			expected:  nil,
		},
		{
			oldConfig: config{dcVersion: "3.11.10"},
			newConfig: config{dcNumTokens: 256},
			expected:  nil,
		},
		{
			oldConfig: config{dcNumTokens: 10},
			newConfig: config{dcNumTokens: 10},
			expected:  nil,
		},
		{
			oldConfig: config{globalNumTokens: 10},
			newConfig: config{dcNumTokens: 10},
			expected:  nil,
		},
	}
	toCassandra := func(c config) *CassandraClusterTemplate {
		cassandra := &CassandraClusterTemplate{ServerType: ServerDistributionCassandra}
		options := DatacenterOptions{ServerVersion: c.globalVersion}
		if c.globalNumTokens != 0 {
			options.CassandraConfig = &CassandraConfig{
				CassandraYaml: unstructured.Unstructured{"num_tokens": float64(c.globalNumTokens)}}
		}
		cassandra.DatacenterOptions = options
		dc := CassandraDatacenterTemplate{
			Meta: EmbeddedObjectMeta{Name: "dc1"},
		}
		dcOptions := DatacenterOptions{ServerVersion: c.dcVersion}
		if c.dcNumTokens != 0 {
			dcOptions.CassandraConfig = &CassandraConfig{
				CassandraYaml: unstructured.Unstructured{"num_tokens": float64(c.dcNumTokens)}}
		}
		dc.DatacenterOptions = dcOptions
		cassandra.Datacenters = []CassandraDatacenterTemplate{dc}
		return cassandra
	}
	for _, testCase := range testCases {
		oldCassandra := toCassandra(testCase.oldConfig)
		newCassandra := toCassandra(testCase.newConfig)
		err := validateUpdateNumTokens(oldCassandra, newCassandra)
		if testCase.expected == nil {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
			require.Equal(t, testCase.expected.Error(), err.Error())
		}
	}
}
