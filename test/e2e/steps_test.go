//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"
)

const (
	clusterName      = "cnpg-plugin-pgdump-e2e"
	registryName     = "kind-registry"
	registryPort     = "5001"
	kindNodeImage    = "docker.io/kindest/node:v1.32.5"
	pluginImageBase  = "localhost:5001/cnpg-plugin-pgdump"
	cnpgVersion      = "1.26.0"
	e2eNamespace     = "pgdump-e2e"
	rustFSAccessKey  = "rustfsadmin"
	rustFSSecretKey  = "rustfsadmin"
	rustFSBucket     = "team-backups"
	rustFSEndpoint   = "http://rustfs.pgdump-e2e.svc.cluster.local:9000"
	defaultPGVersion = "16"
)

var postgresVersionsFlag = flag.String("postgres-versions", envDefault("POSTGRES_VERSIONS", defaultPGVersion), "comma-separated PostgreSQL major versions to test")
var containerRuntimeFlag = flag.String("container-runtime", envDefault("CONTAINER_RUNTIME", "auto"), "container runtime for image builds: auto, docker, or podman")
var pluginImageTag = flag.String("plugin-image-tag", envDefault("PLUGIN_IMAGE_TAG", ""), "unique tag for the e2e plugin image; auto-generated if empty")
var e2eParallelismFlag = flag.Int("parallelism", envDefaultInt("E2E_PARALLELISM", 2), "maximum PostgreSQL versions to test concurrently")
var e2eRestoreTestFlag = flag.Bool("restore-test", envDefaultBool("E2E_RESTORE_TEST", false), "enable restore-from-S3 test after each backup")

type suiteState struct {
	postgresVersions   []string
	verifiedVersions   map[string]bool
	containerRuntime   string
	pluginImage        string
	mu                 sync.Mutex
	restoreTestEnabled bool
}

func InitializeScenario(ctx *godog.ScenarioContext) {
	tag := *pluginImageTag
	if tag == "" {
		tag = fmt.Sprintf("e2e-%d", time.Now().Unix())
	}

	state := &suiteState{
		postgresVersions:   parseVersions(*postgresVersionsFlag),
		verifiedVersions:   map[string]bool{},
		pluginImage:        pluginImageBase + ":" + tag,
		restoreTestEnabled: *e2eRestoreTestFlag,
	}

	ctx.Step(`^a kind cluster for pgdump e2e tests$`, state.aKindClusterForPgdumpE2ETests)
	ctx.Step(`^CloudNativePG is installed$`, state.cloudNativePGIsInstalled)
	ctx.Step(`^RustFS is running as the S3 target$`, state.rustFSIsRunningAsTheS3Target)
	ctx.Step(`^the pgdump plugin is deployed$`, state.thePgdumpPluginIsDeployed)
	ctx.Step(`^I run logical backups for the configured PostgreSQL versions$`, state.iRunLogicalBackupsForTheConfiguredPostgreSQLVersions)
	ctx.Step(`^every PostgreSQL version should have uploaded dumps to RustFS$`, state.everyPostgreSQLVersionShouldHaveUploadedDumpsToRustFS)
	ctx.Step(`^I should be able to restore dumps from S3$`, state.iShouldBeAbleToRestoreDumpsFromS3)
}

func (s *suiteState) aKindClusterForPgdumpE2ETests(ctx context.Context) error {
	runtime, err := detectContainerRuntime(ctx)
	if err != nil {
		return err
	}
	s.containerRuntime = runtime

	if _, err := command(ctx, "kind", "get", "clusters"); err != nil {
		return fmt.Errorf("kind CLI is required: %w", err)
	}
	clusters, err := command(ctx, "kind", "get", "clusters")
	if err != nil {
		return err
	}
	if !containsLine(clusters, clusterName) {
		configPath, cleanup, err := kindConfigWithRegistry()
		if err != nil {
			return err
		}
		defer cleanup()
		if _, err := command(ctx, "kind", "create", "cluster", "--name", clusterName, "--config", configPath, "--wait", "120s"); err != nil {
			return err
		}
	}
	if err := s.ensureLocalRegistry(ctx); err != nil {
		return err
	}

	if _, err := command(ctx, "kubectl", "cluster-info", "--context", "kind-"+clusterName); err != nil {
		return err
	}
	return nil
}

func (s *suiteState) cloudNativePGIsInstalled(ctx context.Context) error {
	manifest := fmt.Sprintf("https://github.com/cloudnative-pg/cloudnative-pg/releases/download/v%s/cnpg-%s.yaml", cnpgVersion, cnpgVersion)
	if _, err := command(ctx, "kubectl", "apply", "--server-side", "-f", manifest); err != nil {
		return err
	}
	return waitFor(ctx, 5*time.Minute, func(ctx context.Context) error {
		_, err := command(ctx, "kubectl", "-n", "cnpg-system", "rollout", "status", "deployment/cnpg-controller-manager", "--timeout=10s")
		return err
	})
}

func (s *suiteState) rustFSIsRunningAsTheS3Target(ctx context.Context) error {
	if _, err := kubectlApply(ctx, rustFSManifest()); err != nil {
		return err
	}
	if err := waitFor(ctx, 3*time.Minute, func(ctx context.Context) error {
		_, err := command(ctx, "kubectl", "-n", e2eNamespace, "rollout", "status", "deployment/rustfs", "--timeout=10s")
		return err
	}); err != nil {
		return err
	}

	return s.runInAWSPod(ctx,
		"aws", "--endpoint-url", "http://rustfs:9000", "s3api", "create-bucket", "--bucket", rustFSBucket,
	)
}

func (s *suiteState) thePgdumpPluginIsDeployed(ctx context.Context) error {
	if s.containerRuntime == "" {
		runtime, err := detectContainerRuntime(ctx)
		if err != nil {
			return err
		}
		s.containerRuntime = runtime
	}
	if _, err := command(ctx, s.containerRuntime, "build", "-t", s.pluginImage, "."); err != nil {
		return err
	}
	if _, err := command(ctx, s.containerRuntime, "push", "--tls-verify=false", s.pluginImage); err != nil {
		return err
	}

	manifest, err := s.pluginManifest()
	if err != nil {
		return err
	}
	if _, err := kubectlApply(ctx, manifest); err != nil {
		return err
	}
	if err := waitFor(ctx, 3*time.Minute, func(ctx context.Context) error {
		_, err := command(ctx, "kubectl", "-n", "cnpg-system", "rollout", "status", "deployment/cnpg-plugin-pgdump", "--timeout=10s")
		return err
	}); err != nil {
		return err
	}
	if err := s.restartOperatorForPluginDiscovery(ctx); err != nil {
		return err
	}
	return nil
}

func (s *suiteState) restartOperatorForPluginDiscovery(ctx context.Context) error {
	if _, err := kubectlApply(ctx, operatorPluginConfigManifest()); err != nil {
		return err
	}
	_, err := command(ctx, "kubectl", "rollout", "restart", "deployment/cnpg-controller-manager", "-n", "cnpg-system")
	if err != nil {
		return err
	}
	if err := waitFor(ctx, 3*time.Minute, func(ctx context.Context) error {
		_, err := command(ctx, "kubectl", "-n", "cnpg-system", "rollout", "status", "deployment/cnpg-controller-manager", "--timeout=10s")
		return err
	}); err != nil {
		return err
	}
	time.Sleep(15 * time.Second)
	return nil
}

func operatorPluginConfigManifest() string {
	return `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cnpg-controller-manager-config
  namespace: cnpg-system
data:
  INCLUDE_PLUGINS: pgdump-backup.cloudnative-pg.io
`
}

func (s *suiteState) iRunLogicalBackupsForTheConfiguredPostgreSQLVersions(ctx context.Context) error {
	if len(s.postgresVersions) == 0 {
		return errors.New("no PostgreSQL versions configured")
	}

	parallelism := *e2eParallelismFlag
	if parallelism < 1 {
		parallelism = 1
	}

	sem := make(chan struct{}, parallelism)
	var wg sync.WaitGroup
	var errMu sync.Mutex
	var firstErr error

	for _, version := range s.postgresVersions {
		version := version
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := s.runLogicalBackupForVersion(ctx, version); err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
			}
		}()
	}
	wg.Wait()
	return firstErr
}

func (s *suiteState) runLogicalBackupForVersion(ctx context.Context, version string) error {
	cluster := "pgdump-pg" + version
	scheduledBackupName := "logical-" + cluster
	prefix := fmt.Sprintf("logical/%s/%s/", e2eNamespace, cluster)

	if err := s.cleanBackupArtifacts(ctx, cluster, scheduledBackupName, prefix); err != nil {
		return fmt.Errorf("PostgreSQL %s cleanup: %w", version, err)
	}
	_, _ = command(ctx, "kubectl", "-n", e2eNamespace, "delete", "cluster", cluster, "--ignore-not-found", "--timeout=120s")
	if _, err := kubectlApply(ctx, cnpgClusterManifest(cluster, version)); err != nil {
		return fmt.Errorf("PostgreSQL %s cluster apply: %w", version, err)
	}
	if err := waitForClusterReady(ctx, cluster); err != nil {
		return fmt.Errorf("PostgreSQL %s cluster ready: %w", version, err)
	}
	if err := createSecondDatabase(ctx, cluster); err != nil {
		return fmt.Errorf("PostgreSQL %s create extra database: %w", version, err)
	}
	if err := seedSampleData(ctx, cluster); err != nil {
		return fmt.Errorf("PostgreSQL %s seed data: %w", version, err)
	}
	if _, err := kubectlApply(ctx, scheduledBackupManifest(scheduledBackupName, cluster)); err != nil {
		return fmt.Errorf("PostgreSQL %s scheduled backup apply: %w", version, err)
	}
	if err := s.waitForBackup(ctx, cluster); err != nil {
		return fmt.Errorf("PostgreSQL %s wait for backup: %w", version, err)
	}
	if err := s.waitForS3Objects(ctx, prefix, 2); err != nil {
		return fmt.Errorf("PostgreSQL %s wait for dumps: %w", version, err)
	}

	if s.restoreTestEnabled {
		if err := s.restoreBackupForVersion(ctx, version); err != nil {
			return fmt.Errorf("PostgreSQL %s restore: %w", version, err)
		}
	}

	s.mu.Lock()
	s.verifiedVersions[version] = true
	s.mu.Unlock()
	return nil
}

func (s *suiteState) cleanBackupArtifacts(ctx context.Context, cluster, scheduledBackupName, prefix string) error {
	_, _ = command(ctx, "kubectl", "-n", e2eNamespace, "delete", "scheduledbackup", scheduledBackupName, "--ignore-not-found", "--timeout=120s")
	_, _ = command(ctx, "kubectl", "-n", e2eNamespace, "delete", "backup", "-l", "cnpg.io/cluster="+cluster, "--ignore-not-found", "--timeout=120s")
	return s.deleteS3Prefix(ctx, prefix)
}

func (s *suiteState) everyPostgreSQLVersionShouldHaveUploadedDumpsToRustFS() error {
	for _, version := range s.postgresVersions {
		s.mu.Lock()
		verified := s.verifiedVersions[version]
		s.mu.Unlock()
		if !verified {
			return fmt.Errorf("PostgreSQL %s was not verified", version)
		}
	}
	return nil
}

func (s *suiteState) iShouldBeAbleToRestoreDumpsFromS3() error {
	return nil
}

func (s *suiteState) restoreBackupForVersion(ctx context.Context, version string) error {
	cluster := "pgdump-pg" + version
	ns := e2eNamespace
	pod := cluster + "-1"

	allOutput, err := s.runInAWSPodOutput(ctx,
		"aws", "--endpoint-url", "http://rustfs:9000", "s3api", "list-objects-v2",
		"--bucket", rustFSBucket,
		"--prefix", fmt.Sprintf("logical/%s/%s/", ns, cluster),
		"--query", "Contents[?ends_with(Key, '.dump')].Key",
		"--output", "text",
	)
	if err != nil {
		return fmt.Errorf("list cluster dumps: %w", err)
	}
	allKeys := s3ObjectKeys(allOutput)
	if len(allKeys) == 0 {
		return fmt.Errorf("no dumps found under prefix logical/%s/%s/", ns, cluster)
	}
	var dumpKey string
	for _, k := range allKeys {
		if strings.Contains(k, "/app/") {
			dumpKey = k
			break
		}
	}
	if dumpKey == "" {
		return fmt.Errorf("no app dump among keys: %v", allKeys)
	}

	dumpData, err := s.runInAWSPodOutput(ctx,
		"aws", "--endpoint-url", "http://rustfs:9000", "s3", "cp", "s3://"+rustFSBucket+"/"+dumpKey, "-",
	)
	if err != nil {
		return fmt.Errorf("download %s: %w", dumpKey, err)
	}

	if _, err := command(ctx, "kubectl", "-n", ns, "exec", pod, "-c", "postgres", "--",
		"psql", "-U", "postgres", "-d", "postgres", "-c", "CREATE DATABASE restore_test",
	); err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("create restore_test: %w", err)
	}

	cmd := exec.CommandContext(ctx, "kubectl", "-n", ns, "exec", "-i", pod, "-c", "postgres", "--",
		"pg_restore", "-U", "postgres", "-d", "restore_test", "-Fc",
	)
	cmd.Stdin = strings.NewReader(dumpData)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_restore failed: %w\nstderr: %s", err, stderr.String())
	}

	out, err := command(ctx, "kubectl", "-n", ns, "exec", pod, "-c", "postgres", "--",
		"psql", "-U", "postgres", "-d", "restore_test", "-t", "-A", "-c", "SELECT COUNT(*) FROM e2e_widgets",
	)
	if err != nil {
		return fmt.Errorf("verify restore count: %w", err)
	}
	count, err := firstInteger(out)
	if err != nil {
		return fmt.Errorf("parse widget count: %w", err)
	}
	if count != 3 {
		return fmt.Errorf("expected 3 widgets after restore, got %d", count)
	}
	return nil
}

func waitForClusterReady(ctx context.Context, name string) error {
	return waitFor(ctx, 10*time.Minute, func(ctx context.Context) error {
		_, err := command(ctx, "kubectl", "-n", e2eNamespace, "wait", "cluster/"+name, "--for=condition=Ready", "--timeout=20s")
		return err
	})
}

func createSecondDatabase(ctx context.Context, cluster string) error {
	pod := cluster + "-1"
	return waitFor(ctx, 3*time.Minute, func(ctx context.Context) error {
		_, err := command(ctx, "kubectl", "-n", e2eNamespace, "exec", pod, "-c", "postgres", "--", "psql", "-U", "postgres", "-d", "postgres", "-c", "CREATE DATABASE extra;")
		if err != nil && strings.Contains(err.Error(), "already exists") {
			return nil
		}
		return err
	})
}

func execInPod(ctx context.Context, pod, database, sql string) error {
	_, err := command(ctx, "kubectl", "-n", e2eNamespace, "exec", pod, "-c", "postgres", "--",
		"psql", "-U", "postgres", "-d", database, "-c", sql,
	)
	return err
}

func seedSampleData(ctx context.Context, cluster string) error {
	pod := cluster + "-1"
	for _, db := range []string{"app", "postgres", "extra"} {
		for _, stmt := range []string{
			`CREATE TABLE IF NOT EXISTS e2e_widgets (id SERIAL PRIMARY KEY, name TEXT NOT NULL, quantity INT NOT NULL DEFAULT 0)`,
			`INSERT INTO e2e_widgets (name, quantity) VALUES ('foo', 10), ('bar', 20), ('baz', 30) ON CONFLICT DO NOTHING`,
			`GRANT SELECT ON ALL TABLES IN SCHEMA public TO app`,
			`GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO app`,
		} {
			if err := execInPod(ctx, pod, db, stmt); err != nil {
				return fmt.Errorf("seed %s/%s: %w", db, stmt[:40], err)
			}
		}
	}
	return nil
}

func (s *suiteState) waitForBackup(ctx context.Context, cluster string) error {
	return waitFor(ctx, 2*time.Minute, func(ctx context.Context) error {
		out, err := command(ctx, "kubectl", "-n", e2eNamespace, "get", "backup", "-l", "cnpg.io/cluster="+cluster, "-o", "name")
		if err != nil {
			return err
		}
		if strings.TrimSpace(out) == "" {
			return fmt.Errorf("no backup resource yet")
		}
		return nil
	})
}

func (s *suiteState) waitForS3Objects(ctx context.Context, prefix string, want int) error {
	return waitFor(ctx, 10*time.Minute, func(ctx context.Context) error {
		output, err := s.runInAWSPodOutput(ctx,
			"aws", "--endpoint-url", "http://rustfs:9000", "s3api", "list-objects-v2",
			"--bucket", rustFSBucket,
			"--prefix", prefix,
			"--query", "Contents[?ends_with(Key, '.dump')].Key",
			"--output", "text",
		)
		if err != nil {
			return err
		}
		count := s3ObjectCount(output)
		if count < want {
			return fmt.Errorf("found %d dump objects under %s, want at least %d", count, prefix, want)
		}
		return nil
	})
}

func s3ObjectCount(output string) int {
	count := 0
	for _, field := range strings.Fields(output) {
		if strings.HasSuffix(field, ".dump") {
			count++
		}
	}
	return count
}

func s3ObjectKeys(output string) []string {
	var keys []string
	for _, field := range strings.Fields(output) {
		if strings.HasSuffix(field, ".dump") {
			keys = append(keys, field)
		}
	}
	return keys
}

func (s *suiteState) deleteS3Prefix(ctx context.Context, prefix string) error {
	return s.runInAWSPod(ctx,
		"aws", "--endpoint-url", "http://rustfs:9000", "s3", "rm", "s3://"+rustFSBucket+"/"+prefix, "--recursive",
	)
}

func firstInteger(output string) (int, error) {
	for _, field := range strings.Fields(output) {
		var value int
		if _, err := fmt.Sscanf(field, "%d", &value); err == nil {
			return value, nil
		}
	}
	return 0, fmt.Errorf("no integer found in output: %s", output)
}

func (s *suiteState) runInAWSPod(ctx context.Context, args ...string) error {
	_, err := s.runInAWSPodOutput(ctx, args...)
	return err
}

func (s *suiteState) runInAWSPodOutput(ctx context.Context, args ...string) (string, error) {
	podName := fmt.Sprintf("aws-cli-%d", time.Now().UnixNano())
	runArgs := []string{
		"-n", e2eNamespace, "run", podName, "--rm", "-i", "--restart=Never",
		"--image", "docker.io/amazon/aws-cli:2.17.50",
		"--env", "AWS_ACCESS_KEY_ID=" + rustFSAccessKey,
		"--env", "AWS_SECRET_ACCESS_KEY=" + rustFSSecretKey,
		"--env", "AWS_DEFAULT_REGION=us-east-1",
		"--command", "--",
	}
	runArgs = append(runArgs, args...)
	return command(ctx, "kubectl", runArgs...)
}

func (s *suiteState) ensureLocalRegistry(ctx context.Context) error {
	_, _ = command(ctx, s.containerRuntime, "rm", "-f", registryName)

	_, err := command(ctx, s.containerRuntime,
		"run", "-d", "--restart=always",
		"--network", "kind",
		"-p", "127.0.0.1:"+registryPort+":5000",
		"--name", registryName,
		"docker.io/library/registry:2",
	)
	return err
}

func kindConfigWithRegistry() (string, func(), error) {
	file, err := os.CreateTemp("", "cnpg-plugin-pgdump-kind-*.yaml")
	if err != nil {
		return "", func() {}, err
	}
	content := fmt.Sprintf(`kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    image: %[3]s
containerdConfigPatches:
  - |-
    [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:%[1]s"]
      endpoint = ["http://%[2]s:5000"]
`, registryPort, registryName, kindNodeImage)
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return "", func() {}, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return "", func() {}, err
	}
	return file.Name(), func() { _ = os.Remove(file.Name()) }, nil
}

func waitFor(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	deadline := time.Now().Add(timeout)
	var last error
	for time.Now().Before(deadline) {
		last = fn(ctx)
		if last == nil {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timed out after %s: %w", timeout, last)
}

func command(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = os.Environ()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = repoRoot()
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%s %s failed: %w\nstdout: %s\nstderr: %s", name, strings.Join(args, " "), err, stdout.String(), stderr.String())
	}
	return stdout.String(), nil
}

func kubectlApply(ctx context.Context, manifest string) (string, error) {
	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	cmd.Dir = repoRoot()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("kubectl apply failed: %w\nstdout: %s\nstderr: %s\nmanifest:\n%s", err, stdout.String(), stderr.String(), manifest)
	}
	return stdout.String(), nil
}

func rustFSManifest() string {
	return fmt.Sprintf(`
apiVersion: v1
kind: Namespace
metadata:
  name: %[1]s
---
apiVersion: v1
kind: Secret
metadata:
  name: backup-s3-credentials
  namespace: %[1]s
type: Opaque
stringData:
  endpoint: %[4]s
  access-key-id: %[2]s
  secret-access-key: %[3]s
  region: us-east-1
  bucket: %[5]s
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: rustfs
  namespace: %[1]s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: rustfs
  template:
    metadata:
      labels:
        app: rustfs
    spec:
      securityContext:
        fsGroup: 10001
      containers:
        - name: rustfs
          image: docker.io/rustfs/rustfs:latest
          ports:
            - containerPort: 9000
            - containerPort: 9001
          env:
            - name: RUSTFS_ACCESS_KEY
              value: %[2]s
            - name: RUSTFS_SECRET_KEY
              value: %[3]s
          volumeMounts:
            - name: data
              mountPath: /data
            - name: logs
              mountPath: /logs
      volumes:
        - name: data
          emptyDir: {}
        - name: logs
          emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: rustfs
  namespace: %[1]s
spec:
  selector:
    app: rustfs
  ports:
    - name: s3
      port: 9000
      targetPort: 9000
    - name: console
      port: 9001
      targetPort: 9001
`, e2eNamespace, rustFSAccessKey, rustFSSecretKey, rustFSEndpoint, rustFSBucket)
}

func (s *suiteState) pluginManifest() (string, error) {
	path := filepath.Join(repoRoot(), "kubernetes", "deployment.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	manifest := strings.ReplaceAll(string(data), "platform/cnpg-plugin-pgdump:latest", s.pluginImage)
	manifest = strings.ReplaceAll(manifest, "image: "+s.pluginImage, "image: "+s.pluginImage+"\n          imagePullPolicy: IfNotPresent")
	manifest = strings.ReplaceAll(manifest, "value: us-east-1", "value: us-east-1")
	tlsManifest, err := pluginTLSSecretsManifest()
	if err != nil {
		return "", err
	}
	manifest = tlsManifest + manifest
	return manifest, nil
}

func pluginTLSSecretsManifest() (string, error) {
	caCertPEM, caKey, err := newCertificateAuthority()
	if err != nil {
		return "", err
	}
	serverCertPEM, serverKeyPEM, err := newSignedCertificate(caCertPEM, caKey, "cnpg-plugin-pgdump", []string{
		"cnpg-plugin-pgdump",
		"cnpg-plugin-pgdump.cnpg-system",
		"cnpg-plugin-pgdump.cnpg-system.svc",
		"cnpg-plugin-pgdump.cnpg-system.svc.cluster.local",
	}, nil)
	if err != nil {
		return "", err
	}
	clientCertPEM, clientKeyPEM, err := newSignedCertificate(caCertPEM, caKey, "cnpg-plugin-pgdump-client", nil, nil)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: cnpg-plugin-pgdump-server-tls
  namespace: cnpg-system
type: kubernetes.io/tls
stringData:
  tls.crt: |
%s
  tls.key: |
%s
  ca.crt: |
%s
---
apiVersion: v1
kind: Secret
metadata:
  name: cnpg-plugin-pgdump-client-tls
  namespace: cnpg-system
type: kubernetes.io/tls
stringData:
  tls.crt: |
%s
  tls.key: |
%s
  ca.crt: |
%s
---
`, indentPEM(serverCertPEM), indentPEM(serverKeyPEM), indentPEM(caCertPEM), indentPEM(clientCertPEM), indentPEM(clientKeyPEM), indentPEM(caCertPEM)), nil
}

func newCertificateAuthority() ([]byte, *rsa.PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: "cnpg-plugin-pgdump-e2e-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), key, nil
}

func newSignedCertificate(caCertPEM []byte, caKey *rsa.PrivateKey, commonName string, dnsNames []string, ips []net.IP) ([]byte, []byte, error) {
	block, _ := pem.Decode(caCertPEM)
	if block == nil {
		return nil, nil, errors.New("invalid CA certificate PEM")
	}
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, err
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:     dnsNames,
		IPAddresses:  ips,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return certPEM, keyPEM, nil
}

func indentPEM(value []byte) string {
	lines := strings.Split(strings.TrimRight(string(value), "\n"), "\n")
	for i := range lines {
		lines[i] = "    " + lines[i]
	}
	return strings.Join(lines, "\n")
}

func cnpgClusterManifest(cluster, version string) string {
	return fmt.Sprintf(`
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: %[1]s
  namespace: %[2]s
spec:
  instances: 1
  imageName: ghcr.io/cloudnative-pg/postgresql:%[3]s
  enableSuperuserAccess: true
  plugins:
    - name: pgdump-backup.cloudnative-pg.io
      enabled: true
  bootstrap:
    initdb:
      database: app
      owner: app
  storage:
    size: 1Gi
`, cluster, e2eNamespace, version)
}

func scheduledBackupManifest(name, cluster string) string {
	return fmt.Sprintf(`
apiVersion: postgresql.cnpg.io/v1
kind: ScheduledBackup
metadata:
  name: %[1]s
  namespace: %[2]s
spec:
  schedule: "0 0 23 31 12 *"
  immediate: true
  method: plugin
  pluginConfiguration:
    name: pgdump-backup.cloudnative-pg.io
    parameters:
      target_type: s3
      bucket_secret_name: backup-s3-credentials
      bucket_secret_key: bucket
      path: logical
      object_key_template: "{namespace}/{cluster}/{database}/{backup_id}.dump"
      retention_days: "30"
      endpoint_url_secret_name: backup-s3-credentials
      endpoint_url_secret_key: endpoint
      region_secret_name: backup-s3-credentials
      region_secret_key: region
      access_key_id_secret_name: backup-s3-credentials
      access_key_id_secret_key: access-key-id
      secret_access_key_secret_name: backup-s3-credentials
      secret_access_key_secret_key: secret-access-key
  cluster:
    name: %[3]s
`, name, e2eNamespace, cluster)
}

func parseVersions(value string) []string {
	var versions []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			versions = append(versions, part)
		}
	}
	return versions
}

func containsLine(output, line string) bool {
	for _, candidate := range strings.Split(output, "\n") {
		if strings.TrimSpace(candidate) == line {
			return true
		}
	}
	return false
}

func envDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envDefaultInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDefaultBool(key string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes":
		return true
	case "0", "false", "no":
		return false
	default:
		return fallback
	}
}

func detectContainerRuntime(ctx context.Context) (string, error) {
	configured := strings.TrimSpace(*containerRuntimeFlag)
	if configured != "" && configured != "auto" {
		if _, err := exec.LookPath(configured); err != nil {
			return "", fmt.Errorf("container runtime %q not found: %w", configured, err)
		}
		return configured, nil
	}

	for _, candidate := range []string{"docker", "podman"} {
		if _, err := exec.LookPath(candidate); err != nil {
			continue
		}
		if _, err := command(ctx, candidate, "info"); err == nil {
			return candidate, nil
		}
	}
	return "", errors.New("neither docker nor podman is available for image builds")
}

func repoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}
