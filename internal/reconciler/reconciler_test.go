package reconciler

import (
	"context"
	"testing"

	"github.com/odit-services/cnpg-plugin-pgdump/internal/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestResolveS3Secrets(t *testing.T) {
	server := New(config.Config{}, nil, fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-s3", Namespace: "app"},
		Data: map[string][]byte{
			"endpoint":          []byte("http://rustfs:9000"),
			"region":            []byte("eu-central-1"),
			"access-key-id":     []byte("access"),
			"secret-access-key": []byte("secret"),
		},
	}), nil)

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

func TestResolveS3SecretsRequiresReferencedKeys(t *testing.T) {
	server := New(config.Config{}, nil, fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-s3", Namespace: "app"},
		Data:       map[string][]byte{},
	}), nil)

	_, err := server.resolveS3Secrets(context.Background(), "app", config.BackupConfig{
		AccessKeyIDSecret: config.SecretKeyRef{Name: "backup-s3", Key: "access-key-id"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
