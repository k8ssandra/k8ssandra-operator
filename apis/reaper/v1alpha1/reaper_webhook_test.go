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
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/bombsimon/logrusr/v2"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc

func TestReaperWebhook(t *testing.T) {
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

	err = admissionv1beta1.AddToScheme(scheme)
	required.NoError(err)

	//err = AddToScheme(scheme)
	//required.NoError(err)

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

	err = (&Reaper{}).SetupWebhookWithManager(mgr)
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

	t.Run("testReaper", testReaper)
	t.Run("testLocalStorage", testLocalStorage)
	t.Run("testCassandraStorage", testCassandraStorage)
}

func testReaper(t *testing.T) {
	require := require.New(t)
	createNamespace(require, "reaper-namespace")
	reaper := createMinimalReaperObj("test-reaper", "reaper-namespace")

	reaperWithBothRefs := reaper.DeepCopy()
	reaperWithBothRefs.Spec.DatacenterRef = createDatacenterRef()
	reaperWithBothRefs.Spec.ReaperRef = createReaperRef()
	err := k8sClient.Create(ctx, reaperWithBothRefs)
	require.Error(err)

	reaperWithTelemetry := reaper.DeepCopy()
	reaperWithTelemetry.Spec.StorageType = StorageTypeLocal
	reaperWithTelemetry.Spec.StorageConfig = createTestStorageConfig()
	reaperWithTelemetry.Spec.Telemetry = &telemetryapi.TelemetrySpec{
		Vector: &telemetryapi.VectorSpec{
			Enabled: ptr.To(true),
		},
	}
	err = k8sClient.Create(ctx, reaper)
	require.Error(err)

	reaperWithoutTelemetry := reaperWithTelemetry.DeepCopy()
	reaperWithoutTelemetry.Spec.Telemetry.Vector.Enabled = ptr.To(false)
	err = k8sClient.Create(ctx, reaperWithoutTelemetry)
	require.NoError(err)
}

func testLocalStorage(t *testing.T) {
	required := require.New(t)
	createNamespace(required, "local-namespace")
	reaper := createMinimalReaperObj("test-reaper", "local-namespace")
	reaper.Spec.StorageType = StorageTypeLocal
	reaper.Spec.StorageConfig = nil // intentionally explicit

	err := k8sClient.Create(ctx, reaper)
	required.Error(err)

	reaper.Spec.StorageConfig = createTestStorageConfig()
	err = k8sClient.Create(ctx, reaper)
	required.NoError(err)

	badAccessModeReaper := reaper.DeepCopy()
	badAccessModeReaper.Spec.StorageConfig.AccessModes = nil
	err = k8sClient.Create(ctx, badAccessModeReaper)
	required.Error(err)

	badStorageSize := reaper.DeepCopy()
	badStorageSize.Spec.StorageConfig.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("0Gi")
	err = k8sClient.Create(ctx, badStorageSize)
	required.Error(err)

	// datacenter ref is used to create reaper's storage, but also register a DC with reaper
	reaper.Spec.DatacenterRef = createDatacenterRef()
	err = k8sClient.Update(ctx, reaper)
	required.NoError(err)
}

func testCassandraStorage(t *testing.T) {
	required := require.New(t)
	createNamespace(required, "cassandra-namespace")
	reaper := createMinimalReaperObj("test-reaper", "cassandra-namespace")
	reaper.Spec.StorageType = StorageTypeCassandra

	err := k8sClient.Create(ctx, reaper)
	required.Error(err)

	reaperWithReaperRef := reaper.DeepCopy()
	reaperWithReaperRef.Spec.ReaperRef = createReaperRef()
	err = k8sClient.Create(ctx, reaperWithReaperRef)
	required.NoError(err)

	reaper.ObjectMeta.Name = "another-test-reaper"
	reaper.Spec.DatacenterRef = createDatacenterRef()
	err = k8sClient.Create(ctx, reaper)
	required.NoError(err)
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

func createMinimalReaperObj(name, namespace string) *Reaper {
	return &Reaper{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func createTestStorageConfig() *corev1.PersistentVolumeClaimSpec {
	return &corev1.PersistentVolumeClaimSpec{
		StorageClassName: ptr.To[string]("test"),
		AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("1Gi"),
			},
		},
	}
}

func createDatacenterRef() CassandraDatacenterRef {
	return CassandraDatacenterRef{
		Name:      "test-dc",
		Namespace: "test-namespace",
	}
}

func createReaperRef() corev1.ObjectReference {
	return corev1.ObjectReference{
		Name:      "test-reaper",
		Namespace: "test-namespace",
	}
}
