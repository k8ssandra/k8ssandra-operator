package k8ssandra

import (
	"context"
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileContactPointsService ensures that the control plane contains a `*-contact-point-service` Service for each
// DC. This is useful for control plane components that need to connect to DCs.
// To get the node information, we rely on the `*-all-pods-service` that cass-operator creates in the data planes for
// each DC. k8ssandra-operator watches the Endpoints resource associated with this service, and keeps the local service
// in sync with it. Because K8ssandra already requires that pod IP addresses be routable across all Kubernetes clusters,
// we can use those IPs directly.
// See also:
//   - NewDatacenter(), where we annotate the remote service via AdditionalServiceConfig.AllPodsService so that it can be
//     watched.
//   - K8ssandraClusterReconciler.SetupWithManager(), where we set up the watch.
func (r *K8ssandraClusterReconciler) reconcileContactPointsService(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	remoteClient client.Client,
	logger logr.Logger,
) result.ReconcileResult {
	// First, try to fetch the Endpoints of the *-all-pods-service of the CassandraDatacenter, it contains the
	// information we want to duplicate locally.
	remoteEndpoints, err := r.loadAllPodsEndpoints(ctx, dc, remoteClient, logger)
	if err != nil {
		return result.Error(err)
	}
	if remoteEndpoints == nil {
		// Not found. Assume things are not ready yet, another reconcile will be triggered later.
		return result.Continue()
	}

	// Ensure the Service exists
	if err = r.createContactPointsService(ctx, kc, dc, remoteEndpoints, logger); err != nil {
		return result.Error(err)
	}

	// Ensure the Endpoints exists and is up-to-date
	if err = r.reconcileContactPointsServiceEndpoints(ctx, kc, dc, remoteEndpoints, logger); err != nil {
		return result.Error(err)
	}
	return result.Continue()
}

func (r *K8ssandraClusterReconciler) loadAllPodsEndpoints(
	ctx context.Context, dc *cassdcapi.CassandraDatacenter, remoteClient client.Client, logger logr.Logger,
) (*corev1.Endpoints, error) {
	key := types.NamespacedName{
		Namespace: dc.Namespace,
		Name:      dc.GetAllPodsServiceName(),
	}
	endpoints := &corev1.Endpoints{}
	if err := remoteClient.Get(ctx, key, endpoints); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Remote Endpoints not found", "key", key)
			return nil, nil
		}
		logger.Error(err, "Failed to fetch remote Endpoints", "key", key)
		return nil, err
	}
	if len(endpoints.Subsets) == 0 {
		logger.Info("Remote Endpoints found but have no subsets", "key", key)
		return nil, nil
	}
	logger.Info("Remote Endpoints found", "key", key)
	return endpoints, nil
}

func contactPointsServiceKey(kc *api.K8ssandraCluster, dc *cassdcapi.CassandraDatacenter) client.ObjectKey {
	return types.NamespacedName{
		Namespace: kc.Namespace,
		Name:      kc.SanitizedName() + "-" + dc.SanitizedName() + "-contact-points-service",
	}
}

func (r *K8ssandraClusterReconciler) createContactPointsService(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	remoteEndpoints *corev1.Endpoints,
	logger logr.Logger,
) error {
	key := contactPointsServiceKey(kc, dc)
	err := r.Client.Get(ctx, key, &corev1.Service{})
	if err == nil {
		// Service already exists, nothing to do.
		// (note that we don't use a hash annotation here, because the contents are always the same)
		return nil
	}
	if !errors.IsNotFound(err) {
		logger.Error(err, "Failed to fetch Service", "key", key)
		return err
	}

	// Else not found, create it
	logger.Info("Creating Service", "key", key)
	service, err := r.newContactPointsService(key, kc, dc, remoteEndpoints)
	if err != nil {
		logger.Error(err, "Failed to initialize Service", "key", key)
		return err
	}
	if err = r.Client.Create(ctx, service); err != nil {
		logger.Error(err, "Failed to create Service", "key", key)
		return err
	}
	return nil
}

func (r *K8ssandraClusterReconciler) newContactPointsService(
	key client.ObjectKey,
	kc *api.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	remoteEndpoints *corev1.Endpoints,
) (*corev1.Service, error) {
	var ports []corev1.ServicePort
	for _, remotePort := range remoteEndpoints.Subsets[0].Ports {
		ports = append(ports, corev1.ServicePort{
			Name:     remotePort.Name,
			Port:     remotePort.Port,
			Protocol: remotePort.Protocol,
		})
	}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
			Labels: map[string]string{
				api.NameLabel:                      api.NameLabelValue,
				api.PartOfLabel:                    api.PartOfLabelValue,
				api.ComponentLabel:                 api.ComponentLabelValueCassandra,
				api.K8ssandraClusterNameLabel:      kc.Name,
				api.K8ssandraClusterNamespaceLabel: kc.Namespace,
				api.DatacenterLabel:                dc.Name,
			},
		},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeClusterIP,
			Ports: ports,
			// We don't provide a selector since the operator manages the Endpoints directly
		},
	}
	if err := controllerutil.SetControllerReference(kc, service, r.Scheme); err != nil {
		return nil, err
	}
	return service, nil
}

func (r *K8ssandraClusterReconciler) reconcileContactPointsServiceEndpoints(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	remoteEndpoints *corev1.Endpoints,
	logger logr.Logger,
) error {
	key := contactPointsServiceKey(kc, dc)
	desiredEndpoints, err := r.newContactPointsServiceEndpoints(key, kc, dc, remoteEndpoints)
	if err != nil {
		logger.Error(err, "Failed to initialize Endpoints", "key", key)
		return err
	}
	actualEndpoints := &corev1.Endpoints{}
	if err = r.Client.Get(ctx, key, actualEndpoints); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Creating Endpoints", "key", key)
			if err = r.Client.Create(ctx, desiredEndpoints); err != nil {
				logger.Error(err, "Failed to create Endpoints", "key", key)
				return err
			}
			return nil
		}
		logger.Error(err, "Failed to fetch Endpoints", "key", key)
		return err
	}
	if !annotations.CompareHashAnnotations(actualEndpoints, desiredEndpoints) {
		resourceVersion := actualEndpoints.GetResourceVersion()
		desiredEndpoints.DeepCopyInto(actualEndpoints)
		actualEndpoints.SetResourceVersion(resourceVersion)
		logger.Info("Updating Endpoints", "key", key)
		if err := r.Client.Update(ctx, actualEndpoints); err != nil {
			logger.Error(err, "Failed to update Endpoints", "key", key)
			return err
		}
	}
	return nil
}

func (r *K8ssandraClusterReconciler) newContactPointsServiceEndpoints(
	serviceKey client.ObjectKey,
	kc *api.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	remoteEndpoints *corev1.Endpoints,
) (*corev1.Endpoints, error) {
	var subsets []corev1.EndpointSubset
	for _, remoteSubset := range remoteEndpoints.Subsets {
		var addresses []corev1.EndpointAddress
		for _, remoteAddress := range remoteSubset.Addresses {
			// Only preserve the IP. Other fields such as HostName, NodeName, etc. are specific to the remote cluster,
			// so keeping them would be at best misleading (or worse if some component relies on them).
			addresses = append(addresses, corev1.EndpointAddress{IP: remoteAddress.IP})
		}
		subsets = append(subsets, corev1.EndpointSubset{
			Addresses: addresses,
			Ports:     remoteSubset.Ports,
		})
	}
	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: serviceKey.Namespace,
			// The Endpoints resource has the same name as the Service (that's how Kubernetes links them)
			Name: serviceKey.Name,
			Labels: map[string]string{
				api.NameLabel:                      api.NameLabelValue,
				api.PartOfLabel:                    api.PartOfLabelValue,
				api.ComponentLabel:                 api.ComponentLabelValueCassandra,
				api.K8ssandraClusterNameLabel:      kc.Name,
				api.K8ssandraClusterNamespaceLabel: kc.Namespace,
				api.DatacenterLabel:                dc.Name,
			},
		},
		Subsets: subsets,
	}
	annotations.AddHashAnnotation(endpoints)
	return endpoints, nil
}

func (r *K8ssandraClusterReconciler) deleteContactPointsService(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	logger logr.Logger,
) error {
	key := contactPointsServiceKey(kc, dc)
	// We just need to delete the Service, Kubernetes automatically cleans up the Endpoints.
	service := corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name}}
	if err := r.Client.Delete(ctx, &service); err != nil && !errors.IsNotFound(err) {
		logger.Error(err, "Failed to delete Service", "key", key)
		return err
	}
	return nil
}
