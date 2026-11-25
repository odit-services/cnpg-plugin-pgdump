// Package instance implements the CNPG-I services of the sidecar injected in the PostgreSQL pods.
//
// CNPG runs plugin backups from the instance manager, in the background, calling the sidecar over a
// unix socket: unlike the operator hooks, the call has no timeout.
package instance

import (
	"context"
	"encoding/json"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	cnpgbackup "github.com/cloudnative-pg/cnpg-i/pkg/backup"
	cnpgi "github.com/cloudnative-pg/cnpg-i/pkg/identity"
	"github.com/cloudnative-pg/machinery/pkg/log"
	pgbackup "github.com/odit-services/cnpg-plugin-pgdump/internal/backup"
	"github.com/odit-services/cnpg-plugin-pgdump/internal/config"
	pgidentity "github.com/odit-services/cnpg-plugin-pgdump/internal/identity"
)

// Identity is the identity of the sidecar: same metadata as the operator plugin, backup service only.
type Identity struct {
	*pgidentity.Server
}

func NewIdentity(version string) *Identity {
	return &Identity{Server: pgidentity.New(version)}
}

func (i *Identity) GetPluginCapabilities(
	context.Context,
	*cnpgi.GetPluginCapabilitiesRequest,
) (*cnpgi.GetPluginCapabilitiesResponse, error) {
	return &cnpgi.GetPluginCapabilitiesResponse{
		Capabilities: []*cnpgi.PluginCapability{{
			Type: &cnpgi.PluginCapability_Service_{
				Service: &cnpgi.PluginCapability_Service{Type: cnpgi.PluginCapability_Service_TYPE_BACKUP_SERVICE},
			},
		}},
	}, nil
}

// BackupService takes the logical backups.
type BackupService struct {
	cnpgbackup.UnimplementedBackupServer
	AppConfig config.Config
	Runner    *pgbackup.Runner
}

func (s *BackupService) GetCapabilities(
	context.Context,
	*cnpgbackup.BackupCapabilitiesRequest,
) (*cnpgbackup.BackupCapabilitiesResult, error) {
	return &cnpgbackup.BackupCapabilitiesResult{
		Capabilities: []*cnpgbackup.BackupCapability{{
			Type: &cnpgbackup.BackupCapability_Rpc{
				Rpc: &cnpgbackup.BackupCapability_RPC{Type: cnpgbackup.BackupCapability_RPC_TYPE_BACKUP},
			},
		}},
	}, nil
}

func (s *BackupService) Backup(
	ctx context.Context, request *cnpgbackup.BackupRequest,
) (*cnpgbackup.BackupResult, error) {
	logger := log.FromContext(ctx)

	var cluster cnpgv1.Cluster
	if err := json.Unmarshal(request.GetClusterDefinition(), &cluster); err != nil {
		return nil, err
	}
	var backup cnpgv1.Backup
	if err := json.Unmarshal(request.GetBackupDefinition(), &backup); err != nil {
		return nil, err
	}

	backupConfig, err := config.ParseBackupConfig(request.GetParameters(), s.AppConfig)
	if err != nil {
		return nil, err
	}

	result, err := s.Runner.Run(ctx, pgbackup.LocalHost, cluster, backup, backupConfig)
	if err != nil {
		logger.Error(err, "Logical backup failed", "backup", backup.Name)
		return nil, err
	}
	logger.Info("Logical backup completed",
		"backupID", result.BackupID, "size", result.LastBackupSize, "databases", result.DatabasesBackedUp)

	return &cnpgbackup.BackupResult{
		BackupId:   result.BackupID,
		BackupName: backup.Name,
		StartedAt:  result.StartedAt.Unix(),
		StoppedAt:  result.StoppedAt.Unix(),
		Online:     true,
		Metadata:   result.Metadata(),
	}, nil
}
