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
	"github.com/k8ssandra/k8ssandra-operator/pkg/shared"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	//+kubebuilder:scaffold:imports

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc

func TestMedusaWebhooks(t *testing.T) {
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
	defer func(testEnv *envtest.Environment) {
		err := testEnv.Stop()
		if err != nil {
			log.Error(err, "failure to stop test environment")
		}
	}(testEnv)

	scheme := runtime.NewScheme()
	err = AddToScheme(scheme)
	require.NoError(err)

	err = corev1.AddToScheme(scheme)
	require.NoError(err)

	err = admissionv1beta1.AddToScheme(scheme)
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

	err = (&MedusaBackupSchedule{}).SetupWebhookWithManager(mgr)
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
		closeErr := conn.Close()
		if closeErr != nil {
			log.Error(closeErr, "failed to close connection")
		}
		return true
	}, 2*time.Second, 300*time.Millisecond)

	t.Run("CronScheduleValidation", testCronScheduleValidation)
}

func testCronScheduleValidation(t *testing.T) {
	require := require.New(t)

	createNamespace(require, "scheduletest")

	backupSchedule := &MedusaBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-schedule",
			Namespace: "scheduletest",
		},
		Spec: MedusaBackupScheduleSpec{
			CronSchedule: "? 8 0 0", // Invalid
			BackupSpec: MedusaBackupJobSpec{
				CassandraDatacenter: "dc1",
				Type:                shared.FullBackup,
			},
		},
	}

	err := k8sClient.Create(context.TODO(), backupSchedule)
	require.Error(err)

	backupSchedule.Spec.CronSchedule = "* 0 * * 0"

	err = k8sClient.Create(context.TODO(), backupSchedule)
	require.NoError(err)

	backupSchedule.Spec.CronSchedule = "invalid"

	err = k8sClient.Update(context.TODO(), backupSchedule)
	require.Error(err)

	backupSchedule.Spec.CronSchedule = "5 0 * 8 *"

	err = k8sClient.Update(context.TODO(), backupSchedule)
	require.NoError(err)
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
