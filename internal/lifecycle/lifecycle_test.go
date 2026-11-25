package lifecycle

import (
	"context"
	"errors"
	"testing"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/odit-services/cnpg-plugin-pgdump/internal/config"
	corev1 "k8s.io/api/core/v1"
)

func TestInjectSidecar(t *testing.T) {
	spec := corev1.PodSpec{Containers: []corev1.Container{{Name: "postgres"}}}
	appConfig := config.Config{SidecarImage: "plugin:test"}

	for range 2 { // idempotent
		if err := InjectSidecar(context.Background(), &spec, cnpgv1.Cluster{}, appConfig); err != nil {
			t.Fatal(err)
		}
	}

	if len(spec.InitContainers) != 1 {
		t.Fatalf("init containers %v", spec.InitContainers)
	}
	sidecar := spec.InitContainers[0]
	if sidecar.Name != SidecarName || sidecar.Image != "plugin:test" || sidecar.Args[0] != "instance" {
		t.Fatalf("sidecar %#v", sidecar)
	}
	if sidecar.RestartPolicy == nil || *sidecar.RestartPolicy != corev1.ContainerRestartPolicyAlways {
		t.Fatal("sidecar is not a native sidecar")
	}
	if len(spec.Volumes) != 2 || spec.Volumes[0].Name != pluginsVolumeName || spec.Volumes[1].Name != workVolumeName {
		t.Fatalf("volumes %v", spec.Volumes)
	}
	if mounts := spec.Containers[0].VolumeMounts; len(mounts) != 1 || mounts[0].MountPath != pluginsMountPath {
		t.Fatalf("postgres mounts %v", mounts)
	}
}

func TestInjectSidecarRequiresPostgresContainer(t *testing.T) {
	spec := corev1.PodSpec{Containers: []corev1.Container{{Name: "other"}}}
	if err := InjectSidecar(context.Background(), &spec, cnpgv1.Cluster{}, config.Config{SidecarImage: "plugin:test"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestInjectSidecarResources(t *testing.T) {
	spec := corev1.PodSpec{Containers: []corev1.Container{{Name: "postgres"}}}
	cluster := cnpgv1.Cluster{Spec: cnpgv1.ClusterSpec{Plugins: []cnpgv1.PluginConfiguration{{
		Name:       config.PluginName,
		Parameters: map[string]string{"sidecar_memory_limit": "512Mi", "sidecar_cpu_limit": "1"},
	}}}}
	appConfig := config.Config{
		SidecarImage:     "plugin:test",
		SidecarResources: map[string]string{"sidecar_cpu_request": "50m", "sidecar_memory_limit": "256Mi"},
	}

	if err := InjectSidecar(context.Background(), &spec, cluster, appConfig); err != nil {
		t.Fatal(err)
	}
	resources := spec.InitContainers[0].Resources
	if got := resources.Requests.Cpu().String(); got != "50m" {
		t.Fatalf("cpu request %s", got)
	}
	if got := resources.Limits.Memory().String(); got != "512Mi" { // Cluster parameter overrides the default
		t.Fatalf("memory limit %s", got)
	}
	if got := resources.Limits.Cpu().String(); got != "1" {
		t.Fatalf("cpu limit %s", got)
	}

	// an existing sidecar is replaced with the current configuration
	appConfig.SidecarImage = "plugin:new"
	if err := InjectSidecar(context.Background(), &spec, cluster, appConfig); err != nil {
		t.Fatal(err)
	}
	if len(spec.InitContainers) != 1 || spec.InitContainers[0].Image != "plugin:new" {
		t.Fatalf("init containers %v", spec.InitContainers)
	}
}

func TestSidecarResourcesRejectsInvalidQuantity(t *testing.T) {
	_, err := SidecarResources(nil, map[string]string{"sidecar_cpu_limit": "one"})
	var parameterError *ParameterError
	if !errors.As(err, &parameterError) || parameterError.Parameter != "sidecar_cpu_limit" {
		t.Fatalf("error %v", err)
	}
}

func TestInjectSidecarIgnoresInvalidResources(t *testing.T) {
	spec := corev1.PodSpec{Containers: []corev1.Container{{Name: "postgres"}}}
	cluster := cnpgv1.Cluster{Spec: cnpgv1.ClusterSpec{Plugins: []cnpgv1.PluginConfiguration{{
		Name:       config.PluginName,
		Parameters: map[string]string{"sidecar_memory_limit": "lots", "sidecar_cpu_limit": "1"},
	}}}}

	if err := InjectSidecar(context.Background(), &spec, cluster, config.Config{SidecarImage: "plugin:test"}); err != nil {
		t.Fatal(err)
	}
	limits := spec.InitContainers[0].Resources.Limits
	if _, ok := limits[corev1.ResourceMemory]; ok || limits.Cpu().String() != "1" {
		t.Fatalf("limits %v", limits)
	}
}
