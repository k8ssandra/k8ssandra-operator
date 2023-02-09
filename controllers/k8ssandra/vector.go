package k8ssandra

import (
	"context"

	"github.com/go-logr/logr"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	k8ssandralabels "github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reconciliation"
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
	kcKey := utils.GetKey(kc)
	namespace := utils.FirstNonEmptyString(dcConfig.Meta.Namespace, kc.Namespace)
	configMapKey := client.ObjectKey{
		Namespace: namespace,
		Name:      telemetry.VectorAgentConfigMapName(kc.SanitizedName(), dcConfig.Meta.Name),
	}
	if kc.Spec.Cassandra.Telemetry.IsVectorEnabled() {
		// Create the vector toml config content
		toml, err := telemetry.CreateCassandraVectorToml(kc.Spec.Cassandra.Telemetry, dcConfig.McacEnabled)
		if err != nil {
			return result.Error(err)
		}

		desiredVectorConfigMap := telemetry.BuildVectorAgentConfigMap(namespace, kc.SanitizedName(), dcConfig.Meta.Name, kc.Namespace, toml)
		k8ssandralabels.WatchedByK8ssandraCluster(kcKey).AddTo(desiredVectorConfigMap)

		// Check if the vector config map already exists
		recRes := reconciliation.ReconcileObject(ctx, remoteClient, r.DefaultDelay, *desiredVectorConfigMap)
		switch {
		case recRes.IsError():
			return recRes
		case recRes.IsRequeue():
			return recRes
		}
	} else {
		if err := deleteConfigMapIfExists(ctx, remoteClient, configMapKey, dcLogger); err != nil {
			return result.Error(err)
		}
	}

	dcLogger.Info("Vector Agent ConfigMap successfully reconciled")
	return result.Continue()
}

func deleteConfigMapIfExists(ctx context.Context, remoteClient client.Client, configMapKey client.ObjectKey, logger logr.Logger) error {
	configMap := &corev1.ConfigMap{}
	if err := remoteClient.Get(ctx, configMapKey, configMap); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		logger.Error(err, "Failed to get ConfigMap", configMapKey)
		return err
	}
	if err := remoteClient.Delete(ctx, configMap); err != nil {
		logger.Error(err, "Failed to delete ConfigMap", configMapKey)
		return err
	}
	return nil
}
