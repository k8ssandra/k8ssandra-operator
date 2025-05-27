package k8ssandra

import (
	"context"

	"github.com/k8ssandra/k8ssandra-operator/pkg/shared"

	"github.com/go-logr/logr"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	annotations "github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	labels "github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reconciliation"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/telemetry"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *K8ssandraClusterReconciler) reconcileVector(
	ctx context.Context,
	kc *k8ssandraapi.K8ssandraCluster,
	dcConfig *cassandra.DatacenterConfig,
	remoteClient client.Client,
	dcLogger logr.Logger,
) result.ReconcileResult {
	kcKey := utils.GetKey(kc)
	namespace := utils.FirstNonEmptyString(dcConfig.Meta.Namespace, kc.Namespace)
	configMapKey := client.ObjectKey{
		Namespace: namespace,
		Name:      telemetry.VectorAgentConfigMapName(kc.SanitizedName(), dcConfig.CassDcName()),
	}
	if kc.Spec.Cassandra.Telemetry.IsVectorEnabled() {
		// Create the vector toml config content
		toml, err := telemetry.CreateCassandraVectorToml(kc.Spec.Cassandra.Telemetry, dcConfig.McacEnabled)
		if err != nil {
			return result.Error(err)
		}

		desiredVectorConfigMap := telemetry.BuildVectorAgentConfigMap(namespace, kc.SanitizedName(), dcConfig.CassDcName(), kc.Namespace, toml)
		labels.AddCommonLabels(desiredVectorConfigMap, kc)
		annotations.AddCommonAnnotations(desiredVectorConfigMap, kc)
		labels.SetWatchedByK8ssandraCluster(desiredVectorConfigMap, kcKey)

		// Check if the vector config map already exists
		desiredVectorConfigMap.SetLabels(labels.CleanedUpByLabels(kcKey))
		if recRes := reconciliation.ReconcileObject(ctx, remoteClient, r.DefaultDelay, *desiredVectorConfigMap); recRes.Completed() {
			return recRes
		}
	} else {
		if err := shared.DeleteConfigMapIfExists(ctx, remoteClient, configMapKey, dcLogger); err != nil {
			return result.Error(err)
		}
	}

	dcLogger.Info("Vector Agent ConfigMap successfully reconciled")
	return result.Continue()
}
