// Package lifecycle injects the backup sidecar in the PostgreSQL pods of the Clusters using the plugin.
package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/decoder"
	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/object"
	cnpglifecycle "github.com/cloudnative-pg/cnpg-i/pkg/lifecycle"
	"github.com/cloudnative-pg/machinery/pkg/log"
	"github.com/odit-services/cnpg-plugin-pgdump/internal/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

const (
	// SidecarName is the name of the injected container.
	SidecarName = "plugin-pgdump"

	pluginsVolumeName = "plugins"
	pluginsMountPath  = "/plugins"
	workVolumeName    = "pgdump-work"
	workMountPath     = "/work"
)

type Server struct {
	cnpglifecycle.UnimplementedOperatorLifecycleServer
	appConfig config.Config
}

func New(appConfig config.Config) *Server {
	return &Server{appConfig: appConfig}
}

func (s *Server) GetCapabilities(
	context.Context,
	*cnpglifecycle.OperatorLifecycleCapabilitiesRequest,
) (*cnpglifecycle.OperatorLifecycleCapabilitiesResponse, error) {
	return &cnpglifecycle.OperatorLifecycleCapabilitiesResponse{
		LifecycleCapabilities: []*cnpglifecycle.OperatorLifecycleCapabilities{{
			Group: "",
			Kind:  "Pod",
			OperationTypes: []*cnpglifecycle.OperatorOperationType{
				{Type: cnpglifecycle.OperatorOperationType_TYPE_CREATE},
				{Type: cnpglifecycle.OperatorOperationType_TYPE_EVALUATE},
			},
		}},
	}, nil
}

func (s *Server) LifecycleHook(
	ctx context.Context, request *cnpglifecycle.OperatorLifecycleRequest,
) (*cnpglifecycle.OperatorLifecycleResponse, error) {
	logger := log.FromContext(ctx)

	kind, err := object.GetKind(request.GetObjectDefinition())
	if err != nil {
		return nil, err
	}
	if kind != "Pod" {
		return &cnpglifecycle.OperatorLifecycleResponse{}, nil
	}

	var cluster cnpgv1.Cluster
	if err := json.Unmarshal(request.GetClusterDefinition(), &cluster); err != nil {
		return nil, err
	}
	pod, err := decoder.DecodePodJSON(request.GetObjectDefinition())
	if err != nil {
		return nil, err
	}

	if s.appConfig.SidecarImage == "" {
		logger.Error(errors.New("SIDECAR_IMAGE is not set"), "Cannot inject the backup sidecar", "pod", pod.Name)
		return &cnpglifecycle.OperatorLifecycleResponse{}, nil
	}

	mutated := pod.DeepCopy()
	if err := InjectSidecar(ctx, &mutated.Spec, cluster, s.appConfig); err != nil {
		if errors.Is(err, object.ErrNoPostgresContainerFound) {
			return &cnpglifecycle.OperatorLifecycleResponse{}, nil
		}
		return nil, err
	}

	patch, err := object.CreatePatch(mutated, pod)
	if err != nil {
		return nil, err
	}
	return &cnpglifecycle.OperatorLifecycleResponse{JsonPatch: patch}, nil
}

// InjectSidecar adds the backup sidecar (native sidecar init container) and its volumes to an instance pod spec.
// An existing sidecar is replaced, so that image and resources changes reach the pods.
func InjectSidecar(ctx context.Context, spec *corev1.PodSpec, cluster cnpgv1.Cluster, appConfig config.Config) error {
	// an invalid value must not prevent CNPG from creating the PostgreSQL pods: it is logged and ignored
	resources, err := SidecarResources(appConfig.SidecarResources, PluginParameters(cluster))
	if err != nil {
		log.FromContext(ctx).Error(err, "Ignoring invalid sidecar resources", "cluster", cluster.Name)
	}

	sidecar := corev1.Container{
		Name:            SidecarName,
		Image:           appConfig.SidecarImage,
		ImagePullPolicy: cluster.Spec.ImagePullPolicy,
		Args:            []string{"instance"},
		Env: []corev1.EnvVar{
			{Name: "PGDUMP_WORKDIR", Value: workMountPath},
			{Name: "PGDUMP_TIMEOUT", Value: appConfig.DumpTimeout.String()},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: pluginsVolumeName, MountPath: pluginsMountPath},
			{Name: workVolumeName, MountPath: workMountPath},
		},
		Resources: resources,
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			Privileged:               ptr.To(false),
			RunAsNonRoot:             ptr.To(true),
			ReadOnlyRootFilesystem:   ptr.To(true),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
			SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		},
	}

	spec.InitContainers = slices.DeleteFunc(spec.InitContainers, func(c corev1.Container) bool {
		return c.Name == SidecarName
	})
	if err := object.InjectPluginInitContainerSidecarSpec(spec, &sidecar, false); err != nil {
		return err
	}

	for _, volume := range spec.Volumes {
		if volume.Name == workVolumeName {
			return nil
		}
	}
	// dumps are written to the node local disk, then uploaded and deleted
	spec.Volumes = append(spec.Volumes, corev1.Volume{
		Name:         workVolumeName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	})
	return nil
}

// PluginParameters returns the parameters of the plugin in the Cluster spec.
func PluginParameters(cluster cnpgv1.Cluster) map[string]string {
	for _, plugin := range cluster.Spec.Plugins {
		if plugin.Name == config.PluginName {
			return plugin.Parameters
		}
	}
	return nil
}

// ParameterError reports an invalid sidecar resource parameter.
type ParameterError struct {
	Parameter string
	Value     string
	Err       error
}

func (e *ParameterError) Error() string {
	return fmt.Sprintf("invalid %s %q: %v", e.Parameter, e.Value, e.Err)
}

// SidecarResources returns the requests and limits of the sidecar: the plugin defaults overridden by the Cluster
// plugin parameters (see config.SidecarResourceParameters). Invalid values are left out and reported in the error.
func SidecarResources(defaults, parameters map[string]string) (corev1.ResourceRequirements, error) {
	var resources corev1.ResourceRequirements
	var errs []error
	for _, item := range []struct {
		parameter string
		list      *corev1.ResourceList
		name      corev1.ResourceName
	}{
		{"sidecar_cpu_request", &resources.Requests, corev1.ResourceCPU},
		{"sidecar_memory_request", &resources.Requests, corev1.ResourceMemory},
		{"sidecar_cpu_limit", &resources.Limits, corev1.ResourceCPU},
		{"sidecar_memory_limit", &resources.Limits, corev1.ResourceMemory},
	} {
		value := parameters[item.parameter]
		if value == "" {
			value = defaults[item.parameter]
		}
		if value == "" {
			continue
		}
		quantity, err := resource.ParseQuantity(value)
		if err != nil {
			errs = append(errs, &ParameterError{Parameter: item.parameter, Value: value, Err: err})
			continue
		}
		if *item.list == nil {
			*item.list = corev1.ResourceList{}
		}
		(*item.list)[item.name] = quantity
	}
	return resources, errors.Join(errs...)
}
