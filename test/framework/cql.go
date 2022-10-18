package framework

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"github.com/datastax/go-cassandra-native-protocol/client"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/square/certigo/jceks"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// These values are hard-coded in our test fixtures - see test/testdata/fixtures/client-encryption-secret.yaml
	secretName = "client-encryption-stores"
	certName   = "cassandra-node_caroot_20220120_134900"
)

// GetCqlClientConnection establishes a connection to the Stargate ingress's CQL port.
func (f *E2eFramework) GetCqlClientConnection(
	ctx context.Context,
	k8sContext, namespace string,
	cqlEndpoint HostAndPort,
	username, password string,
	ssl bool,
) (*client.CqlClientConnection, error) {

	var credentials *client.AuthCredentials
	if username != "" {
		credentials = &client.AuthCredentials{Username: username, Password: password}
	}
	cqlClient := client.NewCqlClient(string(cqlEndpoint), credentials)

	if ssl {
		secret, err := f.getClientEncryptionSecret(ctx, k8sContext, namespace)
		if err != nil {
			return nil, err
		}
		rootCas, err := extractCaCertificates(secret)
		if err != nil {
			return nil, err
		}
		cqlClient.TLSConfig = &tls.Config{
			RootCAs:            rootCas,
			InsecureSkipVerify: true,
		}
	}

	return cqlClient.ConnectAndInit(ctx, primitive.ProtocolVersion4, 1)
}

func (f *E2eFramework) getClientEncryptionSecret(ctx context.Context, k8sContext string, namespace string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	secretkey := ClusterKey{
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      secretName,
		},
		K8sContext: k8sContext,
	}
	if err := f.Get(ctx, secretkey, secret); err != nil {
		return nil, err
	}
	return secret, nil
}

func extractCaCertificates(secret *corev1.Secret) (*x509.CertPool, error) {
	truststoreBytes := secret.Data["truststore"]
	truststorePassword := secret.Data["truststore-password"]
	truststore, err := jceks.LoadFromReader(bytes.NewReader(truststoreBytes), truststorePassword)
	if err != nil {
		return nil, err
	}
	cert, err := truststore.GetCert(certName)
	if err != nil {
		return nil, err
	}
	certPool := x509.NewCertPool()
	certPool.AddCert(cert)
	return certPool, nil
}
