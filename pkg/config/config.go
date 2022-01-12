package config

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	configapi "github.com/k8ssandra/k8ssandra-operator/apis/config/v1beta1"
)

type ReconcilerConfig struct {
	DefaultDelay time.Duration
	LongDelay    time.Duration
}

const (
	RequeueDefaultDelayEnvVar = "REQUEUE_DEFAULT_DELAY"
	RequeueLongDelayEnvVar    = "REQUEUE_LONG_DELAY"
)

// InitConfig is primarily a hook for integration tests. It provides a way to use shorter
// requeue delays which allows the tests to run much faster. Note that this code will
// likely be changed when we tackle
// https://github.com/k8ssandra/k8ssandra-operator/issues/63.
func InitConfig() *ReconcilerConfig {
	var (
		defaultDelay time.Duration
		longDelay    time.Duration
		err          error
	)

	val, found := os.LookupEnv(RequeueDefaultDelayEnvVar)
	if found {
		defaultDelay, err = time.ParseDuration(val)
		if err != nil {
			log.Fatalf("failed to parse value for %s %s: %s", RequeueDefaultDelayEnvVar, val, err)
		}
	} else {
		defaultDelay = 15 * time.Second
	}

	val, found = os.LookupEnv(RequeueLongDelayEnvVar)
	if found {
		longDelay, err = time.ParseDuration(val)
		if err != nil {
			log.Fatalf("failed to parse value for %s %s: %s", RequeueLongDelayEnvVar, val, err)
		}
	} else {
		longDelay = 1 * time.Minute
	}

	return &ReconcilerConfig{
		DefaultDelay: defaultDelay,
		LongDelay:    longDelay,
	}
}

// InitClientConfigs will fetch clientConfigs from the current cluster (control plane cluster) and create all the required Cluster objects for
// other controllers to use
func InitClientConfigs(ctx context.Context, mgr ctrl.Manager, clientCache *clientcache.ClientCache, scheme *runtime.Scheme, watchNamespace string) ([]cluster.Cluster, error) {
	uncachedClient := clientCache.GetLocalNonCacheClient()
	clientConfigs := make([]configapi.ClientConfig, 0)
	namespaces := strings.Split(watchNamespace, ",")

	for _, ns := range namespaces {
		cConfigs := configapi.ClientConfigList{}
		err := uncachedClient.List(ctx, &cConfigs, client.InNamespace(ns))
		if err != nil {
			return nil, err
		}
		clientConfigs = append(clientConfigs, cConfigs.Items...)
	}

	additionalClusters := make([]cluster.Cluster, 0, len(clientConfigs))

	for _, cCfg := range clientConfigs {
		// TODO If the cCfg does not have hash annotation, add it
		// Create clients and add them to the client cache
		cfg, err := clientCache.GetRestConfig(&cCfg)
		if err != nil {
			return nil, err
		}

		// Add cluster to the manager
		var c cluster.Cluster
		if strings.Contains(watchNamespace, ",") {
			c, err = cluster.New(cfg, func(o *cluster.Options) {
				o.Scheme = scheme
				o.Namespace = ""
				o.NewCache = cache.MultiNamespacedCacheBuilder(strings.Split(watchNamespace, ","))
			})
		} else {
			c, err = cluster.New(cfg, func(o *cluster.Options) {
				o.Scheme = scheme
				o.Namespace = watchNamespace
			})
		}
		if err != nil {
			return nil, err
		}

		clientCache.AddClient(cCfg.GetContextName(), c.GetClient())

		err = mgr.Add(c)
		if err != nil {
			return nil, err
		}

		additionalClusters = append(additionalClusters, c)
	}

	return additionalClusters, nil
}
