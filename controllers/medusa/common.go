package medusa

import (
	"context"
	"fmt"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	medusaapi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This file has some helper functions for all the controllers to create Medusa gRPC clients.

func newClient(ctx context.Context, c client.Client, cassdc *cassdcapi.CassandraDatacenter, pod *corev1.Pod, clientFactory medusa.ClientFactory) (medusa.Client, error) {
	medusaConfig, err := medusaConfiguration(ctx, c, cassdc)
	if err != nil {
		return nil, err
	}

	grpcPort := medusa.DefaultMedusaPort
	if medusaConfig.ServiceProperties.GrpcPort > 0 {
		grpcPort = medusaConfig.ServiceProperties.GrpcPort
	}

	address := fmt.Sprintf("%s:%d", pod.Status.PodIP, grpcPort)

	if medusaConfig.ServiceProperties.Encryption != nil && medusaConfig.ServiceProperties.Encryption.ClientSecretName != "" {
		secretKey := types.NamespacedName{
			Namespace: cassdc.Namespace,
			Name:      medusaConfig.ServiceProperties.Encryption.ClientSecretName,
		}
		secret := &corev1.Secret{}
		if err := c.Get(ctx, secretKey, secret); err != nil {
			return nil, fmt.Errorf("failed to get client secret %s: %w", secretKey, err)
		}

		return clientFactory.NewClientWithTLS(address, secret)
	}

	return clientFactory.NewClient(address)
}

func medusaConfiguration(ctx context.Context, c client.Client, cassdc *cassdcapi.CassandraDatacenter) (*medusaapi.MedusaClusterTemplate, error) {
	var k8cOwnerRef *types.NamespacedName
	for _, ownerRef := range cassdc.OwnerReferences {
		if ownerRef.Kind == "K8ssandraCluster" && ownerRef.APIVersion == k8ssandraapi.GroupVersion.String() {
			k8cOwnerRef = &types.NamespacedName{
				Namespace: cassdc.Namespace,
				Name:      ownerRef.Name,
			}
			break
		}
	}

	// This shouldn't happen unless user uses k8ssandra-operator to manage Medusa installation of a non-managed CassandraDatacenter. We don't support such scenario at this point.
	if k8cOwnerRef == nil {
		return nil, fmt.Errorf("CassandraDatacenter %s/%s does not have a K8ssandraCluster owner reference", cassdc.Namespace, cassdc.Name)
	}

	k8c := &k8ssandraapi.K8ssandraCluster{}
	if err := c.Get(ctx, *k8cOwnerRef, k8c); err != nil {
		return nil, fmt.Errorf("failed to get K8ssandraCluster %s: %w", k8cOwnerRef, err)
	}

	if k8c.Spec.Medusa == nil {
		return nil, fmt.Errorf("K8ssandraCluster %s does not have Medusa configured", k8cOwnerRef)
	}

	return k8c.Spec.Medusa, nil
}
