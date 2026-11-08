package medusa

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k8ssandra/k8ssandra-operator/pkg/tlsconfig"
)

type defaultClient struct {
	connection *grpc.ClientConn
	grpcClient MedusaClient
}

type ClientFactory interface {
	NewClient(address string) (Client, error)
	NewClientWithTLS(address string, secret *corev1.Secret, serverName string) (Client, error)
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

func (f *DefaultFactory) NewClientWithTLS(address string, secret *corev1.Secret, serverName string) (Client, error) {
	tlsCreds, err := f.transportCredentials(secret, serverName)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport credentials from secret %s: %s", secret.Name, err)
	}

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(tlsCreds), grpc.WithDefaultCallOptions(grpc.WaitForReady(false), grpc.MaxCallRecvMsgSize(1024*1024*512)))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to %s: %s", address, err)
	}

	return &defaultClient{connection: conn, grpcClient: NewMedusaClient(conn)}, nil
}

func (f *DefaultFactory) transportCredentials(secret *corev1.Secret, serverName string) (credentials.TransportCredentials, error) {
	tlsConfig, err := tlsconfig.NewMutualTLSClientConfig(
		secret.Data["ca.crt"],
		secret.Data["tls.crt"],
		secret.Data["tls.key"],
		serverName,
	)
	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(tlsConfig), nil
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
