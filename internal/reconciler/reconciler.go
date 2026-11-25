package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	cnpgreconciler "github.com/cloudnative-pg/cnpg-i/pkg/reconciler"
	"github.com/cloudnative-pg/machinery/pkg/log"
	pgbackup "github.com/odit-services/cnpg-plugin-pgdump/internal/backup"
	"github.com/odit-services/cnpg-plugin-pgdump/internal/config"
	"github.com/odit-services/cnpg-plugin-pgdump/internal/rbac"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var backupsResource = schema.GroupVersionResource{Group: "postgresql.cnpg.io", Version: "v1", Resource: "backups"}

// Server checks the plugin Backups before CNPG asks the instance sidecar to take them.
type Server struct {
	cnpgreconciler.UnimplementedReconcilerHooksServer
	appConfig     config.Config
	kube          kubernetes.Interface
	dynamicClient dynamic.Interface
}

func New(appConfig config.Config, kube kubernetes.Interface, dynamicClient dynamic.Interface) *Server {
	return &Server{appConfig: appConfig, kube: kube, dynamicClient: dynamicClient}
}

func (s *Server) GetCapabilities(
	context.Context,
	*cnpgreconciler.ReconcilerHooksCapabilitiesRequest,
) (*cnpgreconciler.ReconcilerHooksCapabilitiesResult, error) {
	return &cnpgreconciler.ReconcilerHooksCapabilitiesResult{
		ReconcilerCapabilities: []*cnpgreconciler.ReconcilerHooksCapability{
			{Kind: cnpgreconciler.ReconcilerHooksCapability_KIND_BACKUP},
		},
	}, nil
}

func (s *Server) Pre(
	ctx context.Context,
	request *cnpgreconciler.ReconcilerHooksRequest,
) (*cnpgreconciler.ReconcilerHooksResult, error) {
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

	parameters, ok := extractParameters(request.GetResourceDefinition(), backup.Spec.PluginConfiguration)
	if !ok {
		return continueResult(), nil
	}
	backupConfig, err := config.ParseBackupConfig(parameters, s.appConfig)
	if err != nil {
		now := time.Now().UTC()
		result := pgbackup.Result{
			BackupID:     pgbackup.BackupID(backup, now),
			StartedAt:    now,
			StoppedAt:    now,
			ErrorMessage: err.Error(),
		}
		if statusErr := s.updateBackupStatus(ctx, backup, result, cnpgv1.BackupPhaseFailed); statusErr != nil {
			logger.Error(statusErr, "Cannot update Backup status")
		}
		logger.Error(err, "Invalid backup configuration")
		return terminateResult(), nil
	}

	// The backup runs in the plugin sidecar of the instance chosen by CNPG (backup service, no timeout).
	// The sidecar uses the service account of the Cluster pods: let it read the secrets of the backup.
	if err := rbac.EnsureSecretAccess(ctx, s.kube, cluster, pgbackup.SecretNames(cluster.Name, backupConfig)); err != nil {
		return nil, fmt.Errorf("grant the Cluster pods access to the backup secrets: %w", err)
	}
	return continueResult(), nil
}

func (s *Server) updateBackupStatus(
	ctx context.Context, backup cnpgv1.Backup, result pgbackup.Result, phase cnpgv1.BackupPhase,
) error {
	if s.dynamicClient == nil {
		return fmt.Errorf("dynamic kubernetes client is not configured")
	}

	stoppedAt := result.StoppedAt
	if stoppedAt.IsZero() {
		stoppedAt = time.Now().UTC()
	}
	status := map[string]any{
		"phase":                      string(phase),
		"method":                     string(cnpgv1.BackupMethodPlugin),
		"backupId":                   result.BackupID,
		"startedAt":                  metav1.NewTime(result.StartedAt).Format(time.RFC3339),
		"stoppedAt":                  metav1.NewTime(stoppedAt).Format(time.RFC3339),
		"reconciliationTerminatedAt": metav1.NewTime(stoppedAt).Format(time.RFC3339),
		"pluginMetadata":             map[string]string{"pluginName": config.PluginName},
	}
	if result.StartedAt.IsZero() {
		delete(status, "startedAt")
	}
	if result.ErrorMessage != "" {
		status["error"] = result.ErrorMessage
		status["commandError"] = result.ErrorMessage
	}

	patch := &unstructured.Unstructured{Object: map[string]any{"status": status}}
	patchData, err := patch.MarshalJSON()
	if err != nil {
		return err
	}
	_, err = s.dynamicClient.Resource(backupsResource).Namespace(backup.Namespace).
		Patch(ctx, backup.Name, types.MergePatchType, patchData, metav1.PatchOptions{}, "status")
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (s *Server) Post(
	context.Context,
	*cnpgreconciler.ReconcilerHooksRequest,
) (*cnpgreconciler.ReconcilerHooksResult, error) {
	return continueResult(), nil
}

func extractParameters(raw []byte, typed *cnpgv1.BackupPluginConfiguration) (map[string]string, bool) {
	if typed != nil && typed.Name == config.PluginName {
		return typed.Parameters, true
	}

	var fallback struct {
		Spec struct {
			PluginConfiguration *cnpgv1.BackupPluginConfiguration `json:"pluginConfiguration"`
			Plugin              *cnpgv1.BackupPluginConfiguration `json:"plugin"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(raw, &fallback); err != nil {
		return nil, false
	}

	pluginConfigs := []*cnpgv1.BackupPluginConfiguration{fallback.Spec.PluginConfiguration, fallback.Spec.Plugin}
	for _, candidate := range pluginConfigs {
		if candidate != nil && candidate.Name == config.PluginName {
			return candidate.Parameters, true
		}
	}
	return nil, false
}

func continueResult() *cnpgreconciler.ReconcilerHooksResult {
	return &cnpgreconciler.ReconcilerHooksResult{Behavior: cnpgreconciler.ReconcilerHooksResult_BEHAVIOR_CONTINUE}
}

func terminateResult() *cnpgreconciler.ReconcilerHooksResult {
	return &cnpgreconciler.ReconcilerHooksResult{Behavior: cnpgreconciler.ReconcilerHooksResult_BEHAVIOR_TERMINATE}
}
