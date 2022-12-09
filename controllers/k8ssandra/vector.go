package k8ssandra

import (
	"context"

	"github.com/go-logr/logr"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	k8ssandralabels "github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/telemetry"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *K8ssandraClusterReconciler) reconcileVector(
	ctx context.Context,
	kc *k8ssandraapi.K8ssandraCluster,
	dcConfig *cassandra.DatacenterConfig,
	remoteClient client.Client,
	dcLogger logr.Logger,
) result.ReconcileResult {
	if kc.Spec.Cassandra.Telemetry.IsVectorEnabled() {
		kcKey := utils.GetKey(kc)
		namespace := utils.FirstNonEmptyString(dcConfig.Meta.Namespace, kc.Namespace)
		configMapKey := client.ObjectKey{
			Namespace: namespace,
			Name:      telemetry.VectorAgentConfigMapName(kc.SanitizedName()),
		}
		// Create the vector toml config content
		toml, err := telemetry.CreateCassandraVectorToml(ctx, kc.Spec.Cassandra.Telemetry, dcConfig, remoteClient, namespace)
		if err != nil {
			return result.Error(err)
		}

		desiredVectorConfigMap := telemetry.BuildVectorAgentConfigMap(namespace, kc.SanitizedName(), toml)
		annotations.AddHashAnnotation(desiredVectorConfigMap)
		k8ssandralabels.SetManagedBy(desiredVectorConfigMap, kcKey)

		// Check if the vector config map already exists
		actualVectorConfigMap := &corev1.ConfigMap{}

		if err := remoteClient.Get(ctx, configMapKey, actualVectorConfigMap); err != nil {
			if errors.IsNotFound(err) {
				if err := remoteClient.Create(ctx, desiredVectorConfigMap); err != nil {
					dcLogger.Error(err, "Failed to create Vector Agent ConfigMap")
					return result.Error(err)
				}
			}
		}

		actualVectorConfigMap = actualVectorConfigMap.DeepCopy()

		if !annotations.CompareHashAnnotations(actualVectorConfigMap, desiredVectorConfigMap) {
			resourceVersion := actualVectorConfigMap.GetResourceVersion()
			desiredVectorConfigMap.DeepCopyInto(actualVectorConfigMap)
			actualVectorConfigMap.SetResourceVersion(resourceVersion)
			if err := remoteClient.Update(ctx, actualVectorConfigMap); err != nil {
				dcLogger.Error(err, "Failed to update Vector Agent ConfigMap resource")
				return result.Error(err)
			}
			return result.RequeueSoon(r.DefaultDelay)
		}
	}

	dcLogger.Info("Vector Agent ConfigMap successfully reconciled")
	return result.Continue()
}
