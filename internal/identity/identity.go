package identity

import (
	"context"

	cnpgi "github.com/cloudnative-pg/cnpg-i/pkg/identity"
	"github.com/odit-services/cnpg-plugin-pgdump/internal/config"
)

type Server struct {
	cnpgi.UnimplementedIdentityServer
	version string
}

func New(version string) *Server {
	return &Server{version: version}
}

func (s *Server) GetPluginMetadata(context.Context, *cnpgi.GetPluginMetadataRequest) (*cnpgi.GetPluginMetadataResponse, error) {
	return &cnpgi.GetPluginMetadataResponse{
		Name:          config.PluginName,
		Version:       s.version,
		DisplayName:   "pg_dump Logical Backup Plugin",
		Description:   "Logical PostgreSQL backups for CloudNativePG using pg_dump and S3.",
		ProjectUrl:    "https://github.com/odit-services/cnpg-plugin-pgdump",
		RepositoryUrl: "https://github.com/odit-services/cnpg-plugin-pgdump",
		License:       "Apache-2.0",
		LicenseUrl:    "https://www.apache.org/licenses/LICENSE-2.0",
		Maturity:      "alpha",
		Vendor:        "ODIT.Services",
	}, nil
}

func (s *Server) GetPluginCapabilities(context.Context, *cnpgi.GetPluginCapabilitiesRequest) (*cnpgi.GetPluginCapabilitiesResponse, error) {
	return &cnpgi.GetPluginCapabilitiesResponse{
		Capabilities: []*cnpgi.PluginCapability{
			serviceCapability(cnpgi.PluginCapability_Service_TYPE_OPERATOR_SERVICE),
			serviceCapability(cnpgi.PluginCapability_Service_TYPE_BACKUP_SERVICE),
			serviceCapability(cnpgi.PluginCapability_Service_TYPE_RECONCILER_HOOKS),
		},
	}, nil
}

func (s *Server) Probe(context.Context, *cnpgi.ProbeRequest) (*cnpgi.ProbeResponse, error) {
	return &cnpgi.ProbeResponse{Ready: true}, nil
}

func serviceCapability(kind cnpgi.PluginCapability_Service_Type) *cnpgi.PluginCapability {
	return &cnpgi.PluginCapability{
		Type: &cnpgi.PluginCapability_Service_{
			Service: &cnpgi.PluginCapability_Service{Type: kind},
		},
	}
}
