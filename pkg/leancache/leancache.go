// Package leancache centralizes the cache-scoping configuration that keeps the
// operator's memory bounded regardless of cluster size. It is applied to the local
// manager (main.go) and to every remote cluster.Cluster (clientconfig controller), so
// both caches follow the same rules.
//
// The model:
//   - High-cardinality / foreign types (see DisableFor) are never read through the
//     cache: reads go live to the API server. All such reads in this operator are
//     namespace+label scoped and happen only inside low-frequency reconciles.
//   - Watches on those types use metadata-only projection (builder.OnlyMetadata /
//     PartialObjectMetadata sources), so their informers hold object metadata, not
//     payloads. Every relevant handler map fn only inspects metadata.
package leancache

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	medusav1alpha1 "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
)

// DisableFor lists every type the operator reads but must not cache. Secrets and
// ConfigMaps cannot be label-scoped instead: ReplicatedSecret matches secrets by
// arbitrary user-supplied selectors, ClientConfig reads user kubeconfig secrets and
// Stargate reads the user CassandraConfigMapRef — none of them carry operator labels.
// The remaining entries are cold read paths whose informers would otherwise be
// created lazily and cluster-wide (Pods via findSeeds/medusa/management facade,
// CronJobs via medusa purge, ServiceMonitors via telemetry, MedusaBackup which has no
// watch, and the objects behind the metadata-only Owns/Watches).
func DisableFor() []client.Object {
	return []client.Object{
		&corev1.Secret{},
		&corev1.ConfigMap{},
		&corev1.Pod{},
		&corev1.Service{},
		&corev1.Endpoints{},
		&discoveryv1.EndpointSlice{},
		&appsv1.Deployment{},
		&appsv1.StatefulSet{},
		&batchv1.CronJob{},
		&promapi.ServiceMonitor{},
		&medusav1alpha1.MedusaBackup{},
	}
}

// StripHeavyMetadata extends cache.TransformStripManagedFields by also dropping the
// kubectl last-applied-configuration annotation from every cached object: for a
// kubectl-applied object that annotation embeds the full object (including Secret
// data), so it dominates cached-object size — and even survives metadata-only
// projection, since annotations are part of PartialObjectMetadata. Nothing in the
// operator reads it.
func StripHeavyMetadata() toolscache.TransformFunc {
	stripManagedFields := cache.TransformStripManagedFields()
	return func(obj interface{}) (interface{}, error) {
		obj, err := stripManagedFields(obj)
		if err != nil {
			return obj, err
		}
		if m, ok := obj.(metav1.Object); ok {
			annotations := m.GetAnnotations()
			if _, found := annotations[corev1.LastAppliedConfigAnnotation]; found {
				delete(annotations, corev1.LastAppliedConfigAnnotation)
				m.SetAnnotations(annotations)
			}
		}
		return obj, nil
	}
}
