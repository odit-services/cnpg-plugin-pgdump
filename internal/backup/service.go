package backup

import (
	"context"

	cnpgbackup "github.com/cloudnative-pg/cnpg-i/pkg/backup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Service struct {
	cnpgbackup.UnimplementedBackupServer
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) GetCapabilities(context.Context, *cnpgbackup.BackupCapabilitiesRequest) (*cnpgbackup.BackupCapabilitiesResult, error) {
	return &cnpgbackup.BackupCapabilitiesResult{
		Capabilities: []*cnpgbackup.BackupCapability{
			{
				Type: &cnpgbackup.BackupCapability_Rpc{
					Rpc: &cnpgbackup.BackupCapability_RPC{Type: cnpgbackup.BackupCapability_RPC_TYPE_BACKUP},
				},
			},
		},
	}, nil
}

func (s *Service) Backup(context.Context, *cnpgbackup.BackupRequest) (*cnpgbackup.BackupResult, error) {
	return nil, status.Error(codes.FailedPrecondition, "pg_dump logical backups are handled by ReconcilerHooks.Pre")
}
