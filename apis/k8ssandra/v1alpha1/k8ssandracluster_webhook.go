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

	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	validationpkg "github.com/k8ssandra/k8ssandra-operator/pkg/validation"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	clientCache        *clientcache.ClientCache
	ErrNumTokens       = fmt.Errorf("num_tokens value can't be changed")
	ErrReaperKeyspace  = fmt.Errorf("reaper keyspace can not be changed")
	ErrNoStorageConfig = fmt.Errorf("storageConfig must be defined at cluster level or dc level")
	ErrNoResourcesSet  = fmt.Errorf("softPodAntiAffinity requires Resources to be set")
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
	hasClusterStorageConfig := r.Spec.Cassandra.StorageConfig != nil
	// Verify given k8s-contexts are correct

	for _, dc := range r.Spec.Cassandra.Datacenters {
		_, err := clientCache.GetRemoteClient(dc.K8sContext)
		if err != nil {
			// No client found for this context name, reject
			return errors.Wrap(err, fmt.Sprintf("unable to find k8sContext %s from ClientConfigs", dc.K8sContext))
		}

		// StorageConfig must be set at DC or Cluster level
		if dc.StorageConfig == nil && !hasClusterStorageConfig {
			return ErrNoStorageConfig
		}
		// From cass-operator, if AllowMultipleWorkersPerNode is set, Resources must be defined or cass-operator will reject this Datacenter
		if dc.SoftPodAntiAffinity != nil && *dc.SoftPodAntiAffinity {
			if dc.Resources == nil {
				return ErrNoResourcesSet
			}
		}
	}
	if err := TelemetrySpecsAreValid(r, clientCache); err != nil {
		return err
	}
	return nil
}

type clientGetter interface {
	GetRemoteClient(k8sContextName string) (client.Client, error)
}

func TelemetrySpecsAreValid(kCluster *K8ssandraCluster, clientCache clientGetter) error {
	// Validate Cassandra telemetry
	for _, dc := range kCluster.Spec.Cassandra.Datacenters {
		dcClient, err := clientCache.GetRemoteClient(dc.K8sContext)
		promInstalled, err := validationpkg.IsPromInstalled(dcClient, webhookLog)
		if err != nil {
			return err
		}
		cassToValidate := &telemetryapi.TelemetrySpec{}
		if kCluster.Spec.Cassandra.Telemetry != nil {
			cassToValidate = kCluster.Spec.Cassandra.Telemetry.Merge(dc.Telemetry)
		} else if dc.Telemetry != nil {
			cassToValidate = dc.Telemetry
		}
		if cassToValidate != nil {
			webhookLog.Info("validating cass telemetry in webhook", "cassToValidate", cassToValidate)
			cassIsValid, err := validationpkg.TelemetrySpecIsValid(cassToValidate, promInstalled)
			if err != nil {
				return err
			}
			if !cassIsValid {
				webhookLog.Info("throwing an error, cass telemetry spec invalid")
				return errors.New(fmt.Sprint("Cassandra telemetry specification was incorrect in context", dc.K8sContext))
			}

		}
		sgToValidate := &telemetryapi.TelemetrySpec{}
		if kCluster.Spec.Stargate != nil && kCluster.Spec.Stargate.Telemetry != nil {
			sgToValidate = kCluster.Spec.Stargate.Telemetry.Merge(dc.Stargate.Telemetry)
		} else if dc.Stargate != nil && dc.Stargate.Telemetry != nil {
			sgToValidate = dc.Stargate.Telemetry
		}
		if sgToValidate != nil {
			sgIsValid, err := validationpkg.TelemetrySpecIsValid(sgToValidate, promInstalled)
			if err != nil {
				return err
			}
			if !sgIsValid {
				return errors.New(fmt.Sprint("Stargate telemetry specification was incorrect in context", dc.K8sContext))
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

	oldCassConfig := oldCluster.Spec.Cassandra.CassandraConfig
	newCassConfig := r.Spec.Cassandra.CassandraConfig
	// If num_tokens was set previously, do not allow modifying the value or leaving it out
	if oldCassConfig != nil {
		if oldCassConfig.CassandraYaml.NumTokens != nil {
			// Changing num_tokens is not allowed
			if newCassConfig == nil {
				return ErrNumTokens
			} else if newCassConfig.CassandraYaml.NumTokens != oldCassConfig.CassandraYaml.NumTokens {
				return ErrNumTokens
			}
		}
	}
	// If num_tokens was unset previously, do not allow setting it now
	if newCassConfig != nil {
		if newCassConfig.CassandraYaml.NumTokens != nil {
			if oldCassConfig == nil {
				return ErrNumTokens
			} else if oldCassConfig.CassandraYaml.NumTokens == nil {
				return ErrNumTokens
			}
		}
	}

	// Some of these could be extracted in the cass-operator to reusable methods, do not copy code here.
	// Also, reusing methods from cass-operator allows to follow updates to features if they change in cass-operator,
	// such as allowing rack modifications or expanding PVCs.

	// TODO SoftPodAntiAffinity is not allowed to be modified
	// TODO StorageConfig can not be modified (not Cluster or DC level) in existing datacenters
	// TODO Racks can only be added and only at the end of the list - no other operation is allowed to racks

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *K8ssandraCluster) ValidateDelete() error {
	webhookLog.Info("validate K8ssandraCluster delete", "name", r.Name)
	return nil
}
