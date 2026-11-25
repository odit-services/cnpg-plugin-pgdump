package lifecycle

import (
	"testing"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/odit-services/cnpg-plugin-pgdump/internal/config"
	corev1 "k8s.io/api/core/v1"
)

func TestInjectSidecar(t *testing.T) {
	spec := corev1.PodSpec{Containers: []corev1.Container{{Name: "postgres"}}}
	appConfig := config.Config{SidecarImage: "plugin:test"}

	for range 2 { // idempotent
		if err := InjectSidecar(&spec, cnpgv1.Cluster{}, appConfig); err != nil {
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
	if err := InjectSidecar(&spec, cnpgv1.Cluster{}, config.Config{SidecarImage: "plugin:test"}); err == nil {
		t.Fatal("expected error")
	}
}
