package backup

import (
	"strconv"
	"strings"
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
)

// Keys of the backup result metadata, stored by CNPG in the Backup status (pluginMetadata).
const (
	MetadataDatabases = "databases"
	MetadataSize      = "size"
	MetadataObjects   = "objects"
)

type Result struct {
	BackupID          string    `json:"backup_id"`
	StartedAt         time.Time `json:"started_at"`
	StoppedAt         time.Time `json:"stopped_at"`
	LastBackupSize    int64     `json:"last_backup_size"`
	DatabasesBackedUp []string  `json:"databases_backed_up"`
	Objects           []string  `json:"objects"`
	ErrorMessage      string    `json:"error_message,omitempty"`
}

// Metadata returns the result fields that CNPG keeps in the Backup status.
func (r Result) Metadata() map[string]string {
	return map[string]string{
		MetadataDatabases: strings.Join(r.DatabasesBackedUp, ","),
		MetadataSize:      strconv.FormatInt(r.LastBackupSize, 10),
		MetadataObjects:   strings.Join(r.Objects, ","),
	}
}

// ResultFromBackup rebuilds the result of a finished Backup from its status.
func ResultFromBackup(backup cnpgv1.Backup) Result {
	result := Result{
		BackupID:          backup.Status.BackupID,
		DatabasesBackedUp: split(backup.Status.PluginMetadata[MetadataDatabases]),
		Objects:           split(backup.Status.PluginMetadata[MetadataObjects]),
		ErrorMessage:      backup.Status.Error,
	}
	result.LastBackupSize, _ = strconv.ParseInt(backup.Status.PluginMetadata[MetadataSize], 10, 64)
	if backup.Status.StartedAt != nil {
		result.StartedAt = backup.Status.StartedAt.Time
	}
	if backup.Status.StoppedAt != nil {
		result.StoppedAt = backup.Status.StoppedAt.Time
	}
	return result
}

func split(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, ",")
}
