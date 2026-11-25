package backup

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/odit-services/cnpg-plugin-pgdump/internal/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// LocalHost is the address of the PostgreSQL instance seen from the instance sidecar.
const LocalHost = "localhost"

// Runner dumps every database of a Cluster and uploads the dumps to S3.
type Runner struct {
	AppConfig config.Config
	Executor  DumpExecutor
	Kube      kubernetes.Interface
}

// Run takes the logical backup of the PostgreSQL instance reachable at host.
func (r *Runner) Run(
	ctx context.Context, host string, cluster cnpgv1.Cluster, backup cnpgv1.Backup, backupConfig config.BackupConfig,
) (Result, error) {
	now := time.Now().UTC()
	id := BackupID(backup, now)
	result := Result{BackupID: id, StartedAt: now}
	backupUser := backupConfig.BackupUser
	if backupUser == "" {
		backupUser = config.DefaultBackupUser
	}

	password, user, err := r.readBackupUserSecret(ctx, cluster.Namespace, cluster.Name, backupUser)
	if err != nil {
		return result, err
	}
	if user == "" {
		user = backupUser
	}

	conn := Connection{
		Host:     host,
		Port:     5432,
		User:     user,
		Password: password,
	}
	conn.Major, err = r.Executor.ServerMajor(ctx, conn)
	if err != nil {
		return result, err
	}

	databases, err := r.Executor.ListDatabases(ctx, conn, backupConfig.SkipInaccessible)
	if err != nil {
		return result, err
	}
	if len(databases) == 0 {
		return result, fmt.Errorf("no dumpable databases found")
	}

	backupConfig, err = r.resolveS3Secrets(ctx, cluster.Namespace, backupConfig)
	if err != nil {
		return result, err
	}

	uploader, err := NewS3Uploader(ctx, backupConfig)
	if err != nil {
		return result, err
	}

	for _, database := range databases {
		localPath, size, err := r.Executor.Dump(ctx, conn, database, id, r.AppConfig.WorkDir)
		if err != nil {
			return result, err
		}

		key := ObjectKey(backupConfig.Path, backupConfig.ObjectKeyTemplate,
			cluster.Namespace, cluster.Name, database, id)
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

		prefix := DatabasePrefix(backupConfig.Path, backupConfig.ObjectKeyTemplate,
			cluster.Namespace, cluster.Name, database)
		if err := ApplyRetention(ctx, uploader, prefix, backupConfig.RetentionDays, now); err != nil {
			return result, err
		}
	}

	result.StoppedAt = time.Now().UTC()
	return result, nil
}

// SecretNames returns the secrets a backup reads: the backup user secret and the S3 secrets.
func SecretNames(clusterName string, backupConfig config.BackupConfig) []string {
	names := []string{BackupUserSecretName(clusterName, backupConfig.BackupUser)}
	for _, ref := range []config.SecretKeyRef{
		backupConfig.AccessKeyIDSecret,
		backupConfig.SecretAccessKeySecret,
		backupConfig.EndpointURLSecret,
		backupConfig.RegionSecret,
		backupConfig.BucketSecret,
	} {
		if ref.Name != "" {
			names = append(names, ref.Name)
		}
	}
	return names
}

func (r *Runner) resolveS3Secrets(
	ctx context.Context, namespace string, backupConfig config.BackupConfig,
) (config.BackupConfig, error) {
	var err error
	backupConfig.AccessKeyID, err = r.secretValue(ctx, namespace, backupConfig.AccessKeyIDSecret)
	if err != nil {
		return backupConfig, err
	}
	backupConfig.SecretAccessKey, err = r.secretValue(ctx, namespace, backupConfig.SecretAccessKeySecret)
	if err != nil {
		return backupConfig, err
	}
	if value, err := r.secretValue(ctx, namespace, backupConfig.EndpointURLSecret); err != nil {
		return backupConfig, err
	} else if value != "" {
		backupConfig.EndpointURL = value
	}
	if value, err := r.secretValue(ctx, namespace, backupConfig.RegionSecret); err != nil {
		return backupConfig, err
	} else if value != "" {
		backupConfig.Region = value
	}
	if value, err := r.secretValue(ctx, namespace, backupConfig.BucketSecret); err != nil {
		return backupConfig, err
	} else if value != "" {
		backupConfig.Bucket = value
	}
	return backupConfig, nil
}

func (r *Runner) secretValue(ctx context.Context, namespace string, ref config.SecretKeyRef) (string, error) {
	if ref.Name == "" {
		return "", nil
	}
	if r.Kube == nil {
		return "", fmt.Errorf("kubernetes client is not configured")
	}
	secret, err := r.Kube.CoreV1().Secrets(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	value, ok := secret.Data[ref.Key]
	if !ok {
		return "", fmt.Errorf("secret %s/%s is missing key %s", namespace, ref.Name, ref.Key)
	}
	return string(value), nil
}

func (r *Runner) readBackupUserSecret(
	ctx context.Context, namespace, clusterName, backupUser string,
) (password, user string, err error) {
	if r.Kube == nil {
		return "", "", fmt.Errorf("kubernetes client is not configured")
	}

	secretName := BackupUserSecretName(clusterName, backupUser)
	secret, err := r.Kube.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}

	return string(secret.Data[corev1.BasicAuthPasswordKey]), string(secret.Data[corev1.BasicAuthUsernameKey]), nil
}

// BackupUserSecretName returns the secret holding the credentials of the backup user.
func BackupUserSecretName(clusterName, backupUser string) string {
	switch backupUser {
	case "", config.DefaultBackupUser:
		return clusterName + cnpgv1.ApplicationUserSecretSuffix
	case "postgres":
		return clusterName + cnpgv1.SuperUserSecretSuffix
	default:
		return clusterName + "-" + backupUser
	}
}

// BackupID returns the identifier of a backup, used in the object keys.
func BackupID(backup cnpgv1.Backup, now time.Time) string {
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
