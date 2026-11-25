package operator

import (
	"context"
	"encoding/json"
	"testing"

	cnpgoperator "github.com/cloudnative-pg/cnpg-i/pkg/operator"
	"github.com/odit-services/cnpg-plugin-pgdump/internal/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestSetStatusInClusterUsesLatestFinishedBackup(t *testing.T) {
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{backupsResource: "BackupList"},
		backup("old", "db", config.PluginName, "completed", "2026-11-01T02:00:00Z", "1"),
		backup("new", "db", config.PluginName, "completed", "2026-11-02T02:00:00Z", "42"),
		backup("other-plugin", "db", "barman-cloud.cloudnative-pg.io", "completed", "2026-11-03T02:00:00Z", "7"),
		backup("other-cluster", "db2", config.PluginName, "completed", "2026-11-03T02:00:00Z", "7"),
	)
	server := New(client)

	response, err := server.SetStatusInCluster(context.Background(), &cnpgoperator.SetStatusInClusterRequest{
		Cluster: []byte(`{"metadata":{"name":"db","namespace":"app"}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	var status map[string]any
	if err := json.Unmarshal(response.GetJsonStatus(), &status); err != nil {
		t.Fatal(err)
	}
	if status["backup_id"] != "new-id" || status["last_backup_size"] != float64(42) {
		t.Fatalf("status %v", status)
	}
}

func backup(name, cluster, plugin, phase, stoppedAt, size string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Backup",
		"metadata":   map[string]any{"namespace": "app", "name": name},
		"spec": map[string]any{
			"cluster":             map[string]any{"name": cluster},
			"method":              "plugin",
			"pluginConfiguration": map[string]any{"name": plugin},
		},
		"status": map[string]any{
			"phase":          phase,
			"backupId":       name + "-id",
			"stoppedAt":      stoppedAt,
			"pluginMetadata": map[string]any{"size": size, "databases": "app"},
		},
	}}
}
