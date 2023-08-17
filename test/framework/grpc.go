package framework

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/stargate/stargate-grpc-go-client/stargate/pkg/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (f *E2eFramework) GetStargateGrpcConnection(
	ctx context.Context,
	k8sContext, namespace, clusterName, username, password string,
	grpcEndpoint, authEndpoint HostAndPort,
) (*grpc.ClientConn, error) {
	grpcCert, err := f.getGrpcCert(ctx, k8sContext, namespace, clusterName)
	if err != nil {
		return nil, err
	}
	certPool := x509.NewCertPool()
	certPool.AddCert(grpcCert)
	config := &tls.Config{}
	config.RootCAs = certPool

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return grpc.DialContext(ctx, string(grpcEndpoint), grpc.WithTransportCredentials(credentials.NewTLS(config)),
		grpc.WithBlock(),
		grpc.WithPerRPCCredentials(
			auth.NewTableBasedTokenProvider(
				fmt.Sprintf("http://%v/v1/auth", authEndpoint), username, password,
			),
		),
	)
}

func (f *E2eFramework) getGrpcCert(ctx context.Context, k8sContext, namespace, clusterName string) (*x509.Certificate, error) {
	secret := &corev1.Secret{}
	secretkey := ClusterKey{
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      fmt.Sprintf("%s-stargate-service-grpc-tls-secret", clusterName),
		},
		K8sContext: k8sContext,
	}
	if err := f.Get(ctx, secretkey, secret); err != nil {
		return nil, err
	}
	bytes := secret.Data["ca.crt"]

	block, _ := pem.Decode(bytes)

	return x509.ParseCertificate(block.Bytes)
}
