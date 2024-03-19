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
	"fmt"
	"k8s.io/apimachinery/pkg/util/validation"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	clientCache        *clientcache.ClientCache
	ErrNumTokens       = fmt.Errorf("num_tokens value can't be changed")
	ErrReaperKeyspace  = fmt.Errorf("reaper keyspace can not be changed")
	ErrNoStorageConfig = fmt.Errorf("storageConfig must be defined at cluster level or dc level")
	ErrNoResourcesSet  = fmt.Errorf("softPodAntiAffinity requires Resources to be set")
	ErrClusterName     = fmt.Errorf("cluster name can not be changed")
	ErrNoStoragePrefix = fmt.Errorf("medusa storage prefix must be set when a medusaConfigurationRef is used")
)

// log is for logging in this package.
var webhookLog = logf.Log.WithName("k8ssandracluster-webhook")

func (r *K8ssandraCluster) SetupWebhookWithManager(mgr ctrl.Manager, cCache *clientcache.ClientCache) error {
	clientCache = cCache
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

var _ webhook.Defaulter = &K8ssandraCluster{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *K8ssandraCluster) Default() {
	webhookLog.Info("K8ssandraCluster default values", "K8ssandraCluster", r.Name)

}

//+kubebuilder:webhook:path=/validate-k8ssandra-io-v1alpha1-k8ssandracluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8ssandra.io,resources=k8ssandraclusters,verbs=create;update,versions=v1alpha1,name=vk8ssandracluster.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &K8ssandraCluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *K8ssandraCluster) ValidateCreate() error {
	webhookLog.Info("validate K8ssandraCluster create", "K8ssandraCluster", r.Name)

	return r.validateK8ssandraCluster()
}

func (r *K8ssandraCluster) validateK8ssandraCluster() error {
	hasClusterStorageConfig := r.Spec.Cassandra.DatacenterOptions.StorageConfig != nil
	for _, dc := range r.Spec.Cassandra.Datacenters {
		dns1035Errs := validation.IsDNS1035Label(dc.Meta.Name)
		if len(dns1035Errs) > 0 {
			return fmt.Errorf(
				"invalid DC name (you might want to use datacenterName to override the name used in Cassandra): %s",
				strings.Join(dns1035Errs, ", "))
		}

		// Verify given k8s-context is correct
		_, err := clientCache.GetRemoteClient(dc.K8sContext)
		if err != nil {
			// No client found for this context name, reject
			return errors.Wrap(err, fmt.Sprintf("unable to find k8sContext %s from ClientConfigs", dc.K8sContext))
		}

		// StorageConfig must be set at DC or Cluster level
		if dc.DatacenterOptions.StorageConfig == nil && !hasClusterStorageConfig {
			return ErrNoStorageConfig
		}
		// From cass-operator, if AllowMultipleWorkersPerNode is set, Resources must be defined or cass-operator will reject this Datacenter
		if dc.DatacenterOptions.SoftPodAntiAffinity != nil && *dc.DatacenterOptions.SoftPodAntiAffinity {
			if dc.DatacenterOptions.Resources == nil {
				return ErrNoResourcesSet
			}
		}
	}

	// Verify the Medusa storage prefix is explicitly set
	// only relevant if Medusa is enabled and the MedusaConfiguration object is referenced
	if r.Spec.Medusa != nil {
		if r.Spec.Medusa.MedusaConfigurationRef.Name != "" {
			if r.Spec.Medusa.StorageProperties.Prefix == "" {
				return ErrNoStoragePrefix
			}
		}
	}

	if err := r.validateStatefulsetNameSize(); err != nil {
		return err
	}

	return nil
}

func (r *K8ssandraCluster) validateStatefulsetNameSize() error {
	for _, dc := range r.Spec.Cassandra.Datacenters {
		if len(dc.Racks) > 0 {
			for _, rack := range dc.Racks {
				if len(r.SanitizedName()+"-"+dc.CassDcName()+"-"+rack.Name+"-sts-") > 60 {
					return fmt.Errorf("the name of the statefulset for rack %s in DC %s is too long", rack.Name, dc.CassDcName())
				}
			}
		} else {
			if len(r.SanitizedName()+"-"+dc.CassDcName()+"-default-sts-") > 60 {
				return fmt.Errorf("the name of the statefulset for DC %s is too long", dc.CassDcName())
			}
		}
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *K8ssandraCluster) ValidateUpdate(old runtime.Object) error {
	webhookLog.Info("validate K8ssandraCluster update", "K8ssandraCluster", r.Name)

	if err := r.validateK8ssandraCluster(); err != nil {
		return err
	}

	oldCluster, ok := old.(*K8ssandraCluster)
	if !ok {
		return fmt.Errorf("previous object could not be casted to K8ssandraCluster")
	}

	// Verify Reaper keyspace is not changed
	oldReaperSpec := oldCluster.Spec.Reaper
	reaperSpec := r.Spec.Reaper
	if reaperSpec != nil && oldReaperSpec != nil {
		if reaperSpec.Keyspace != oldReaperSpec.Keyspace {
			return ErrReaperKeyspace
		}
	}

	oldCassConfig := oldCluster.Spec.Cassandra.DatacenterOptions.CassandraConfig
	newCassConfig := r.Spec.Cassandra.DatacenterOptions.CassandraConfig
	if oldCassConfig != nil && newCassConfig != nil {
		oldNumTokens, oldNumTokensExists := oldCassConfig.CassandraYaml["num_tokens"]
		newNumTokens, newNumTokensExists := newCassConfig.CassandraYaml["num_tokens"]

		if !oldNumTokensExists {
			cassVersion, err := semver.NewVersion(oldCluster.Spec.Cassandra.ServerVersion)
			if err != nil {
				return err
			}
			defaultNumTokens := oldCluster.DefaultNumTokens(cassVersion)
			if newNumTokensExists && newNumTokens.(float64) != defaultNumTokens {
				return ErrNumTokens
			}
		} else {
			if oldNumTokens != newNumTokens {
				return ErrNumTokens
			}
		}
	}

	// Verify that the cluster name override was not changed
	if r.Spec.Cassandra.ClusterName != oldCluster.Spec.Cassandra.ClusterName {
		return ErrClusterName
	}

	// Some of these could be extracted in the cass-operator to reusable methods, do not copy code here.
	// Also, reusing methods from cass-operator allows to follow updates to features if they change in cass-operator,
	// such as allowing rack modifications or expanding PVCs.

	// TODO SoftPodAntiAffinity is not allowed to be modified
	// TODO StorageConfig can not be modified (not Cluster or DC level) in existing datacenters
	// TODO Racks can only be added and only at the end of the list - no other operation is allowed to racks

	if err := r.validateStatefulsetNameSize(); err != nil {
		return err
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *K8ssandraCluster) ValidateDelete() error {
	webhookLog.Info("validate K8ssandraCluster delete", "name", r.Name)
	return nil
}
