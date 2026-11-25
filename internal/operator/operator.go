package operator

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	cnpgoperator "github.com/cloudnative-pg/cnpg-i/pkg/operator"
	pgbackup "github.com/odit-services/cnpg-plugin-pgdump/internal/backup"
	"github.com/odit-services/cnpg-plugin-pgdump/internal/config"
	"github.com/odit-services/cnpg-plugin-pgdump/internal/lifecycle"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var backupsResource = schema.GroupVersionResource{Group: "postgresql.cnpg.io", Version: "v1", Resource: "backups"}

type Server struct {
	cnpgoperator.UnimplementedOperatorServer
	dynamicClient dynamic.Interface
	mu            sync.Mutex
	published     map[string]string
}

func New(dynamicClient dynamic.Interface) *Server {
	return &Server{dynamicClient: dynamicClient, published: map[string]string{}}
}

func (s *Server) GetCapabilities(
	context.Context,
	*cnpgoperator.OperatorCapabilitiesRequest,
) (*cnpgoperator.OperatorCapabilitiesResult, error) {
	return &cnpgoperator.OperatorCapabilitiesResult{
		Capabilities: []*cnpgoperator.OperatorCapability{
			rpcCapability(cnpgoperator.OperatorCapability_RPC_TYPE_VALIDATE_CLUSTER_CREATE),
			rpcCapability(cnpgoperator.OperatorCapability_RPC_TYPE_VALIDATE_CLUSTER_CHANGE),
			rpcCapability(cnpgoperator.OperatorCapability_RPC_TYPE_MUTATE_CLUSTER),
			rpcCapability(cnpgoperator.OperatorCapability_RPC_TYPE_SET_STATUS_IN_CLUSTER),
			rpcCapability(cnpgoperator.OperatorCapability_RPC_TYPE_DEREGISTER),
		},
	}, nil
}

func (s *Server) ValidateClusterCreate(
	_ context.Context,
	request *cnpgoperator.OperatorValidateClusterCreateRequest,
) (*cnpgoperator.OperatorValidateClusterCreateResult, error) {
	validationErrors, err := validateCluster(request.GetDefinition())
	return &cnpgoperator.OperatorValidateClusterCreateResult{ValidationErrors: validationErrors}, err
}

func (s *Server) ValidateClusterChange(
	_ context.Context,
	request *cnpgoperator.OperatorValidateClusterChangeRequest,
) (*cnpgoperator.OperatorValidateClusterChangeResult, error) {
	validationErrors, err := validateCluster(request.GetNewCluster())
	return &cnpgoperator.OperatorValidateClusterChangeResult{ValidationErrors: validationErrors}, err
}

// validateCluster checks the sidecar resource parameters of the plugin in the Cluster spec.
func validateCluster(definition []byte) ([]*cnpgoperator.ValidationError, error) {
	var cluster cnpgv1.Cluster
	if err := json.Unmarshal(definition, &cluster); err != nil {
		return nil, err
	}
	_, err := lifecycle.SidecarResources(nil, lifecycle.PluginParameters(cluster))
	var parameterError *lifecycle.ParameterError
	if errors.As(err, &parameterError) {
		return []*cnpgoperator.ValidationError{{
			PathComponents: []string{"spec", "plugins", config.PluginName, "parameters", parameterError.Parameter},
			Value:          parameterError.Value,
			Message:        parameterError.Error(),
		}}, nil
	}
	return nil, err
}

func (s *Server) MutateCluster(
	context.Context,
	*cnpgoperator.OperatorMutateClusterRequest,
) (*cnpgoperator.OperatorMutateClusterResult, error) {
	return &cnpgoperator.OperatorMutateClusterResult{}, nil
}

func (s *Server) SetStatusInCluster(
	ctx context.Context,
	request *cnpgoperator.SetStatusInClusterRequest,
) (*cnpgoperator.SetStatusInClusterResponse, error) {
	var cluster cnpgv1.Cluster
	if err := json.Unmarshal(request.GetCluster(), &cluster); err != nil {
		return nil, err
	}

	last, err := s.lastBackup(ctx, cluster)
	if err != nil {
		return nil, err
	}
	if last.BackupID == "" {
		return &cnpgoperator.SetStatusInClusterResponse{}, nil
	}

	status := map[string]any{
		"last_successful_backup": last.StoppedAt.Format("2006-01-02T15:04:05Z07:00"),
		"last_backup_size":       last.LastBackupSize,
		"databases_backed_up":    last.DatabasesBackedUp,
		"backup_id":              last.BackupID,
		"objects":                last.Objects,
	}
	if last.ErrorMessage != "" {
		status["error_message"] = last.ErrorMessage
		delete(status, "last_successful_backup")
	}

	jsonStatus, err := json.Marshal(status)
	if err != nil {
		return nil, err
	}
	clusterKey := cluster.Namespace + "/" + cluster.Name
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.published[clusterKey] == last.BackupID {
		return &cnpgoperator.SetStatusInClusterResponse{}, nil
	}
	s.published[clusterKey] = last.BackupID
	return &cnpgoperator.SetStatusInClusterResponse{JsonStatus: jsonStatus}, nil
}

func (s *Server) Deregister(
	_ context.Context,
	request *cnpgoperator.DeregisterRequest,
) (*cnpgoperator.DeregisterResponse, error) {
	var cluster cnpgv1.Cluster
	if err := json.Unmarshal(request.GetDefinition(), &cluster); err == nil {
		clusterKey := cluster.Namespace + "/" + cluster.Name
		s.mu.Lock()
		delete(s.published, clusterKey)
		s.mu.Unlock()
	}
	return &cnpgoperator.DeregisterResponse{}, nil
}

// lastBackup returns the result of the latest finished plugin Backup of the Cluster.
func (s *Server) lastBackup(ctx context.Context, cluster cnpgv1.Cluster) (pgbackup.Result, error) {
	if s.dynamicClient == nil {
		return pgbackup.Result{}, nil
	}
	list, err := s.dynamicClient.Resource(backupsResource).Namespace(cluster.Namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return pgbackup.Result{}, err
	}

	var last *cnpgv1.Backup
	for i := range list.Items {
		var backup cnpgv1.Backup
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(list.Items[i].Object, &backup); err != nil {
			continue
		}
		if backup.Spec.Cluster.Name != cluster.Name || backup.Spec.Method != cnpgv1.BackupMethodPlugin ||
			backup.Spec.PluginConfiguration == nil || backup.Spec.PluginConfiguration.Name != config.PluginName ||
			(backup.Status.Phase != cnpgv1.BackupPhaseCompleted && backup.Status.Phase != cnpgv1.BackupPhaseFailed) {
			continue
		}
		if last == nil || stoppedAt(backup).After(stoppedAt(*last).Time) {
			last = &backup
		}
	}
	if last == nil {
		return pgbackup.Result{}, nil
	}
	return pgbackup.ResultFromBackup(*last), nil
}

func stoppedAt(backup cnpgv1.Backup) metav1.Time {
	if backup.Status.StoppedAt != nil {
		return *backup.Status.StoppedAt
	}
	return backup.CreationTimestamp
}

func rpcCapability(kind cnpgoperator.OperatorCapability_RPC_Type) *cnpgoperator.OperatorCapability {
	return &cnpgoperator.OperatorCapability{
		Type: &cnpgoperator.OperatorCapability_Rpc{
			Rpc: &cnpgoperator.OperatorCapability_RPC{Type: kind},
		},
	}
}
