package k8ssandra

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
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
	remoteEndpoints, err := r.loadAllPodsEndpoints(ctx, dc, remoteClient)
	if err != nil {
		return result.Error(err)
	}
	if len(remoteEndpoints) < 1 {
		// Not found. Assume things are not ready yet, another reconcile will be triggered later.
		return result.Continue()
	}

	ports := make([]discoveryv1.EndpointPort, 0, 4)
	ports = append(ports, remoteEndpoints[0].Ports...)

	// Ensure the Service exists
	if err = r.createContactPointsService(ctx, logger, kc, dc, ports); err != nil {
		return result.Error(err)
	}

	// Ensure the Endpoints exists and is up-to-date
	if err = r.reconcileContactPointsServiceEndpoints(ctx, logger, kc, dc, remoteEndpoints); err != nil {
		return result.Error(err)
	}
	return result.Continue()
}

func (r *K8ssandraClusterReconciler) loadAllPodsEndpoints(
	ctx context.Context, dc *cassdcapi.CassandraDatacenter, remoteClient client.Client,
) ([]discoveryv1.EndpointSlice, error) {
	endpoints := &discoveryv1.EndpointSliceList{}
	searchLabels := dc.GetDatacenterLabels()
	searchLabels["kubernetes.io/service-name"] = dc.GetAllPodsServiceName()
	if err := remoteClient.List(ctx, endpoints, client.MatchingLabels(searchLabels)); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return endpoints.Items, nil
}

func contactPointsServiceKey(kc *api.K8ssandraCluster, dc *cassdcapi.CassandraDatacenter) client.ObjectKey {
	return types.NamespacedName{
		Namespace: kc.Namespace,
		Name:      kc.SanitizedName() + "-" + dc.LabelResourceName() + "-contact-points-service",
	}
}

func (r *K8ssandraClusterReconciler) createContactPointsService(
	ctx context.Context,
	logger logr.Logger,
	kc *api.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	ports []discoveryv1.EndpointPort,
) error {
	actualService := &corev1.Service{}
	key := contactPointsServiceKey(kc, dc)
	create := false
	if err := r.Client.Get(ctx, key, actualService); client.IgnoreNotFound(err) != nil {
		logger.Error(err, "Failed to fetch Service", "key", key)
		return err
	} else if errors.IsNotFound(err) {
		create = true
	}

	var servicePorts []corev1.ServicePort
	for _, remotePort := range ports {
		servicePorts = append(servicePorts, corev1.ServicePort{
			Name:     *remotePort.Name,
			Port:     *remotePort.Port,
			Protocol: *remotePort.Protocol,
		})
	}

	desiredService, err := r.newContactPointsService(key, kc, dc, servicePorts)
	if err != nil {
		logger.Error(err, "Failed to initialize Service", "key", key)
		return err
	}

	if create {
		if err = r.Client.Create(ctx, desiredService); err != nil {
			logger.Error(err, "Failed to create Service", "key", key)
			return err
		}
	} else if !annotations.CompareHashAnnotations(actualService, desiredService) {
		resourceVersion := actualService.GetResourceVersion()
		desiredService.DeepCopyInto(actualService)
		actualService.SetResourceVersion(resourceVersion)
		if err = r.Client.Update(ctx, actualService); err != nil {
			logger.Error(err, "Failed to update contact-points Service")
			return err
		}
	}

	return nil
}

func (r *K8ssandraClusterReconciler) newContactPointsService(
	key client.ObjectKey,
	kc *api.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	ports []corev1.ServicePort,
) (*corev1.Service, error) {
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
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: corev1.ClusterIPNone,
			Ports:     ports,
			// We don't provide a selector since the operator manages the Endpoints directly
		},
	}
	if err := controllerutil.SetControllerReference(kc, service, r.Scheme); err != nil {
		return nil, err
	}
	labels.AddCommonLabels(service, kc)
	annotations.AddCommonAnnotations(service, kc)
	annotations.AddHashAnnotation(service)

	return service, nil
}

func (r *K8ssandraClusterReconciler) reconcileContactPointsServiceEndpoints(
	ctx context.Context,
	logger logr.Logger,
	kc *api.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	remoteEndpoints []discoveryv1.EndpointSlice,
) error {
	key := contactPointsServiceKey(kc, dc)
	endpoints := []*discoveryv1.EndpointSlice{}

	for _, remoteEndpoint := range remoteEndpoints {
		name := fmt.Sprintf("%s-%s", key.Name, strings.ToLower(string(remoteEndpoint.AddressType)))
		addresses := make([]string, 0, len(remoteEndpoint.Endpoints))
		for _, remoteAddress := range remoteEndpoint.Endpoints {
			addresses = append(addresses, remoteAddress.Addresses...)
		}
		endpoint := &discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: kc.Namespace,
				Name:      name,
				Labels: map[string]string{
					api.NameLabel:                      api.NameLabelValue,
					api.PartOfLabel:                    api.PartOfLabelValue,
					api.ComponentLabel:                 api.ComponentLabelValueCassandra,
					api.K8ssandraClusterNameLabel:      kc.Name,
					api.K8ssandraClusterNamespaceLabel: kc.Namespace,
					api.DatacenterLabel:                dc.Name,
					discoveryv1.LabelManagedBy:         api.NameLabelValue,
					discoveryv1.LabelServiceName:       key.Name,
				},
			},
			AddressType: remoteEndpoint.AddressType,
			Endpoints: []discoveryv1.Endpoint{
				{
					Addresses: addresses,
				},
			},
			Ports: remoteEndpoint.Ports,
		}
		endpoints = append(endpoints, endpoint)
	}

	if err := reconciliation.ReconcileEndpointSlices(ctx, r.Client, logger, endpoints); err != nil {
		return err
	}

	// Cleanup old endpoints object, it's not used anymore
	oldEndpoints := &corev1.Endpoints{} //nolint:staticcheck // Endpoints is deprecated, but we still need to clean it up.
	if err := r.Client.Get(ctx, key, oldEndpoints); client.IgnoreNotFound(err) != nil {
		return err
	}

	if err := r.Client.Delete(ctx, oldEndpoints); client.IgnoreNotFound(err) != nil {
		return err
	}

	return nil
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
