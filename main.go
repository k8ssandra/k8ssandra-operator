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
	"flag"
	"fmt"
	"os"

	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	k8ssandraiov1alpha1 "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/controllers"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(k8ssandraiov1alpha1.AddToScheme(scheme))
	utilruntime.Must(cassdcapi.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
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

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "dcabfccc.k8ssandra.io",
		Namespace:              watchNamespace,
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	uncachedClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "unable to fetch config connection")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()

	if isControlPlane() {
		// Fetch ClientConfigs and create the clientCache
		clientCache := clientcache.New(mgr.GetClient(), uncachedClient, scheme)

		cConfigs := k8ssandraiov1alpha1.ClientConfigList{}
		err = uncachedClient.List(ctx, &cConfigs, client.InNamespace(watchNamespace))
		if err != nil {
			setupLog.Error(err, "unable to fetch cluster connections")
			os.Exit(1)
		}

		additionalClusters := make([]cluster.Cluster, 0, len(cConfigs.Items))

		for _, cCfg := range cConfigs.Items {
			// Create clients and add them to the client cache
			cfg, err := clientCache.GetRestConfig(&cCfg)
			if err != nil {
				setupLog.Error(err, "unable to setup cluster connections")
				os.Exit(1)
			}

			_, err = clientCache.CreateClient(cCfg.GetContextName(), cfg)
			if err != nil {
				setupLog.Error(err, "unable to create cluster connection")
				os.Exit(1)
			}

			// Add cluster to the manager
			c, err := cluster.New(cfg, func(o *cluster.Options) {
				o.Scheme = scheme
				o.Namespace = watchNamespace
			})
			if err != nil {
				setupLog.Error(err, "unable to create manager cluster connection")
				os.Exit(1)
			}

			err = mgr.Add(c)
			if err != nil {
				setupLog.Error(err, "unable to add cluster to manager")
				os.Exit(1)
			}

			additionalClusters = append(additionalClusters, c)
		}

		// Create the reconciler and start it

		if err = (&controllers.K8ssandraClusterReconciler{
			Client:        mgr.GetClient(),
			Scheme:        mgr.GetScheme(),
			ClientCache:   clientCache,
			SeedsResolver: cassandra.NewRemoteSeedsResolver(),
		}).SetupWithManager(mgr, additionalClusters); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "K8ssandraCluster")
			os.Exit(1)
		}
	}

	if err = (&controllers.StargateReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Stargate")
		os.Exit(1)
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

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
