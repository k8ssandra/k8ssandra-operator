package k8ssandra

import (
	"context"
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	k8ssandralabels "github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reconciliation"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/shared"
	"github.com/k8ssandra/k8ssandra-operator/pkg/telemetry"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	v1 "k8s.io/api/core/v1"
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
		k8ssandralabels.SetWatchedByK8ssandraCluster(desiredVectorConfigMap, kcKey)

		// Check if the vector config map already exists
		desiredVectorConfigMap.SetLabels(labels.CleanedUpByLabels(kcKey))
		recRes := reconciliation.ReconcileObject(ctx, remoteClient, r.DefaultDelay, *desiredVectorConfigMap)
		switch {
		case recRes.IsError():
			return recRes
		case recRes.IsRequeue():
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

// setupVectorCleanup adds owner references to ensure that the remote resources created by reconcileVector are correctly
// cleaned up when the CassandraDatacenter is deleted. We do that in a second pass because the CassandraDatacenter did
// not exist yet at the time those resources were created.
func (r *K8ssandraClusterReconciler) setupVectorCleanup(
	ctx context.Context,
	kc *k8ssandraapi.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	remoteClient client.Client,
	logger logr.Logger,
) result.ReconcileResult {
	configMapKey := client.ObjectKey{
		Namespace: dc.Namespace,
		Name:      telemetry.VectorAgentConfigMapName(kc.SanitizedName(), dc.SanitizedName()),
	}
	return setDcOwnership(ctx, dc, configMapKey, &v1.ConfigMap{}, remoteClient, r.Scheme, logger)
}
