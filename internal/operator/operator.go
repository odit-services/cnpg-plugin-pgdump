package operator

import (
	"context"
	"encoding/json"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	cnpgoperator "github.com/cloudnative-pg/cnpg-i/pkg/operator"
	pgbackup "github.com/odit-services/cnpg-plugin-pgdump/internal/backup"
)

type Server struct {
	cnpgoperator.UnimplementedOperatorServer
	store *pgbackup.Store
}

func New(store *pgbackup.Store) *Server {
	return &Server{store: store}
}

func (s *Server) GetCapabilities(context.Context, *cnpgoperator.OperatorCapabilitiesRequest) (*cnpgoperator.OperatorCapabilitiesResult, error) {
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

func (s *Server) ValidateClusterCreate(context.Context, *cnpgoperator.OperatorValidateClusterCreateRequest) (*cnpgoperator.OperatorValidateClusterCreateResult, error) {
	return &cnpgoperator.OperatorValidateClusterCreateResult{}, nil
}

func (s *Server) ValidateClusterChange(context.Context, *cnpgoperator.OperatorValidateClusterChangeRequest) (*cnpgoperator.OperatorValidateClusterChangeResult, error) {
	return &cnpgoperator.OperatorValidateClusterChangeResult{}, nil
}

func (s *Server) MutateCluster(context.Context, *cnpgoperator.OperatorMutateClusterRequest) (*cnpgoperator.OperatorMutateClusterResult, error) {
	return &cnpgoperator.OperatorMutateClusterResult{}, nil
}

func (s *Server) SetStatusInCluster(_ context.Context, request *cnpgoperator.SetStatusInClusterRequest) (*cnpgoperator.SetStatusInClusterResponse, error) {
	var cluster cnpgv1.Cluster
	if err := json.Unmarshal(request.GetCluster(), &cluster); err != nil {
		return nil, err
	}

	last := s.store.Last(cluster.Namespace + "/" + cluster.Name)
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
	return &cnpgoperator.SetStatusInClusterResponse{JsonStatus: jsonStatus}, nil
}

func (s *Server) Deregister(_ context.Context, request *cnpgoperator.DeregisterRequest) (*cnpgoperator.DeregisterResponse, error) {
	var cluster cnpgv1.Cluster
	if err := json.Unmarshal(request.GetDefinition(), &cluster); err == nil {
		s.store.Set(cluster.Namespace+"/"+cluster.Name, pgbackup.Result{})
	}
	return &cnpgoperator.DeregisterResponse{}, nil
}

func rpcCapability(kind cnpgoperator.OperatorCapability_RPC_Type) *cnpgoperator.OperatorCapability {
	return &cnpgoperator.OperatorCapability{
		Type: &cnpgoperator.OperatorCapability_Rpc{
			Rpc: &cnpgoperator.OperatorCapability_RPC{Type: kind},
		},
	}
}
