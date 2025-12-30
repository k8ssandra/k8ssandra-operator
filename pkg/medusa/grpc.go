package medusa

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type defaultClient struct {
	connection *grpc.ClientConn
	grpcClient MedusaClient
}

type ClientFactory interface {
	NewClient(address string) (Client, error)
	NewClientWithTLS(address string, secret *corev1.Secret) (Client, error)
}

type DefaultFactory struct {
	K8sClient client.Client
}

func (f *DefaultFactory) NewClient(address string) (Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.WaitForReady(false), grpc.MaxCallRecvMsgSize(1024*1024*512)))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to %s: %s", address, err)
	}

	return &defaultClient{connection: conn, grpcClient: NewMedusaClient(conn)}, nil
}

func (f *DefaultFactory) NewClientWithTLS(address string, secret *corev1.Secret) (Client, error) {
	tlsCreds, err := f.transportCredentials(secret)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport credentials from secret %s: %s", secret.Name, err)
	}

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(tlsCreds), grpc.WithDefaultCallOptions(grpc.WaitForReady(false), grpc.MaxCallRecvMsgSize(1024*1024*512)))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to %s: %s", address, err)
	}

	return &defaultClient{connection: conn, grpcClient: NewMedusaClient(conn)}, nil
}

func (f *DefaultFactory) transportCredentials(secret *corev1.Secret) (credentials.TransportCredentials, error) {
	// Sadly the Sonarcloud does not understand how tls.Config work, so we have to use default settings here.
	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(secret.Data["ca.crt"]); !ok {
		return nil, fmt.Errorf("no certificates found in %s when parsing 'ca.crt' value: %v",
			secret.Name,
			secret.Data["ca.crt"])
	}

	cert, err := tls.X509KeyPair(secret.Data["tls.crt"], secret.Data["tls.key"])
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates:          []tls.Certificate{cert},
		InsecureSkipVerify:    true,
		VerifyPeerCertificate: buildVerifyPeerCertificateNoHostCheck(caCertPool),
		RootCAs:               caCertPool,
	}

	return credentials.NewTLS(tlsConfig), nil
}

// Below implementation modified from:
//
// https://go-review.googlesource.com/c/go/+/193620/5/src/crypto/tls/example_test.go#210
func buildVerifyPeerCertificateNoHostCheck(rootCAs *x509.CertPool) func([][]byte, [][]*x509.Certificate) error {
	f := func(certificates [][]byte, _ [][]*x509.Certificate) error {
		certs := make([]*x509.Certificate, len(certificates))
		for i, asn1Data := range certificates {
			cert, err := x509.ParseCertificate(asn1Data)
			if err != nil {
				return err
			}
			certs[i] = cert
		}

		_, err := verifyPeerCertificateNoHostCheck(certs, rootCAs)
		return err
	}
	return f
}

func verifyPeerCertificateNoHostCheck(certificates []*x509.Certificate, rootCAs *x509.CertPool) ([][]*x509.Certificate, error) {
	opts := x509.VerifyOptions{
		Roots: rootCAs,
		// Setting the DNSName to the empty string will cause
		// Certificate.Verify() to skip hostname checking
		DNSName:       "",
		Intermediates: x509.NewCertPool(),
	}
	for _, cert := range certificates[1:] {
		opts.Intermediates.AddCert(cert)
	}
	chains, err := certificates[0].Verify(opts)
	return chains, err
}

type Client interface {
	Close() error

	CreateBackup(ctx context.Context, name string, backupType string) (*BackupResponse, error)

	GetBackups(ctx context.Context) ([]*BackupSummary, error)

	PurgeBackups(ctx context.Context) (*PurgeBackupsResponse, error)

	PrepareRestore(ctx context.Context, datacenter, backupName, restoreKey string) (*PrepareRestoreResponse, error)

	BackupStatus(ctx context.Context, backupName string) (*BackupStatusResponse, error)
}

func (c *defaultClient) Close() error {
	return c.connection.Close()
}

func (c *defaultClient) CreateBackup(ctx context.Context, name string, backupType string) (*BackupResponse, error) {
	backupMode := BackupRequest_DIFFERENTIAL
	if backupType == "full" {
		backupMode = BackupRequest_FULL
	}

	request := BackupRequest{
		Name: name,
		Mode: backupMode,
	}

	resp, err := c.grpcClient.AsyncBackup(ctx, &request)
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (c *defaultClient) GetBackups(ctx context.Context) ([]*BackupSummary, error) {
	response, err := c.grpcClient.GetBackups(ctx, &GetBackupsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to get backups: %s", err)
	}
	return response.Backups, nil
}

func (c *defaultClient) DeleteBackup(ctx context.Context, name string) error {
	request := DeleteBackupRequest{Name: name}
	_, err := c.grpcClient.DeleteBackup(context.Background(), &request)
	return err
}

func (c *defaultClient) PurgeBackups(ctx context.Context) (*PurgeBackupsResponse, error) {
	request := PurgeBackupsRequest{}
	response, err := c.grpcClient.PurgeBackups(ctx, &request)

	return response, err
}

func (c *defaultClient) PrepareRestore(ctx context.Context, datacenter, backupName, restoreKey string) (*PrepareRestoreResponse, error) {
	request := PrepareRestoreRequest{
		Datacenter: datacenter,
		BackupName: backupName,
		RestoreKey: restoreKey,
	}
	response, err := c.grpcClient.PrepareRestore(ctx, &request)

	return response, err
}

func (c *defaultClient) BackupStatus(ctx context.Context, backupName string) (*BackupStatusResponse, error) {
	request := BackupStatusRequest{
		BackupName: backupName,
	}
	return c.grpcClient.BackupStatus(ctx, &request)
}
