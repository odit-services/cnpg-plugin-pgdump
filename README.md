# CNPG pg_dump Backup Plugin

CNPG-I plugin for CloudNativePG v1.26+ that performs logical PostgreSQL backups with `pg_dump -Fc` and uploads one dump per database to S3.

The plugin uses `ReconcilerHooks.Pre` for `Backup` reconciliation. On success it returns `BEHAVIOR_TERMINATE`, so the CNPG operator skips its physical backup flow. On failure it logs/stores the error and returns `BEHAVIOR_CONTINUE`.

## Supported PostgreSQL Versions

The image bundles `pg_dump` from these PostgreSQL major versions:

| Version | pg_dump binary |
|---------|----------------|
| 14      | `pg_dump-14`   |
| 15      | `pg_dump-15`   |
| 16      | `pg_dump-16`   |
| 17      | `pg_dump-17`   |
| 18      | `pg_dump-18`   |

At backup time the plugin reads `SHOW server_version_num` and selects the matching `pg_dump` binary — guaranteeing same-major restore compatibility. Unsupported versions fail with a clear error.

## Quickstart

Prerequisites:

- CloudNativePG v1.26+ is installed.
- `kubectl` is available locally.
- cert-manager is installed, or `openssl` is available for the manual fallback below.
- An S3-compatible bucket exists.

Configure the CNPG operator to discover this plugin:

```sh
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: cnpg-controller-manager-config
  namespace: cnpg-system
data:
  INCLUDE_PLUGINS: pgdump-backup.cloudnative-pg.io
EOF
kubectl -n cnpg-system rollout restart deployment/cnpg-controller-manager
kubectl -n cnpg-system rollout status deployment/cnpg-controller-manager
```

Create the CNPG-I mTLS Secrets referenced by `kubernetes/deployment.yaml`.

Recommended cert-manager setup:

```sh
kubectl apply -f - <<'EOF'
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: cnpg-plugin-pgdump-selfsigned
  namespace: cnpg-system
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: cnpg-plugin-pgdump-ca
  namespace: cnpg-system
spec:
  secretName: cnpg-plugin-pgdump-ca
  isCA: true
  commonName: cnpg-plugin-pgdump-ca
  duration: 8760h
  renewBefore: 720h
  issuerRef:
    name: cnpg-plugin-pgdump-selfsigned
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: cnpg-plugin-pgdump-ca
  namespace: cnpg-system
spec:
  ca:
    secretName: cnpg-plugin-pgdump-ca
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: cnpg-plugin-pgdump-server-tls
  namespace: cnpg-system
spec:
  secretName: cnpg-plugin-pgdump-server-tls
  duration: 2160h
  renewBefore: 360h
  issuerRef:
    name: cnpg-plugin-pgdump-ca
  commonName: cnpg-plugin-pgdump.cnpg-system.svc
  dnsNames:
    - cnpg-plugin-pgdump
    - cnpg-plugin-pgdump.cnpg-system
    - cnpg-plugin-pgdump.cnpg-system.svc
    - cnpg-plugin-pgdump.cnpg-system.svc.cluster.local
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: cnpg-plugin-pgdump-client-tls
  namespace: cnpg-system
spec:
  secretName: cnpg-plugin-pgdump-client-tls
  duration: 2160h
  renewBefore: 360h
  issuerRef:
    name: cnpg-plugin-pgdump-ca
  commonName: cnpg-plugin-pgdump-client
EOF
```

Manual OpenSSL fallback:

```sh
tmpdir="$(mktemp -d)"
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout "${tmpdir}/ca.key" \
  -out "${tmpdir}/ca.crt" \
  -subj "/CN=cnpg-plugin-pgdump-ca" \
  -days 365
openssl req -newkey rsa:2048 -nodes \
  -keyout "${tmpdir}/server.key" \
  -out "${tmpdir}/server.csr" \
  -subj "/CN=cnpg-plugin-pgdump.cnpg-system.svc"
printf "subjectAltName=DNS:cnpg-plugin-pgdump,DNS:cnpg-plugin-pgdump.cnpg-system,DNS:cnpg-plugin-pgdump.cnpg-system.svc,DNS:cnpg-plugin-pgdump.cnpg-system.svc.cluster.local" > "${tmpdir}/server.ext"
openssl x509 -req \
  -in "${tmpdir}/server.csr" \
  -CA "${tmpdir}/ca.crt" \
  -CAkey "${tmpdir}/ca.key" \
  -CAcreateserial \
  -out "${tmpdir}/server.crt" \
  -days 365 \
  -extfile "${tmpdir}/server.ext"
openssl req -newkey rsa:2048 -nodes \
  -keyout "${tmpdir}/client.key" \
  -out "${tmpdir}/client.csr" \
  -subj "/CN=cnpg-plugin-pgdump-client"
openssl x509 -req \
  -in "${tmpdir}/client.csr" \
  -CA "${tmpdir}/ca.crt" \
  -CAkey "${tmpdir}/ca.key" \
  -CAcreateserial \
  -out "${tmpdir}/client.crt" \
  -days 365
kubectl -n cnpg-system create secret generic cnpg-plugin-pgdump-server-tls \
  --type=kubernetes.io/tls \
  --from-file=tls.crt="${tmpdir}/server.crt" \
  --from-file=tls.key="${tmpdir}/server.key" \
  --from-file=ca.crt="${tmpdir}/ca.crt" \
  --dry-run=client -o yaml |
  kubectl apply -f -
kubectl -n cnpg-system create secret generic cnpg-plugin-pgdump-client-tls \
  --type=kubernetes.io/tls \
  --from-file=tls.crt="${tmpdir}/client.crt" \
  --from-file=tls.key="${tmpdir}/client.key" \
  --from-file=ca.crt="${tmpdir}/ca.crt" \
  --dry-run=client -o yaml |
  kubectl apply -f -
rm -rf "${tmpdir}"
```

Deploy the plugin from GHCR:

```sh
kubectl apply -f https://raw.githubusercontent.com/odit-services/cnpg-plugin-pgdump/main/kubernetes/deployment.yaml
kubectl -n cnpg-system set image deployment/cnpg-plugin-pgdump plugin=ghcr.io/odit-services/cnpg-plugin-pgdump:main
```

Use a SemVer tag instead of `main` for production, for example `ghcr.io/odit-services/cnpg-plugin-pgdump:1.2.3`. The GitHub Actions workflow publishes branch, SemVer, and SHA tags to GHCR.

Release pages also attach a rendered `cnpg-plugin-pgdump-deployment.yaml` and `cnpg-plugin-pgdump-quickstart.tar.gz`. The quickstart archive contains Kustomize, Flux, and Argo CD examples pinned to the release tag.

Enable the plugin on a CNPG cluster:

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: my-app-db
spec:
  instances: 3
  plugins:
    - name: pgdump-backup.cloudnative-pg.io
      enabled: true
  storage:
    size: 10Gi
```

Create S3 credentials and schedule a logical backup:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: logical-backup-s3
type: Opaque
stringData:
  endpoint: https://minio.platform.svc:9000
  access-key-id: minio
  secret-access-key: minio123
  region: us-east-1
---
apiVersion: postgresql.cnpg.io/v1
kind: ScheduledBackup
metadata:
  name: logical-daily-backup
spec:
  schedule: "0 0 2 * * *"
  method: plugin
  pluginConfiguration:
    name: pgdump-backup.cloudnative-pg.io
    parameters:
      target_type: s3
      bucket: team-backups
      path: logical
      retention_days: "30"
      endpoint_url_secret_name: logical-backup-s3
      region_secret_name: logical-backup-s3
      access_key_id_secret_name: logical-backup-s3
      secret_access_key_secret_name: logical-backup-s3
  cluster:
    name: my-app-db
```

Check status:

```sh
kubectl get scheduledbackup logical-daily-backup
kubectl get backup -l cnpg.io/scheduled-backup=logical-daily-backup
kubectl -n cnpg-system logs deployment/cnpg-plugin-pgdump
```

## Build

Using `make`:

```sh
make build
make test
make docker-build IMAGE=platform/cnpg-plugin-pgdump:latest
```

Using `task`:

```sh
task build
task test
task docker-build IMAGE=platform/cnpg-plugin-pgdump:latest
```

## E2E Tests

The Cucumber/Godog E2E suite creates a Kind cluster, installs CloudNativePG, runs RustFS as the S3 target, deploys the plugin image, and triggers a CNPG `ScheduledBackup` for each configured PostgreSQL major version.

Required local CLIs:

- `docker` or `podman`
- `kind`
- `kubectl`

The suite uses the Kind local-registry pattern on `localhost:5001` to make the locally built plugin image available to the cluster.

Run for one version:

```sh
make e2e POSTGRES_VERSIONS=16
```

Equivalent Taskfile command:

```sh
task e2e POSTGRES_VERSIONS=16
```

Run for multiple versions:

```sh
make e2e POSTGRES_VERSIONS=14,15,16,17,18
```

Run multiple versions concurrently:

```sh
make e2e POSTGRES_VERSIONS=14,15,16 E2E_PARALLELISM=3
```

Equivalent direct command:

```sh
go test -tags=e2e ./test/e2e -count=1 -timeout=45m -postgres-versions="14,15,16,17,18" -parallelism=2
```

## Runtime

See [Supported PostgreSQL Versions](#supported-postgresql-versions) for the bundled `pg_dump` binaries.

Configuration can be set with flags or environment variables:

- `--listen-address`, e.g. `:50051` for TCP or `unix:///plugins` for same-pod socket setups
- `PGDUMP_TIMEOUT`, default `12h`
- `PGDUMP_WORKDIR`, default OS temp dir

S3 configuration belongs to the `ScheduledBackup`. The plugin does not read S3 settings from Deployment environment variables.

## Backup (On-Demand)

A one-time ad-hoc backup uses the standard CNPG `Backup` resource with `method: plugin`:

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Backup
metadata:
  name: logical-manual-backup
spec:
  method: plugin
  pluginConfiguration:
    name: pgdump-backup.cloudnative-pg.io
    parameters:
      target_type: s3
      bucket: team-backups
      path: logical
      retention_days: "30"
      endpoint_url_secret_name: logical-backup-s3
      region_secret_name: logical-backup-s3
      access_key_id_secret_name: logical-backup-s3
      secret_access_key_secret_name: logical-backup-s3
  cluster:
    name: my-app-db
```

The S3 secret and cluster plugin config from the quickstart apply here as well.

## ScheduledBackup

For recurring backups use the standard CNPG `ScheduledBackup` with `method: plugin`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: logical-backup-s3
type: Opaque
stringData:
  endpoint: https://minio.platform.svc:9000
  access-key-id: minio
  secret-access-key: minio123
  region: us-east-1
---
apiVersion: postgresql.cnpg.io/v1
kind: ScheduledBackup
metadata:
  name: logical-daily-backup
spec:
  schedule: "0 0 2 * * *"
  method: plugin
  pluginConfiguration:
    name: pgdump-backup.cloudnative-pg.io
    parameters:
      target_type: s3
      bucket: team-backups
      path: logical
      object_key_template: "{namespace}/{cluster}/{database}/{backup_id}.dump"
      retention_days: "30"
      endpoint_url_secret_name: logical-backup-s3
      endpoint_url_secret_key: endpoint
      region_secret_name: logical-backup-s3
      region_secret_key: region
      access_key_id_secret_name: logical-backup-s3
      access_key_id_secret_key: access-key-id
      secret_access_key_secret_name: logical-backup-s3
      secret_access_key_secret_key: secret-access-key
  cluster:
    name: my-app-db
```

Secret ref parameter defaults:

- `endpoint_url_secret_key`: `endpoint`
- `region_secret_key`: `region`
- `access_key_id_secret_key`: `access-key-id`
- `secret_access_key_secret_key`: `secret-access-key`

The bucket can be set directly via `bucket` or read from a Kubernetes Secret via `bucket_secret_name` / `bucket_secret_key` (default key: `bucket`). This is useful for tools like [s3ops](https://github.com/odit-services/s3ops) that manage per-service S3 configuration. When both are specified, the secret value takes precedence.

`path` and `object_key_template` are configured per `ScheduledBackup`, so each CNPG cluster can use its own bucket or object layout. `path` is an optional prefix. `object_key_template` defaults to `{namespace}/{cluster}/{database}/{backup_id}.dump` and supports these placeholders:

- `{namespace}`
- `{cluster}`
- `{database}`
- `{backup_id}`
- `{timestamp}`

The template must include `{database}` and `{backup_id}` to avoid overwriting dumps. For a bucket dedicated to one CNPG cluster, a compact layout can be:

```yaml
parameters:
  bucket: my-app-db-backups
  path: logical
  object_key_template: "{database}/{timestamp}/{backup_id}.dump"
```

Object keys are written as:

```text
<path>/<rendered-object-key-template>
```

## Notes

- The plugin connects through the CNPG read service: `<cluster>-r.<namespace>.svc:5432`.
- Credentials are read from the CNPG application secret: `<cluster>-app`.
- Databases are discovered with `SELECT datname FROM pg_database WHERE datallowconn AND NOT datistemplate`.
- Retention deletes objects whose backup timestamp in the filename is older than `retention_days`.
