package medusa

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// DefaultCallTimeout bounds the short Medusa calls (AsyncBackup, BackupStatus). They only start or look up
	// an operation and should return quickly.
	DefaultCallTimeout = 45 * time.Second
	// DefaultListTimeout bounds calls that read the backup index from the storage backend (GetBackups,
	// PrepareRestore).
	DefaultListTimeout = 2 * time.Minute
	// DefaultPurgeTimeout bounds PurgeBackups, which keeps the call open for the whole purge. A purge that
	// hits it fails, and the next scheduled purge continues with what is left.
	DefaultPurgeTimeout = 24 * time.Hour

	// keepaliveTime is how long the connection may be idle before the client pings Medusa. The Python gRPC
	// server sends GOAWAY (too_many_pings) to clients pinging more often than every 5 minutes while no data
	// is flowing, which is the case during a long purge, so this must stay above 5 minutes.
	keepaliveTime    = 6 * time.Minute
	keepaliveTimeout = 20 * time.Second
)

// dialOptions are shared by the plaintext and TLS clients. Keepalive detects a Medusa container that went away
// while a call was in flight, instead of leaving the call waiting on the TCP stack.
func dialOptions(creds credentials.TransportCredentials) []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(false), grpc.MaxCallRecvMsgSize(1024*1024*512)),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                keepaliveTime,
			Timeout:             keepaliveTimeout,
			PermitWithoutStream: false,
		}),
	}
}

// withDefaultTimeout bounds ctx by timeout, unless the caller already set a deadline.
func withDefaultTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

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
	conn, err := grpc.NewClient(address, dialOptions(insecure.NewCredentials())...)
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

	conn, err := grpc.NewClient(address, dialOptions(tlsCreds)...)
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

// Client calls the Medusa gRPC API. Unless the caller's context already has a deadline, each call is bounded by
// DefaultCallTimeout, DefaultListTimeout or DefaultPurgeTimeout, see the method comments.
type Client interface {
	Close() error

	// CreateBackup starts an asynchronous backup. Default timeout: DefaultCallTimeout.
	CreateBackup(ctx context.Context, name string, backupType string) (*BackupResponse, error)

	// GetBackups default timeout: DefaultListTimeout.
	GetBackups(ctx context.Context) ([]*BackupSummary, error)

	// PurgeBackups runs the whole purge in one call. Default timeout: DefaultPurgeTimeout.
	PurgeBackups(ctx context.Context) (*PurgeBackupsResponse, error)

	// PrepareRestore default timeout: DefaultListTimeout.
	PrepareRestore(ctx context.Context, datacenter, backupName, restoreKey string) (*PrepareRestoreResponse, error)

	// BackupStatus default timeout: DefaultCallTimeout.
	BackupStatus(ctx context.Context, backupName string) (*BackupStatusResponse, error)

	// PreparePitrRestore asks Medusa to check that a point-in-time restore to targetTimestamp is possible
	// for the given Cassandra datacenter. The caller owns the deadline and the error handling.
	PreparePitrRestore(ctx context.Context, datacenter, targetTimestamp string) (*PreparePitrRestoreResponse, error)
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

	ctx, cancel := withDefaultTimeout(ctx, DefaultCallTimeout)
	defer cancel()
	resp, err := c.grpcClient.AsyncBackup(ctx, &request)
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (c *defaultClient) GetBackups(ctx context.Context) ([]*BackupSummary, error) {
	ctx, cancel := withDefaultTimeout(ctx, DefaultListTimeout)
	defer cancel()
	response, err := c.grpcClient.GetBackups(ctx, &GetBackupsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to get backups: %s", err)
	}
	return response.Backups, nil
}

func (c *defaultClient) DeleteBackup(ctx context.Context, name string) error {
	request := DeleteBackupRequest{Name: name}
	ctx, cancel := withDefaultTimeout(ctx, DefaultListTimeout)
	defer cancel()
	_, err := c.grpcClient.DeleteBackup(ctx, &request)
	return err
}

func (c *defaultClient) PurgeBackups(ctx context.Context) (*PurgeBackupsResponse, error) {
	ctx, cancel := withDefaultTimeout(ctx, DefaultPurgeTimeout)
	defer cancel()
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
	ctx, cancel := withDefaultTimeout(ctx, DefaultListTimeout)
	defer cancel()
	response, err := c.grpcClient.PrepareRestore(ctx, &request)

	return response, err
}

func (c *defaultClient) PreparePitrRestore(ctx context.Context, datacenter, targetTimestamp string) (*PreparePitrRestoreResponse, error) {
	return c.grpcClient.PreparePitrRestore(ctx, &PreparePitrRestoreRequest{
		Datacenter:      datacenter,
		TargetTimestamp: targetTimestamp,
	})
}

func (c *defaultClient) BackupStatus(ctx context.Context, backupName string) (*BackupStatusResponse, error) {
	request := BackupStatusRequest{
		BackupName: backupName,
	}
	ctx, cancel := withDefaultTimeout(ctx, DefaultCallTimeout)
	defer cancel()
	return c.grpcClient.BackupStatus(ctx, &request)
}
