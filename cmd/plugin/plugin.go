package plugin

import (
	"fmt"
	"path/filepath"
	"strings"

	pluginhttp "github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/http"
	cnpgbackup "github.com/cloudnative-pg/cnpg-i/pkg/backup"
	cnpgoperator "github.com/cloudnative-pg/cnpg-i/pkg/operator"
	cnpgreconciler "github.com/cloudnative-pg/cnpg-i/pkg/reconciler"
	pgbackup "github.com/odit-services/cnpg-plugin-pgdump/internal/backup"
	"github.com/odit-services/cnpg-plugin-pgdump/internal/config"
	pgidentity "github.com/odit-services/cnpg-plugin-pgdump/internal/identity"
	pgoperator "github.com/odit-services/cnpg-plugin-pgdump/internal/operator"
	pgreconciler "github.com/odit-services/cnpg-plugin-pgdump/internal/reconciler"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func New(version string) *cobra.Command {
	appConfig := config.FromEnv(version)
	store := &pgbackup.Store{}
	identityServer := pgidentity.New(appConfig.Version)

	cmd := pluginhttp.CreateMainCmd(identityServer, func(server *grpc.Server) error {
		kube, err := kubeClient()
		if err != nil {
			return err
		}

		cnpgoperator.RegisterOperatorServer(server, pgoperator.New(store))
		cnpgreconciler.RegisterReconcilerHooksServer(server, pgreconciler.New(
			appConfig,
			pgbackup.NewPGDumpExecutor(appConfig.DumpTimeout),
			kube,
			store,
		))
		cnpgbackup.RegisterBackupServer(server, pgbackup.NewService())
		return nil
	})

	cmd.Use = "plugin"
	cmd.Flags().String("listen-address", "", "Listen address. Use unix:///path for plugin-path compatibility or host:port for TCP")

	_ = viper.BindPFlag("listen-address", cmd.Flags().Lookup("listen-address"))

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if err := translateListenAddress(); err != nil {
			return err
		}
		return nil
	}

	return cmd
}

func kubeClient() (kubernetes.Interface, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("create in-cluster kubernetes config: %w", err)
	}
	return kubernetes.NewForConfig(cfg)
}

func translateListenAddress() error {
	address := viper.GetString("listen-address")
	if address == "" {
		return nil
	}

	if strings.HasPrefix(address, "unix://") {
		path := strings.TrimPrefix(address, "unix://")
		if path == "" {
			return fmt.Errorf("empty unix listen-address")
		}
		if filepath.Base(path) == config.PluginName {
			viper.Set("plugin-path", filepath.Dir(path))
		} else {
			viper.Set("plugin-path", path)
		}
		viper.Set("server-address", "")
		return nil
	}

	viper.Set("server-address", strings.TrimPrefix(address, "tcp://"))
	return nil
}
