package medusa

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
)

type defaultClient struct {
	connection *grpc.ClientConn
	grpcClient MedusaClient
}

type ClientFactory interface {
	NewClient(address string) (Client, error)
}

type DefaultFactory struct {
}

func (f *DefaultFactory) NewClient(address string) (Client, error) {
	conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithDefaultCallOptions(grpc.WaitForReady(false)))

	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to %s: %s", address, err)
	}

	return &defaultClient{connection: conn, grpcClient: NewMedusaClient(conn)}, nil
}

type Client interface {
	Close() error

	CreateBackup(ctx context.Context, name string, backupType string) error

	GetBackups(ctx context.Context) ([]*BackupSummary, error)

	PurgeBackups(ctx context.Context) (*PurgeBackupsResponse, error)

	PrepareRestore(ctx context.Context, datacenter, backupName, restoreKey string) (*PrepareRestoreResponse, error)
}

func (c *defaultClient) Close() error {
	return c.connection.Close()
}

func (c *defaultClient) CreateBackup(ctx context.Context, name string, backupType string) error {
	backupMode := BackupRequest_DIFFERENTIAL
	if backupType == "full" {
		backupMode = BackupRequest_FULL
	}

	request := BackupRequest{
		Name: name,
		Mode: backupMode,
	}
	_, err := c.grpcClient.Backup(ctx, &request)

	return err
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
