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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	controlcontrollers "github.com/k8ssandra/k8ssandra-operator/controllers/control"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	cassctl "github.com/k8ssandra/cass-operator/apis/control/v1alpha1"
	"go.uber.org/zap/zapcore"

	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reaper"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	configapi "github.com/k8ssandra/k8ssandra-operator/apis/config/v1beta1"
	controlv1alpha1 "github.com/k8ssandra/k8ssandra-operator/apis/control/v1alpha1"
	k8ssandraiov1alpha1 "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	medusav1alpha1 "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	replicationapi "github.com/k8ssandra/k8ssandra-operator/apis/replication/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	configctrl "github.com/k8ssandra/k8ssandra-operator/controllers/config"
	k8ssandractrl "github.com/k8ssandra/k8ssandra-operator/controllers/k8ssandra"
	medusactrl "github.com/k8ssandra/k8ssandra-operator/controllers/medusa"
	reaperctrl "github.com/k8ssandra/k8ssandra-operator/controllers/reaper"
	replicationctrl "github.com/k8ssandra/k8ssandra-operator/controllers/replication"
	secretswebhook "github.com/k8ssandra/k8ssandra-operator/controllers/secrets-webhook"
	stargatectrl "github.com/k8ssandra/k8ssandra-operator/controllers/stargate"
	// +kubebuilder:scaffold:imports
)

var (
	version        string
	commit         string
	date           string
	versionMessage = "#######################" +
		fmt.Sprintf("#### version %s commit %s date %s ####", version, commit, date) +
		"#######################"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(k8ssandraiov1alpha1.AddToScheme(scheme))
	utilruntime.Must(cassdcapi.AddToScheme(scheme))
	utilruntime.Must(cassctl.AddToScheme(scheme))
	utilruntime.Must(replicationapi.AddToScheme(scheme))
	utilruntime.Must(stargateapi.AddToScheme(scheme))
	utilruntime.Must(configapi.AddToScheme(scheme))
	utilruntime.Must(reaperapi.AddToScheme(scheme))
	utilruntime.Must(promapi.AddToScheme(scheme))
	utilruntime.Must(medusav1alpha1.AddToScheme(scheme))
	utilruntime.Must(controlv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	watchNamespace, err := getWatchNamespace()
	if err != nil {
		setupLog.Error(err, "unable to get WatchNamespace, "+
			"the manager will watch and manage resources in all namespaces")
	} else {
		setupLog.Info("watch namespace configured", "namespace", watchNamespace)
	}

	setupLog.Info(versionMessage)
	whServer := webhook.NewServer(webhook.Options{
		Port: 9443,
	})

	options := ctrl.Options{
		Scheme:                 scheme,
		WebhookServer:          whServer,
		LeaderElection:         enableLeaderElection,
		HealthProbeBindAddress: probeAddr,
		LeaderElectionID:       "dcabfccc.k8ssandra.io",
		Metrics: server.Options{
			BindAddress: metricsAddr,
		},
	}

	options.Cache = cache.Options{
		DefaultNamespaces: map[string]cache.Config{},
	}

	// Add support for MultiNamespace set in WATCH_NAMESPACE (e.g ns1,ns2)
	if strings.Contains(watchNamespace, ",") {
		setupLog.Info("manager set up with multiple namespaces", "namespaces", watchNamespace)
		// configure cluster-scoped with MultiNamespacedCacheBuilder
		namespaces := strings.Split(watchNamespace, ",")
		for _, namespace := range namespaces {
			options.Cache.DefaultNamespaces[namespace] = cache.Config{}
		}
	} else if watchNamespace != "" {
		setupLog.Info("Adding watch namespace to DefaultNamespaces", "namespace", watchNamespace)
		options.Cache.DefaultNamespaces[watchNamespace] = cache.Config{}
	} else {
		setupLog.Info("manager set up with cluster scope")
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	uncachedClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "unable to fetch config connection")
		os.Exit(1)
	}
	ctx, cancel := context.WithCancel(ctrl.SetupSignalHandler())
	reconcilerConfig := config.InitConfig()

	if isControlPlane() {
		// Fetch ClientConfigs and create the clientCache
		clientCache := clientcache.New(mgr.GetClient(), uncachedClient, scheme)

		configCtrler := &configctrl.ClientConfigReconciler{
			Scheme:      mgr.GetScheme(),
			ClientCache: clientCache,
		}

		additionalClusters, err := configCtrler.InitClientConfigs(ctx, mgr, watchNamespace)
		if err != nil {
			setupLog.Error(err, "unable to create manager cluster connections")
			os.Exit(1)
		}

		if err = configCtrler.SetupWithManager(mgr, cancel); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "ClientConfig")
			os.Exit(1)
		}

		if err = (&k8ssandractrl.K8ssandraClusterReconciler{
			ReconcilerConfig: reconcilerConfig,
			Client:           mgr.GetClient(),
			Scheme:           mgr.GetScheme(),
			ClientCache:      clientCache,
			ManagementApi:    cassandra.NewManagementApiFactory(),
			Recorder:         mgr.GetEventRecorderFor("k8ssandracluster-controller"),
		}).SetupWithManager(mgr, additionalClusters); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "K8ssandraCluster")
			os.Exit(1)
		}
		if err = k8ssandraiov1alpha1.SetupK8ssandraClusterWebhookWithManager(mgr, clientCache); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "K8ssandraCluster")
			os.Exit(1)
		}

		if err = (&replicationctrl.SecretSyncController{
			ReconcilerConfig: reconcilerConfig,
			ClientCache:      clientCache,
			WatchNamespaces:  strings.Split(watchNamespace, ","),
		}).SetupWithManager(mgr, additionalClusters, setupLog); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "SecretSync")
			os.Exit(1)
		}

		if err = (&controlcontrollers.K8ssandraTaskReconciler{
			ReconcilerConfig: reconcilerConfig,
			Client:           mgr.GetClient(),
			Scheme:           mgr.GetScheme(),
			ClientCache:      clientCache,
			Recorder:         mgr.GetEventRecorderFor("k8ssandratask-controller"),
		}).SetupWithManager(mgr, additionalClusters); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "K8ssandraTask")
			os.Exit(1)
		}
	}

	if err = (&stargatectrl.StargateReconciler{
		ReconcilerConfig: reconcilerConfig,
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		ManagementApi:    cassandra.NewManagementApiFactory(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Stargate")
		os.Exit(1)
	}

	if err = (&reaperctrl.ReaperReconciler{
		ReconcilerConfig: reconcilerConfig,
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		NewManager:       reaper.NewManager,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Reaper")
		os.Exit(1)
	}
	if err = (&medusactrl.MedusaTaskReconciler{
		ReconcilerConfig: reconcilerConfig,
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		ClientFactory:    &medusa.DefaultFactory{},
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MedusaTask")
		os.Exit(1)
	}
	if err = (&medusactrl.MedusaBackupJobReconciler{
		ReconcilerConfig: reconcilerConfig,
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		ClientFactory:    &medusa.DefaultFactory{},
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MedusaBackupJob")
		os.Exit(1)
	}
	if err = (&medusactrl.MedusaRestoreJobReconciler{
		ReconcilerConfig: reconcilerConfig,
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		ClientFactory:    &medusa.DefaultFactory{},
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MedusaRestoreJob")
	}

	if err = (&medusactrl.MedusaBackupScheduleReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Clock:  &medusactrl.RealClock{},
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MedusaBackupSchedule")
		os.Exit(1)
	}
	if err = medusav1alpha1.SetupMedusaBackupScheduleWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "MedusaBackupSchedule")
		os.Exit(1)
	}
	if err = (&medusactrl.MedusaConfigurationReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MedusaConfiguration")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	secretswebhook.SetupSecretsInjectorWebhook(mgr)

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

}

// getWatchNamespace returns the Namespace the operator should be watching for changes
func getWatchNamespace() (string, error) {
	// WatchNamespaceEnvVar is the constant for env variable WATCH_NAMESPACE
	// which specifies the Namespace to watch.
	// An empty value means the operator is running with cluster scope.
	var watchNamespaceEnvVar = "WATCH_NAMESPACE"

	ns, found := os.LookupEnv(watchNamespaceEnvVar)
	if !found {
		return "", fmt.Errorf("%s must be set", watchNamespaceEnvVar)
	}
	return ns, nil
}

func isControlPlane() bool {
	controlPlaneEnvVar := "K8SSANDRA_CONTROL_PLANE"
	val, found := os.LookupEnv(controlPlaneEnvVar)
	if !found {
		return false
	}

	return val == "true"
}
