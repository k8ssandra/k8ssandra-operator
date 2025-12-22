package medusa

import (
	"context"
	"fmt"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This file has some helper functions for all the controllers to create Medusa gRPC clients.

func newClient(ctx context.Context, c client.Client, cassdc *cassdcapi.CassandraDatacenter, pod *corev1.Pod, clientFactory medusa.ClientFactory) (medusa.Client, error) {
	clientSecretName := medusaClientSecretName(cassdc)

	grpcPort := medusa.DefaultMedusaPort
	explicitPort, found := cassandra.FindContainerPort(pod, "medusa", "grpc")
	if found {
		grpcPort = explicitPort
	}

	address := fmt.Sprintf("%s:%d", pod.Status.PodIP, grpcPort)

	if clientSecretName != "" {
		secretKey := types.NamespacedName{
			Namespace: cassdc.Namespace,
			Name:      clientSecretName,
		}
		secret := &corev1.Secret{}
		if err := c.Get(ctx, secretKey, secret); err != nil {
			return nil, fmt.Errorf("failed to get client secret %s: %w", secretKey, err)
		}

		return clientFactory.NewClientWithTLS(address, secret)
	}

	return clientFactory.NewClient(address)
}

func medusaClientSecretName(cassdc *cassdcapi.CassandraDatacenter) string {
	if metav1.HasAnnotation(cassdc.ObjectMeta, medusa.MedusaClientSecretNameAnnotation) {
		return cassdc.ObjectMeta.Annotations[medusa.MedusaClientSecretNameAnnotation]
	}
	return ""
}
