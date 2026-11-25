package reconciler

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	cnpgreconciler "github.com/cloudnative-pg/cnpg-i/pkg/reconciler"
	pgbackup "github.com/odit-services/cnpg-plugin-pgdump/internal/backup"
	"github.com/odit-services/cnpg-plugin-pgdump/internal/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func TestUpdateBackupStatusCompleted(t *testing.T) {
	backup := backupObject("app", "logical-backup")
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{
			backupsResource: "BackupList",
		}, backup)
	server := New(config.Config{}, nil, client)
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

	got, err := client.Resource(backupsResource).Namespace("app").
		Get(context.Background(), "logical-backup", metav1.GetOptions{})
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
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{
			backupsResource: "BackupList",
		}, backup)
	server := New(config.Config{}, nil, client)

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

	got, err := client.Resource(backupsResource).Namespace("app").
		Get(context.Background(), "logical-backup", metav1.GetOptions{})
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

func TestPreIgnoresBackupOfOtherPlugin(t *testing.T) {
	backup := backupObject("app", "barman-backup")
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{
			backupsResource: "BackupList",
		}, backup)
	server := New(config.Config{}, nil, client)

	clusterJSON, err := json.Marshal(cnpgv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "app"}})
	if err != nil {
		t.Fatal(err)
	}
	backupJSON, err := json.Marshal(cnpgv1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "barman-backup", Namespace: "app"},
		Spec: cnpgv1.BackupSpec{
			Cluster:             cnpgv1.LocalObjectReference{Name: "db"},
			Method:              cnpgv1.BackupMethodPlugin,
			PluginConfiguration: &cnpgv1.BackupPluginConfiguration{Name: "barman-cloud.cloudnative-pg.io"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := server.Pre(context.Background(), &cnpgreconciler.ReconcilerHooksRequest{
		ClusterDefinition:  clusterJSON,
		ResourceDefinition: backupJSON,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.GetBehavior() != cnpgreconciler.ReconcilerHooksResult_BEHAVIOR_CONTINUE {
		t.Fatalf("behavior %v", result.GetBehavior())
	}

	got, err := client.Resource(backupsResource).Namespace("app").
		Get(context.Background(), "barman-backup", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if phase, _, _ := unstructured.NestedString(got.Object, "status", "phase"); phase != "" {
		t.Fatalf("phase %q", phase)
	}
}

func TestPreGrantsSecretAccessAndContinues(t *testing.T) {
	kube := kubefake.NewSimpleClientset()
	server := New(config.Config{}, kube, nil)

	clusterJSON, err := json.Marshal(cnpgv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "app", UID: "uid-1"}})
	if err != nil {
		t.Fatal(err)
	}
	backupJSON, err := json.Marshal(cnpgv1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "logical", Namespace: "app"},
		Spec: cnpgv1.BackupSpec{
			Cluster: cnpgv1.LocalObjectReference{Name: "db"},
			Method:  cnpgv1.BackupMethodPlugin,
			PluginConfiguration: &cnpgv1.BackupPluginConfiguration{
				Name: config.PluginName,
				Parameters: map[string]string{
					"bucket":                        "backups",
					"access_key_id_secret_name":     "s3",
					"secret_access_key_secret_name": "s3",
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := server.Pre(context.Background(), &cnpgreconciler.ReconcilerHooksRequest{
		ClusterDefinition:  clusterJSON,
		ResourceDefinition: backupJSON,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.GetBehavior() != cnpgreconciler.ReconcilerHooksResult_BEHAVIOR_CONTINUE {
		t.Fatalf("behavior %v", result.GetBehavior())
	}

	role, err := kube.RbacV1().Roles("app").Get(context.Background(), "db-pgdump-backup", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := role.Rules[0].ResourceNames; len(got) != 2 || got[0] != "db-app" || got[1] != "s3" {
		t.Fatalf("role resource names %v", got)
	}
	binding, err := kube.RbacV1().RoleBindings("app").Get(context.Background(), "db-pgdump-backup", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if binding.Subjects[0].Name != "db" || binding.RoleRef.Name != "db-pgdump-backup" {
		t.Fatalf("role binding %#v", binding)
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
