package backup

import (
	"context"
	"testing"

	"github.com/odit-services/cnpg-plugin-pgdump/internal/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func TestResolveS3Secrets(t *testing.T) {
	runner := &Runner{Kube: kubefake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-s3", Namespace: "app"},
		Data: map[string][]byte{
			"endpoint":          []byte("http://rustfs:9000"),
			"region":            []byte("eu-central-1"),
			"access-key-id":     []byte("access"),
			"secret-access-key": []byte("secret"),
		},
	})}

	cfg, err := runner.resolveS3Secrets(context.Background(), "app", config.BackupConfig{
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
	runner := &Runner{Kube: kubefake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "s3-creds", Namespace: "app"},
		Data: map[string][]byte{
			"bucket": []byte("my-backups"),
		},
	})}

	cfg, err := runner.resolveS3Secrets(context.Background(), "app", config.BackupConfig{
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
	runner := &Runner{}
	cfg, err := runner.resolveS3Secrets(context.Background(), "app", config.BackupConfig{
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
	runner := &Runner{Kube: kubefake.NewSimpleClientset()}
	_, err := runner.resolveS3Secrets(context.Background(), "app", config.BackupConfig{
		BucketSecret: config.SecretKeyRef{Name: "nonexistent", Key: "bucket"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveS3SecretsRequiresReferencedKeys(t *testing.T) {
	runner := &Runner{Kube: kubefake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-s3", Namespace: "app"},
		Data:       map[string][]byte{},
	})}

	_, err := runner.resolveS3Secrets(context.Background(), "app", config.BackupConfig{
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
		if got := BackupUserSecretName("cluster", user); got != want {
			t.Fatalf("BackupUserSecretName(%q) = %q, want %q", user, got, want)
		}
	}
}

func TestReadBackupUserSecretUsesSuperuserSecret(t *testing.T) {
	runner := &Runner{Kube: kubefake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-superuser", Namespace: "app"},
		Data: map[string][]byte{
			corev1.BasicAuthUsernameKey: []byte("postgres"),
			corev1.BasicAuthPasswordKey: []byte("secret"),
		},
	})}

	password, user, err := runner.readBackupUserSecret(context.Background(), "app", "cluster", "postgres")
	if err != nil {
		t.Fatal(err)
	}
	if user != "postgres" || password != "secret" {
		t.Fatalf("credentials user=%q password=%q", user, password)
	}
}

func TestSecretNames(t *testing.T) {
	got := SecretNames("db", config.BackupConfig{
		BackupUser:            "postgres",
		AccessKeyIDSecret:     config.SecretKeyRef{Name: "s3", Key: "access-key-id"},
		SecretAccessKeySecret: config.SecretKeyRef{Name: "s3", Key: "secret-access-key"},
	})
	want := []string{"db-superuser", "s3", "s3"}
	if len(got) != len(want) {
		t.Fatalf("SecretNames = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SecretNames = %v, want %v", got, want)
		}
	}
}
