package k8ssandra

import (
	"context"
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/telemetry/cassandra_agent"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// setupTelemetryCleanup adds an owner reference to ensure that the remote ConfigMap created by
// cassandra_agent.Configurator is correctly cleaned up when the CassandraDatacenter is deleted. We do that in a
// second pass because the CassandraDatacenter did not exist yet at the time the ConfigMap was created.
func (r *K8ssandraClusterReconciler) setupTelemetryCleanup(
	ctx context.Context,
	kc *k8ssandraapi.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	remoteClient client.Client,
	logger logr.Logger,
) result.ReconcileResult {
	configMapKey := client.ObjectKey{
		Namespace: dc.Namespace,
		Name:      cassandra_agent.ConfigMapName(kc.CassClusterName(), dc.DatacenterName()),
	}
	return setDcOwnership(ctx, dc, configMapKey, &corev1.ConfigMap{}, remoteClient, r.Scheme, logger)
}
