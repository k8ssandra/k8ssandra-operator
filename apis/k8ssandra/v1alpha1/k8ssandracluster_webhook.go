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
	"strings"

	"github.com/Masterminds/semver/v3"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	clientCache                 *clientcache.ClientCache
	ErrNumTokens                = fmt.Errorf("num_tokens value can't be changed")
	ErrReaperKeyspace           = fmt.Errorf("reaper keyspace can not be changed")
	ErrNoStorageConfig          = fmt.Errorf("storageConfig must be defined at cluster level or dc level")
	ErrNoResourcesSet           = fmt.Errorf("softPodAntiAffinity requires Resources to be set")
	ErrClusterName              = fmt.Errorf("cluster name can not be changed")
	ErrNoStoragePrefix          = fmt.Errorf("medusa storage prefix must be set when a medusaConfigurationRef is used")
	ErrNoReaperStorageConfig    = fmt.Errorf("reaper StorageConfig not set")
	ErrNoReaperAccessMode       = fmt.Errorf("reaper StorageConfig.AccessModes not set")
	ErrNoReaperResourceRequests = fmt.Errorf("reaper StorageConfig.Resources.Requests not set")
	ErrNoReaperStorageRequest   = fmt.Errorf("reaper StorageConfig.Resources.Requests.Storage not set")
	ErrNoReaperPerDcWithLocal   = fmt.Errorf("reaper DeploymentModePerDc is only supported when using cassandra storage")
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
func (r *K8ssandraCluster) ValidateCreate() (admission.Warnings, error) {
	webhookLog.Info("validate K8ssandraCluster create", "K8ssandraCluster", r.Name)

	return ValidateDeprecatedFieldUsage(r), r.validateK8ssandraCluster()
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

	if metav1.HasAnnotation(r.ObjectMeta, AutomatedUpdateAnnotation) {
		// Allow only always and once in the annotation
		annotationValue := r.ObjectMeta.GetAnnotations()[AutomatedUpdateAnnotation]
		if annotationValue != string(AllowUpdateAlways) && annotationValue != string(AllowUpdateOnce) {
			return fmt.Errorf("invalid value for %s annotation: %s", AutomatedUpdateAnnotation, annotationValue)
		}
	}

	if err := r.ValidateMedusa(); err != nil {
		return err
	}

	if err := r.validateReaper(); err != nil {
		return err
	}

	if err := r.validateStatefulsetNameSize(); err != nil {
		return err
	}

	return nil
}

func (r *K8ssandraCluster) validateStatefulsetNameSize() error {
	clusterName := r.ObjectMeta.Name
	if r.Spec.Cassandra.ClusterName != "" {
		clusterName = r.Spec.Cassandra.ClusterName
	}

	for _, dc := range r.Spec.Cassandra.Datacenters {
		realDc := &cassdcapi.CassandraDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Name: dc.Meta.Name,
			},
			Spec: cassdcapi.CassandraDatacenterSpec{
				ClusterName: clusterName,
			},
		}

		if len(dc.Racks) > 0 {
			for _, rack := range dc.Racks {
				stsName := reconciliation.NewNamespacedNameForStatefulSet(realDc, rack.Name)
				if len(stsName.Name) > 60 {
					return fmt.Errorf("the name of the statefulset for rack %s in DC %s is too long", rack.Name, dc.CassDcName())
				}
			}
		} else {
			stsName := reconciliation.NewNamespacedNameForStatefulSet(realDc, "default")
			if len(stsName.Name) > 60 {
				return fmt.Errorf("the name of the statefulset for DC %s is too long", dc.CassDcName())
			}
		}
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *K8ssandraCluster) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	webhookLog.Info("validate K8ssandraCluster update", "K8ssandraCluster", r.Name)

	if err := r.validateK8ssandraCluster(); err != nil {
		return nil, err
	}

	oldCluster, ok := old.(*K8ssandraCluster)
	if !ok {
		return nil, fmt.Errorf("previous object could not be casted to K8ssandraCluster")
	}

	// Verify Reaper keyspace is not changed
	oldReaperSpec := oldCluster.Spec.Reaper
	reaperSpec := r.Spec.Reaper
	if reaperSpec != nil && oldReaperSpec != nil {
		if reaperSpec.Keyspace != oldReaperSpec.Keyspace {
			return nil, ErrReaperKeyspace
		}
	}

	if err := validateUpdateNumTokens(oldCluster.Spec.Cassandra, r.Spec.Cassandra); err != nil {
		return nil, err
	}
	if DcRemoved(oldCluster.Spec, r.Spec) && DcAdded(oldCluster.Spec, r.Spec) {
		return nil, fmt.Errorf("renaming, as well as adding and removing DCs at the same time is prohibited as it can cause data loss")
	}
	// Verify that the cluster name override was not changed
	if r.Spec.Cassandra.ClusterName != oldCluster.Spec.Cassandra.ClusterName {
		return nil, ErrClusterName
	}

	// Some of these could be extracted in the cass-operator to reusable methods, do not copy code here.
	// Also, reusing methods from cass-operator allows to follow updates to features if they change in cass-operator,
	// such as allowing rack modifications or expanding PVCs.

	// TODO SoftPodAntiAffinity is not allowed to be modified
	// TODO StorageConfig can not be modified (not Cluster or DC level) in existing datacenters
	// TODO Racks can only be added and only at the end of the list - no other operation is allowed to racks

	if err := r.validateStatefulsetNameSize(); err != nil {
		return nil, err
	}

	return ValidateDeprecatedFieldUsage(r), nil
}

func validateUpdateNumTokens(
	oldCassandra *CassandraClusterTemplate,
	newCassandra *CassandraClusterTemplate,
) error {
	oldNumTokensPerDc, err := numTokensPerDc(oldCassandra)
	if err != nil {
		return err
	}
	newNumTokensPerDc, err := numTokensPerDc(newCassandra)
	if err != nil {
		return err
	}

	for dcName, newNumTokens := range newNumTokensPerDc {
		oldNumTokens, oldExists := oldNumTokensPerDc[dcName]
		if oldExists && oldNumTokens != newNumTokens {
			return ErrNumTokens
		}
	}
	return nil
}

func numTokensPerDc(cassandra *CassandraClusterTemplate) (map[string]interface{}, error) {
	var globalNumTokens interface{}
	globalConfig := cassandra.DatacenterOptions.CassandraConfig
	if globalConfig != nil {
		globalNumTokens = globalConfig.CassandraYaml["num_tokens"]
	}

	numTokensPerDc := make(map[string]interface{})
	for _, dc := range cassandra.Datacenters {
		var numTokens interface{}
		// Try to set from DC config
		config := dc.DatacenterOptions.CassandraConfig
		if config != nil {
			numTokens = config.CassandraYaml["num_tokens"]
		}
		// Otherwise, try from global config
		if numTokens == nil {
			numTokens = globalNumTokens
		}
		// Otherwise, use version-specific default
		if numTokens == nil {
			versionString := dc.ServerVersion
			if versionString == "" {
				versionString = cassandra.ServerVersion
			}
			if versionString == "" {
				return nil, errors.New("serverVersion should be set globally or at DC level")
			}
			version, err := semver.NewVersion(versionString)
			if err != nil {
				return nil, err
			}
			if cassandra.ServerType.IsCassandra() && version.Major() == 3 {
				numTokens = float64(256)
			} else {
				numTokens = float64(16)
			}
		}
		numTokensPerDc[dc.Meta.Name] = numTokens
	}
	return numTokensPerDc, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *K8ssandraCluster) ValidateDelete() (admission.Warnings, error) {
	webhookLog.Info("validate K8ssandraCluster delete", "name", r.Name)
	return nil, nil
}

func (r *K8ssandraCluster) ValidateMedusa() error {
	if r.Spec.Medusa == nil {
		return nil
	}

	// Verify the Medusa storage prefix is explicitly set
	// only relevant if Medusa is enabled and the MedusaConfiguration object is referenced
	if r.Spec.Medusa.MedusaConfigurationRef.Name != "" {
		if r.Spec.Medusa.StorageProperties.Prefix == "" {
			return ErrNoStoragePrefix
		}
		// Verify that any referenced MedusaConfig is NS-local
		if r.Spec.Medusa.MedusaConfigurationRef.Namespace != "" {
			return errors.New("Medusa config must be namespace local")
		}
		if r.Spec.Medusa.MedusaConfigurationRef.APIVersion != "" ||
			r.Spec.Medusa.MedusaConfigurationRef.Kind != "" ||
			r.Spec.Medusa.MedusaConfigurationRef.FieldPath != "" ||
			r.Spec.Medusa.MedusaConfigurationRef.ResourceVersion != "" ||
			r.Spec.Medusa.MedusaConfigurationRef.UID != "" {
			return errors.New("Medusa config invalid, invalid field used")
		}
	}

	return nil
}

func (r *K8ssandraCluster) validateReaper() error {
	if r.Spec.Reaper == nil {
		return nil
	}
	if r.Spec.Reaper.StorageType == reaperapi.StorageTypeLocal && r.Spec.Reaper.StorageConfig == nil {
		return ErrNoReaperStorageConfig
	}
	if r.Spec.Reaper.StorageType == reaperapi.StorageTypeLocal {
		// we're checking for validity of the storage config in Reaper's webhook too, so this is a duplicate of that
		// for now, I don't see a better way of reusing code to validate the storage config
		// not checking StorageClassName because Kubernetes will use a default one if it's not set
		if r.Spec.Reaper.StorageConfig.AccessModes == nil {
			return ErrNoReaperAccessMode
		}
		if r.Spec.Reaper.StorageConfig.Resources.Requests == nil {
			return ErrNoReaperResourceRequests
		}
		if r.Spec.Reaper.StorageConfig.Resources.Requests.Storage().IsZero() {
			return ErrNoReaperStorageRequest
		}
	}
	if r.Spec.Reaper.StorageType == reaperapi.StorageTypeLocal && r.Spec.Reaper.DeploymentMode == reaperapi.DeploymentModePerDc {
		return ErrNoReaperPerDcWithLocal
	}
	return nil
}

// ValidateDeprecatedFieldUsage adds warning about fields that are deprecated
func ValidateDeprecatedFieldUsage(r *K8ssandraCluster) admission.Warnings {
	warnings := admission.Warnings{}

	if r.Spec.Stargate != nil {
		warnings = append(warnings, deprecatedWarning("stargate", "", ""))
	}

	if r.Spec.Cassandra != nil && len(r.Spec.Cassandra.Datacenters) > 0 {
		for _, dc := range r.Spec.Cassandra.Datacenters {
			if dc.Stargate != nil {
				warnings = append(warnings, deprecatedWarning("cassandra.datacenters.stargate", "", ""))
			}
		}
	}

	return warnings
}

func deprecatedWarning(field, instead, extra string) string {
	warning := fmt.Sprintf("K8ssandraCluster is using deprecated field '%s'", field)
	if instead != "" {
		warning += fmt.Sprintf(", use '%s' instead", instead)
	}
	if extra != "" {
		warning += ". %s"
	}
	return warning
}
