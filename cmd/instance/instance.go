// Package instance contains the command of the sidecar injected in the PostgreSQL pods.
package instance

import (
	"fmt"

	pluginhttp "github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/http"
	cnpgbackup "github.com/cloudnative-pg/cnpg-i/pkg/backup"
	"github.com/cloudnative-pg/machinery/pkg/log"
	pgbackup "github.com/odit-services/cnpg-plugin-pgdump/internal/backup"
	"github.com/odit-services/cnpg-plugin-pgdump/internal/config"
	pginstance "github.com/odit-services/cnpg-plugin-pgdump/internal/instance"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// PluginPath is the folder of the plugin sockets in the PostgreSQL pods, scanned by the instance manager.
const PluginPath = "/plugins"

func New(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "instance",
		Short: "Run the backup service in the PostgreSQL pods (sidecar injected by the plugin)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			logFlags := log.NewFlags(zap.Options{})
			logFlags.ConfigureLogging()
			ctx := log.IntoContext(cmd.Context(), log.WithName("instance"))

			appConfig := config.FromEnv(version)
			restConfig, err := rest.InClusterConfig()
			if err != nil {
				return fmt.Errorf("create in-cluster kubernetes config: %w", err)
			}
			kube, err := kubernetes.NewForConfig(restConfig)
			if err != nil {
				return err
			}

			srv := pluginhttp.Server{
				IdentityImpl: pginstance.NewIdentity(appConfig.Version),
				Enrichers: []pluginhttp.ServerEnricher{func(server *grpc.Server) error {
					cnpgbackup.RegisterBackupServer(server, &pginstance.BackupService{
						AppConfig: appConfig,
						Runner: &pgbackup.Runner{
							AppConfig: appConfig,
							Executor:  pgbackup.NewPGDumpExecutor(appConfig.DumpTimeout),
							Kube:      kube,
						},
					})
					return nil
				}},
				PluginPath: PluginPath,
			}
			return srv.Start(ctx)
		},
	}
}
