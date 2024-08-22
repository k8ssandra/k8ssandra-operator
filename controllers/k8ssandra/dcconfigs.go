package k8ssandra

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reaper"
	"github.com/k8ssandra/k8ssandra-operator/pkg/telemetry"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/types"
)

// This method merges the cluster and datacenter level DC templates into a single object, then
// applies various defaults, and validates the resulting object. This method does NOT create the
// actual DCs, nor any other dependent object such as ConfigMaps or Secrets; but it does all the
// preparatory work required before starting creating such objects.
func (r *K8ssandraClusterReconciler) createDatacenterConfigs(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	logger logr.Logger,
	systemReplication cassandra.SystemReplication,
) ([]*cassandra.DatacenterConfig, error) {

	kcKey := utils.GetKey(kc)
	var dcConfigs []*cassandra.DatacenterConfig

	for _, dcTemplate := range kc.Spec.Cassandra.Datacenters {

		dcConfig := cassandra.Coalesce(kc.CassClusterName(), kc.Spec.Cassandra.DeepCopy(), dcTemplate.DeepCopy())
		dcConfig.ExternalSecrets = kc.Spec.UseExternalSecrets()
		dcConfig.SuperuserSecretRef.Name = SuperuserSecretName(kc)

		dcKey := types.NamespacedName{Namespace: utils.FirstNonEmptyString(dcConfig.Meta.Namespace, kcKey.Namespace), Name: dcConfig.Meta.Name}
		dcLogger := logger.WithValues("CassandraDatacenter", dcKey, "K8SContext", dcConfig.K8sContext)

		remoteClient, err := r.ClientCache.GetRemoteClient(dcConfig.K8sContext)
		if err != nil {
			dcLogger.Error(err, "Failed to get remote client")
			return nil, err
		}

		if err = cassandra.ReadEncryptionStoresSecrets(ctx, kcKey, dcConfig, remoteClient, dcLogger); err != nil {
			dcLogger.Error(err, "Failed to read encryption secrets")
			return nil, err
		}

		if err := cassandra.HandleEncryptionOptions(dcConfig); err != nil {
			return nil, err
		}

		cassandra.ApplyAuth(dcConfig, kc.Spec.IsAuthEnabled(), kc.Spec.UseExternalSecrets())

		// This is only really required when auth is enabled, but it doesn't hurt to apply system replication on
		// unauthenticated clusters.
		// DSE doesn't support replicating to unexisting datacenters, even through the system property,
		// which is why we're doing this for Cassandra only.
		// We only set this for the first DC. For subsequent DCs, the replication will be altered and a rebuild
		// triggered.
		if kc.Spec.Cassandra.ServerType.IsCassandra() && len(dcConfigs) == 0 {
			cassandra.ApplySystemReplication(dcConfig, systemReplication)
		}

		// Stargate has a bug when backed by Cassandra 4, unless `cassandra.allow_alter_rf_during_range_movement` is
		// set (see https://github.com/stargate/stargate/issues/1274).
		// Set the option preemptively (we don't check `kc.HasStargates()` explicitly, because that causes the operator
		// to restart the whole DC whenever Stargate is added or removed).
		if kc.Spec.Cassandra.ServerType.IsCassandra() && dcConfig.ServerVersion.Major() != 3 {
			cassandra.AllowAlterRfDuringRangeMovement(dcConfig)
		}

		// Inject Reaper settings, unless we just reference an existing Reaper
		if kc.Spec.Reaper != nil && kc.Spec.Reaper.ReaperRef.Name == "" {
			reaper.AddReaperSettingsToDcConfig(kc.Spec.Reaper.DeepCopy(), dcConfig, kc.Spec.IsAuthEnabled())
		}

		// Inject MCAC metrics filters
		if kc.Spec.Cassandra.Telemetry.IsMcacEnabled() {
			telemetry.InjectCassandraTelemetryFilters(kc.Spec.Cassandra.Telemetry, dcConfig)
		}

		// The new metrics endpoint is available since 3.11.13 and 4.0.4.
		// If MCAC is disabled and the new metrics endpoint is not available then we should return an error.
		mergedTelemetrySpec := MergeTelemetrySpecs(kc, dcTemplate)
		if !mergedTelemetrySpec.IsMcacEnabled() && !telemetry.IsNewMetricsEndpointAvailable(dcConfig.ServerVersion.String()) && kc.Spec.Cassandra.ServerType == api.ServerDistributionCassandra {
			return dcConfigs, errors.New("new metrics endpoint is only available since Cassandra 3.11.13/4.0.4, so MCAC cannot be disabled")
		} else {
			logger.Info("new metrics endpoint is available, so MCAC can be disabled", "serverVersion", kc.Spec.Cassandra.ServerVersion)
		}

		// Inject Vector agent
		if err = telemetry.InjectCassandraVectorAgentConfig(kc.Spec.Cassandra.Telemetry, dcConfig, kc.SanitizedName(), dcLogger); err != nil {
			return nil, err
		}

		cassandra.AddNumTokens(dcConfig)
		cassandra.AddStartRpc(dcConfig)
		cassandra.HandleDeprecatedJvmOptions(&dcConfig.CassandraConfig.JvmOptions)
		cassandra.EnableSmartTokenAllocation(dcConfig)

		// If the user has specified external datacenters, we should skip user creation
		// otherwise cass-operator will attempt to create users although the replication of system_auth
		// hasn't been updated to include the new DC.
		if len(kc.Spec.ExternalDatacenters) > 0 {
			dcConfig.Meta.Annotations = map[string]string{
				"cassandra.datastax.com/skip-user-creation": "true",
			}
		}

		if err := cassandra.ValidateDatacenterConfig(dcConfig); err != nil {
			return nil, err
		}

		dcConfigs = append(dcConfigs, dcConfig)
	}

	err := cassandra.ComputeInitialTokens(dcConfigs)
	if err != nil {
		logger.Info("Initial token computation could not be performed or is not required in this cluster", "error", err)
	}

	return dcConfigs, nil
}
