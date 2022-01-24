package k8ssandra

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/stargate"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *K8ssandraClusterReconciler) reconcileStargate(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	dcTemplate api.CassandraDatacenterTemplate,
	actualDc *cassdcapi.CassandraDatacenter,
	logger logr.Logger,
	remoteClient client.Client,
) result.ReconcileResult {

	kcKey := client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}
	stargateTemplate := dcTemplate.Stargate.Coalesce(kc.Spec.Stargate)
	stargateKey := types.NamespacedName{
		Namespace: actualDc.Namespace,
		Name:      stargate.ResourceName(actualDc),
	}
	actualStargate := &stargateapi.Stargate{}
	logger = logger.WithValues("Stargate", stargateKey)

	if stargateTemplate != nil {
		logger.Info("Reconcile Stargate")

		desiredStargate := r.newStargate(stargateKey, kc, stargateTemplate, actualDc)
		annotations.AddHashAnnotation(desiredStargate)

		if err := remoteClient.Get(ctx, stargateKey, actualStargate); err != nil {
			if errors.IsNotFound(err) {
				logger.Info("Creating Stargate resource")
				if err := remoteClient.Create(ctx, desiredStargate); err != nil {
					logger.Error(err, "Failed to create Stargate resource")
					return result.Error(err)
				} else {
					return result.RequeueSoon(r.DefaultDelay)
				}
			} else {
				logger.Error(err, "Failed to get Stargate resource")
				return result.Error(err)
			}
		} else {
			if err = r.setStatusForStargate(kc, actualStargate, dcTemplate.Meta.Name); err != nil {
				logger.Error(err, "Failed to update status for stargate")
				return result.Error(err)
			}
			if !annotations.CompareHashAnnotations(desiredStargate, actualStargate) {
				logger.Info("Updating Stargate")
				resourceVersion := actualStargate.GetResourceVersion()
				desiredStargate.DeepCopyInto(actualStargate)
				actualStargate.SetResourceVersion(resourceVersion)
				if err = remoteClient.Update(ctx, actualStargate); err == nil {
					return result.RequeueSoon(r.DefaultDelay)
				} else {
					logger.Error(err, "Failed to update Stargate")
					return result.Error(err)
				}
			}
			if !actualStargate.Status.IsReady() {
				logger.Info("Waiting for Stargate to become ready")
				return result.RequeueSoon(r.DefaultDelay)
			}
			logger.Info("Stargate is ready")
		}
	} else {
		logger.Info("Stargate not present")

		// Test if Stargate was removed
		if err := remoteClient.Get(ctx, stargateKey, actualStargate); err != nil {
			if errors.IsNotFound(err) {
				// OK
			} else {
				logger.Error(err, "Failed to get Stargate")
				return result.Error(err)
			}
		} else if labels.IsPartOf(actualStargate, kcKey) {
			if err := remoteClient.Delete(ctx, actualStargate); err != nil {
				logger.Error(err, "Failed to delete Stargate")
				return result.Error(err)
			} else {
				r.removeStargateStatus(kc, dcTemplate.Meta.Name)
				logger.Info("Stargate deleted")
			}
		} else {
			logger.Info("Not deleting Stargate since it wasn't created by this controller")
		}
	}
	return result.Continue()
}

// TODO move to stargate package
func (r *K8ssandraClusterReconciler) newStargate(stargateKey types.NamespacedName, kc *api.K8ssandraCluster, stargateTemplate *stargateapi.StargateDatacenterTemplate, actualDc *cassdcapi.CassandraDatacenter) *stargateapi.Stargate {
	desiredStargate := &stargateapi.Stargate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   stargateKey.Namespace,
			Name:        stargateKey.Name,
			Annotations: map[string]string{},
			Labels: map[string]string{
				api.NameLabel:                      api.NameLabelValue,
				api.PartOfLabel:                    api.PartOfLabelValue,
				api.ComponentLabel:                 api.ComponentLabelValueStargate,
				api.CreatedByLabel:                 api.CreatedByLabelValueK8ssandraClusterController,
				api.K8ssandraClusterNameLabel:      kc.Name,
				api.K8ssandraClusterNamespaceLabel: kc.Namespace,
			},
		},
		Spec: stargateapi.StargateSpec{
			StargateDatacenterTemplate: *stargateTemplate,
			DatacenterRef:              corev1.LocalObjectReference{Name: actualDc.Name},
			Auth:                       kc.Spec.Auth,
			ClientEncryptionStores:     kc.Spec.Cassandra.ClientEncryptionStores,
			ServerEncryptionStores:     kc.Spec.Cassandra.ServerEncryptionStores,
		},
	}
	return desiredStargate
}

func (r *K8ssandraClusterReconciler) setStatusForStargate(kc *api.K8ssandraCluster, stargate *stargateapi.Stargate, dcName string) error {
	if len(kc.Status.Datacenters) == 0 {
		kc.Status.Datacenters = make(map[string]api.K8ssandraStatus)
	}

	kdcStatus, found := kc.Status.Datacenters[dcName]

	if found {
		if kdcStatus.Stargate == nil {
			kdcStatus.Stargate = stargate.Status.DeepCopy()
			kc.Status.Datacenters[dcName] = kdcStatus
		} else {
			stargate.Status.DeepCopyInto(kdcStatus.Stargate)
		}
	} else {
		kc.Status.Datacenters[dcName] = api.K8ssandraStatus{
			Stargate: stargate.Status.DeepCopy(),
		}
	}

	if kc.Status.Datacenters[dcName].Stargate.Progress == "" {
		kc.Status.Datacenters[dcName].Stargate.Progress = stargateapi.StargateProgressPending
	}
	return nil
}

func (r *K8ssandraClusterReconciler) reconcileStargateAuthSchema(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	mgmtApi cassandra.ManagementApiFacade,
	logger logr.Logger) result.ReconcileResult {

	if !kc.HasStargates() {
		return result.Continue()
	}

	if recResult := r.versionCheck(ctx, kc); recResult.Completed() {
		return recResult
	}

	datacenters := kc.GetReadyDatacenters()
	replication := cassandra.ComputeReplicationFromDcTemplates(3, datacenters...)

	if err := stargate.ReconcileAuthKeyspace(mgmtApi, replication, logger); err != nil {
		return result.Error(err)
	}

	return result.Continue()
}

func (r *K8ssandraClusterReconciler) removeStargateStatus(kc *api.K8ssandraCluster, dcName string) {
	if kdcStatus, found := kc.Status.Datacenters[dcName]; found {
		kc.Status.Datacenters[dcName] = api.K8ssandraStatus{
			Stargate:  nil,
			Cassandra: kdcStatus.Cassandra.DeepCopy(),
			Reaper:    kdcStatus.Reaper.DeepCopy(),
		}
	}
}

func (r *K8ssandraClusterReconciler) reconcileStargateConfigMap(
	ctx context.Context,
	remoteClient client.Client,
	kc *api.K8ssandraCluster,
	desiredConfig map[string]interface{},
	userConfigMapContent,
	namespace string,
	logger logr.Logger,
) result.ReconcileResult {
	logger.Info("Reconciling Stargate Cassandra yaml configMap on namespace : " + namespace)
	filteredCassandraConfig := filterYamlConfig(desiredConfig["cassandra-yaml"].(map[string]interface{}))

	configYamlString := ""
	if len(filteredCassandraConfig) > 0 {
		if configYaml, err := yaml.Marshal(filteredCassandraConfig); err != nil {
			return result.Error(err)
		} else {
			configYamlString = string(configYaml)
		}
	}
	logger.Info(fmt.Sprintf("Stargate configMap content : %s", configYamlString))

	if userConfigMapContent != "" {
		separator := "\n"
		if strings.HasSuffix(userConfigMapContent, "\n") {
			separator = ""
		}
		configYamlString = userConfigMapContent + separator + configYamlString
		logger.Info(fmt.Sprintf("Stargate configMap updated content : %s", configYamlString))
	}

	configMapKey := client.ObjectKey{
		Namespace: namespace,
		Name:      stargate.CassandraConfigMap,
	}

	logger = logger.WithValues("StargateConfigMap", configMapKey)
	desiredConfigMap := createStargateConfigMap(namespace, configYamlString, kc)
	// Compute a hash which will allow to compare desired and actual configMaps
	annotations.AddHashAnnotation(desiredConfigMap)
	actualConfigMap := &corev1.ConfigMap{}

	if err := remoteClient.Get(ctx, configMapKey, actualConfigMap); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Stargate configMap doesn't exist, creating it")
			if err := remoteClient.Create(ctx, desiredConfigMap); err != nil {
				logger.Error(err, "Failed to create Stargate ConfigMap")
				return result.Error(err)
			} else {
				// Let's wait for the configMap to be created
				return result.RequeueSoon(r.DefaultDelay)
			}
		}
	} else {
		logger.Info("Stargate configMap already exists")
	}

	actualConfigMap = actualConfigMap.DeepCopy()

	if !annotations.CompareHashAnnotations(actualConfigMap, desiredConfigMap) {
		logger.Info("Updating configMap on namespace " + actualConfigMap.ObjectMeta.Namespace)
		resourceVersion := actualConfigMap.GetResourceVersion()
		desiredConfigMap.DeepCopyInto(actualConfigMap)
		actualConfigMap.SetResourceVersion(resourceVersion)
		if err := remoteClient.Update(ctx, actualConfigMap); err != nil {
			logger.Error(err, "Failed to update Stargate ConfigMap resource")
			return result.Error(err)
		}
		return result.RequeueSoon(r.DefaultDelay)
	}
	logger.Info("Stargate ConfigMap successfully reconciled")
	return result.Continue()
}

func filterYamlConfig(config map[string]interface{}) map[string]interface{} {
	allowedConfigSettings := []string{"server_encryption_options", "client_encryption_options"}
	filteredConfig := make(map[string]interface{})
	for k, v := range config {
		// check if the key is allowed
		if utils.SliceContains(allowedConfigSettings, k) {
			filteredConfig[k] = v
		}
	}
	return filteredConfig
}

func createStargateConfigMap(namespace, configYaml string, kc *api.K8ssandraCluster) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      stargate.CassandraConfigMap,
			Namespace: namespace,
			Labels: map[string]string{
				api.NameLabel:                      api.NameLabelValue,
				api.PartOfLabel:                    api.PartOfLabelValue,
				api.ComponentLabel:                 api.ComponentLabelValueStargate,
				api.CreatedByLabel:                 api.CreatedByLabelValueK8ssandraClusterController,
				api.K8ssandraClusterNameLabel:      kc.Name,
				api.K8ssandraClusterNamespaceLabel: kc.Namespace,
			},
		},
		Data: map[string]string{
			"cassandra.yaml": configYaml,
		},
	}
}
