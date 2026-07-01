package reconciler

import (
	"context"
	"testing"
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	pgbackup "github.com/odit-services/cnpg-plugin-pgdump/internal/backup"
	"github.com/odit-services/cnpg-plugin-pgdump/internal/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func TestResolveS3Secrets(t *testing.T) {
	server := New(config.Config{}, nil, kubefake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-s3", Namespace: "app"},
		Data: map[string][]byte{
			"endpoint":          []byte("http://rustfs:9000"),
			"region":            []byte("eu-central-1"),
			"access-key-id":     []byte("access"),
			"secret-access-key": []byte("secret"),
		},
	}), nil, nil)

	cfg, err := server.resolveS3Secrets(context.Background(), "app", config.BackupConfig{
		EndpointURL:           "http://fallback:9000",
		Region:                "us-east-1",
		EndpointURLSecret:     config.SecretKeyRef{Name: "backup-s3", Key: "endpoint"},
		RegionSecret:          config.SecretKeyRef{Name: "backup-s3", Key: "region"},
		AccessKeyIDSecret:     config.SecretKeyRef{Name: "backup-s3", Key: "access-key-id"},
		SecretAccessKeySecret: config.SecretKeyRef{Name: "backup-s3", Key: "secret-access-key"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.EndpointURL != "http://rustfs:9000" || cfg.Region != "eu-central-1" {
		t.Fatalf("resolved endpoint/region %#v", cfg)
	}
	if cfg.AccessKeyID != "access" || cfg.SecretAccessKey != "secret" {
		t.Fatalf("resolved credentials %#v", cfg)
	}
}

func TestResolveS3SecretsResolvesBucket(t *testing.T) {
	server := New(config.Config{}, nil, kubefake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "s3-creds", Namespace: "app"},
		Data: map[string][]byte{
			"bucket": []byte("my-backups"),
		},
	}), nil, nil)

	cfg, err := server.resolveS3Secrets(context.Background(), "app", config.BackupConfig{
		Bucket:       "fallback",
		BucketSecret: config.SecretKeyRef{Name: "s3-creds", Key: "bucket"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Bucket != "my-backups" {
		t.Fatalf("bucket %q", cfg.Bucket)
	}
}

func TestResolveS3SecretsBucketSecretFallback(t *testing.T) {
	// When BucketSecret.Name is empty, Bucket should not be overwritten
	server := New(config.Config{}, nil, nil, nil, nil)
	cfg, err := server.resolveS3Secrets(context.Background(), "app", config.BackupConfig{
		Bucket: "direct-bucket",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Bucket != "direct-bucket" {
		t.Fatalf("bucket %q", cfg.Bucket)
	}
}

func TestResolveS3SecretsBucketSecretEmptyName(t *testing.T) {
	// When BucketSecret references a non-existent secret, it should error
	server := New(config.Config{}, nil, kubefake.NewSimpleClientset(), nil, nil)
	_, err := server.resolveS3Secrets(context.Background(), "app", config.BackupConfig{
		BucketSecret: config.SecretKeyRef{Name: "nonexistent", Key: "bucket"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveS3SecretsRequiresReferencedKeys(t *testing.T) {
	server := New(config.Config{}, nil, kubefake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-s3", Namespace: "app"},
		Data:       map[string][]byte{},
	}), nil, nil)

	_, err := server.resolveS3Secrets(context.Background(), "app", config.BackupConfig{
		AccessKeyIDSecret: config.SecretKeyRef{Name: "backup-s3", Key: "access-key-id"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBackupUserSecretName(t *testing.T) {
	tests := map[string]string{
		"":         "cluster-app",
		"app":      "cluster-app",
		"postgres": "cluster-superuser",
		"backup":   "cluster-backup",
	}

	for user, want := range tests {
		if got := backupUserSecretName("cluster", user); got != want {
			t.Fatalf("backupUserSecretName(%q) = %q, want %q", user, got, want)
		}
	}
}

func TestReadBackupUserSecretUsesSuperuserSecret(t *testing.T) {
	server := New(config.Config{}, nil, kubefake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-superuser", Namespace: "app"},
		Data: map[string][]byte{
			corev1.BasicAuthUsernameKey: []byte("postgres"),
			corev1.BasicAuthPasswordKey: []byte("secret"),
		},
	}), nil, nil)

	password, user, err := server.readBackupUserSecret(context.Background(), "app", "cluster", "postgres")
	if err != nil {
		t.Fatal(err)
	}
	if user != "postgres" || password != "secret" {
		t.Fatalf("credentials user=%q password=%q", user, password)
	}
}

func TestUpdateBackupStatusCompleted(t *testing.T) {
	backup := backupObject("app", "logical-backup")
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		backupsResource: "BackupList",
	}, backup)
	server := New(config.Config{}, nil, nil, client, nil)
	started := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	stopped := started.Add(time.Minute)

	err := server.updateBackupStatus(context.Background(), cnpgv1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "logical-backup", Namespace: "app"},
	}, pgbackup.Result{
		BackupID:       "backup-1",
		StartedAt:      started,
		StoppedAt:      stopped,
		LastBackupSize: 42,
	}, cnpgv1.BackupPhaseCompleted)
	if err != nil {
		t.Fatal(err)
	}

	got, err := client.Resource(backupsResource).Namespace("app").Get(context.Background(), "logical-backup", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	phase, _, _ := unstructured.NestedString(got.Object, "status", "phase")
	if phase != string(cnpgv1.BackupPhaseCompleted) {
		t.Fatalf("phase %q", phase)
	}
	backupID, _, _ := unstructured.NestedString(got.Object, "status", "backupId")
	if backupID != "backup-1" {
		t.Fatalf("backup ID %q", backupID)
	}
}

func TestUpdateBackupStatusFailed(t *testing.T) {
	backup := backupObject("app", "logical-backup")
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		backupsResource: "BackupList",
	}, backup)
	server := New(config.Config{}, nil, nil, client, nil)

	err := server.updateBackupStatus(context.Background(), cnpgv1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "logical-backup", Namespace: "app"},
	}, pgbackup.Result{
		BackupID:     "backup-1",
		StartedAt:    time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
		ErrorMessage: "pg_dump failed",
	}, cnpgv1.BackupPhaseFailed)
	if err != nil {
		t.Fatal(err)
	}

	got, err := client.Resource(backupsResource).Namespace("app").Get(context.Background(), "logical-backup", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	phase, _, _ := unstructured.NestedString(got.Object, "status", "phase")
	if phase != string(cnpgv1.BackupPhaseFailed) {
		t.Fatalf("phase %q", phase)
	}
	message, _, _ := unstructured.NestedString(got.Object, "status", "error")
	if message != "pg_dump failed" {
		t.Fatalf("error %q", message)
	}
}

func backupObject(namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Backup",
		"metadata": map[string]any{
			"namespace": namespace,
			"name":      name,
		},
	}}
}
