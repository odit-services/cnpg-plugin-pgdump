package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	cnpgreconciler "github.com/cloudnative-pg/cnpg-i/pkg/reconciler"
	"github.com/cloudnative-pg/machinery/pkg/log"
	pgbackup "github.com/odit-services/cnpg-plugin-pgdump/internal/backup"
	"github.com/odit-services/cnpg-plugin-pgdump/internal/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Server struct {
	cnpgreconciler.UnimplementedReconcilerHooksServer
	appConfig config.Config
	executor  pgbackup.DumpExecutor
	kube      kubernetes.Interface
	store     *pgbackup.Store
}

func New(appConfig config.Config, executor pgbackup.DumpExecutor, kube kubernetes.Interface, store *pgbackup.Store) *Server {
	return &Server{appConfig: appConfig, executor: executor, kube: kube, store: store}
}

func (s *Server) GetCapabilities(context.Context, *cnpgreconciler.ReconcilerHooksCapabilitiesRequest) (*cnpgreconciler.ReconcilerHooksCapabilitiesResult, error) {
	return &cnpgreconciler.ReconcilerHooksCapabilitiesResult{
		ReconcilerCapabilities: []*cnpgreconciler.ReconcilerHooksCapability{
			{Kind: cnpgreconciler.ReconcilerHooksCapability_KIND_BACKUP},
		},
	}, nil
}

func (s *Server) Pre(ctx context.Context, request *cnpgreconciler.ReconcilerHooksRequest) (*cnpgreconciler.ReconcilerHooksResult, error) {
	logger := log.FromContext(ctx)

	var cluster cnpgv1.Cluster
	if err := json.Unmarshal(request.GetClusterDefinition(), &cluster); err != nil {
		logger.Error(err, "Cannot parse cluster definition")
		return continueResult(), nil
	}

	var backup cnpgv1.Backup
	if err := json.Unmarshal(request.GetResourceDefinition(), &backup); err != nil {
		logger.Error(err, "Cannot parse backup definition")
		return continueResult(), nil
	}

	if backup.Spec.Method != cnpgv1.BackupMethodPlugin {
		return continueResult(), nil
	}

	parameters := extractParameters(request.GetResourceDefinition(), backup.Spec.PluginConfiguration)
	backupConfig, err := config.ParseBackupConfig(parameters, s.appConfig)
	if err != nil {
		s.recordError(clusterKey(cluster.Namespace, cluster.Name), backupID(backup, time.Now().UTC()), err)
		logger.Error(err, "Invalid backup configuration")
		return continueResult(), nil
	}

	result, err := s.runBackup(ctx, cluster, backup, backupConfig)
	if err != nil {
		result.ErrorMessage = err.Error()
		s.store.Set(clusterKey(cluster.Namespace, cluster.Name), result)
		logger.Error(err, "Logical backup failed")
		return continueResult(), nil
	}

	s.store.Set(clusterKey(cluster.Namespace, cluster.Name), result)
	logger.Info("Logical backup completed", "backupID", result.BackupID, "size", result.LastBackupSize, "databases", result.DatabasesBackedUp)
	return terminateResult(), nil
}

func (s *Server) Post(context.Context, *cnpgreconciler.ReconcilerHooksRequest) (*cnpgreconciler.ReconcilerHooksResult, error) {
	return continueResult(), nil
}

func (s *Server) runBackup(ctx context.Context, cluster cnpgv1.Cluster, backup cnpgv1.Backup, backupConfig config.BackupConfig) (pgbackup.Result, error) {
	now := time.Now().UTC()
	id := backupID(backup, now)
	result := pgbackup.Result{BackupID: id, StartedAt: now}

	password, user, err := s.readApplicationSecret(ctx, cluster.Namespace, cluster.Name)
	if err != nil {
		return result, err
	}
	if user == "" {
		user = "app"
	}

	conn := pgbackup.Connection{
		Host:     fmt.Sprintf("%s-r.%s.svc", cluster.Name, cluster.Namespace),
		Port:     5432,
		User:     user,
		Password: password,
	}

	databases, err := s.executor.ListDatabases(ctx, conn)
	if err != nil {
		return result, err
	}
	if len(databases) == 0 {
		return result, fmt.Errorf("no dumpable databases found")
	}

	backupConfig, err = s.resolveS3Secrets(ctx, cluster.Namespace, backupConfig)
	if err != nil {
		return result, err
	}

	uploader, err := pgbackup.NewS3Uploader(ctx, backupConfig)
	if err != nil {
		return result, err
	}

	for _, database := range databases {
		localPath, size, err := s.executor.Dump(ctx, conn, database, id, s.appConfig.WorkDir)
		if err != nil {
			return result, err
		}

		key := pgbackup.ObjectKey(backupConfig.Path, cluster.Namespace, cluster.Name, database, id)
		uploadedSize, err := uploader.Upload(ctx, localPath, key)
		_ = os.Remove(localPath)
		if err != nil {
			return result, err
		}
		if uploadedSize == 0 {
			uploadedSize = size
		}

		result.LastBackupSize += uploadedSize
		result.DatabasesBackedUp = append(result.DatabasesBackedUp, database)
		result.Objects = append(result.Objects, "s3://"+path.Join(backupConfig.Bucket, key))

		prefix := pgbackup.DatabasePrefix(backupConfig.Path, cluster.Namespace, cluster.Name, database)
		if err := pgbackup.ApplyRetention(ctx, uploader, prefix, backupConfig.RetentionDays, now); err != nil {
			return result, err
		}
	}

	result.StoppedAt = time.Now().UTC()
	return result, nil
}

func (s *Server) resolveS3Secrets(ctx context.Context, namespace string, backupConfig config.BackupConfig) (config.BackupConfig, error) {
	var err error
	backupConfig.AccessKeyID, err = s.secretValue(ctx, namespace, backupConfig.AccessKeyIDSecret)
	if err != nil {
		return backupConfig, err
	}
	backupConfig.SecretAccessKey, err = s.secretValue(ctx, namespace, backupConfig.SecretAccessKeySecret)
	if err != nil {
		return backupConfig, err
	}
	if value, err := s.secretValue(ctx, namespace, backupConfig.EndpointURLSecret); err != nil {
		return backupConfig, err
	} else if value != "" {
		backupConfig.EndpointURL = value
	}
	if value, err := s.secretValue(ctx, namespace, backupConfig.RegionSecret); err != nil {
		return backupConfig, err
	} else if value != "" {
		backupConfig.Region = value
	}
	return backupConfig, nil
}

func (s *Server) secretValue(ctx context.Context, namespace string, ref config.SecretKeyRef) (string, error) {
	if ref.Name == "" {
		return "", nil
	}
	if s.kube == nil {
		return "", fmt.Errorf("kubernetes client is not configured")
	}
	secret, err := s.kube.CoreV1().Secrets(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	value, ok := secret.Data[ref.Key]
	if !ok {
		return "", fmt.Errorf("secret %s/%s is missing key %s", namespace, ref.Name, ref.Key)
	}
	return string(value), nil
}

func (s *Server) readApplicationSecret(ctx context.Context, namespace, clusterName string) (password, user string, err error) {
	if s.kube == nil {
		return "", "", fmt.Errorf("kubernetes client is not configured")
	}

	secretName := clusterName + cnpgv1.ApplicationUserSecretSuffix
	secret, err := s.kube.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}

	return string(secret.Data[corev1.BasicAuthPasswordKey]), string(secret.Data[corev1.BasicAuthUsernameKey]), nil
}

func (s *Server) recordError(clusterKey, id string, err error) {
	s.store.Set(clusterKey, pgbackup.Result{
		BackupID:     id,
		StartedAt:    time.Now().UTC(),
		StoppedAt:    time.Now().UTC(),
		ErrorMessage: err.Error(),
	})
}

func clusterKey(namespace, name string) string {
	return namespace + "/" + name
}

func extractParameters(raw []byte, typed *cnpgv1.BackupPluginConfiguration) map[string]string {
	if typed != nil && typed.Name == config.PluginName {
		return typed.Parameters
	}

	var fallback struct {
		Spec struct {
			PluginConfiguration *cnpgv1.BackupPluginConfiguration `json:"pluginConfiguration"`
			Plugin              *cnpgv1.BackupPluginConfiguration `json:"plugin"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(raw, &fallback); err != nil {
		return nil
	}

	for _, candidate := range []*cnpgv1.BackupPluginConfiguration{fallback.Spec.PluginConfiguration, fallback.Spec.Plugin} {
		if candidate != nil && candidate.Name == config.PluginName {
			return candidate.Parameters
		}
	}
	return nil
}

func backupID(backup cnpgv1.Backup, now time.Time) string {
	stamp := now.Format("20060102T150405Z")
	if !backup.CreationTimestamp.IsZero() {
		stamp = backup.CreationTimestamp.UTC().Format("20060102T150405Z")
	}
	name := strings.TrimSpace(backup.Name)
	if name == "" {
		name = "manual"
	}
	return name + "-" + stamp
}

func continueResult() *cnpgreconciler.ReconcilerHooksResult {
	return &cnpgreconciler.ReconcilerHooksResult{Behavior: cnpgreconciler.ReconcilerHooksResult_BEHAVIOR_CONTINUE}
}

func terminateResult() *cnpgreconciler.ReconcilerHooksResult {
	return &cnpgreconciler.ReconcilerHooksResult{Behavior: cnpgreconciler.ReconcilerHooksResult_BEHAVIOR_TERMINATE}
}
