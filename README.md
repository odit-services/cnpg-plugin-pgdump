# CNPG pg_dump Backup Plugin

CNPG-I plugin for CloudNativePG v1.26+ that performs logical PostgreSQL backups with `pg_dump -Fc` and uploads one dump per database to S3.

The plugin uses `ReconcilerHooks.Pre` for `Backup` reconciliation. On success it returns `BEHAVIOR_TERMINATE`, so the CNPG operator skips its physical backup flow. On failure it logs/stores the error and returns `BEHAVIOR_CONTINUE`.

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
make e2e POSTGRES_VERSIONS=14,15,16,17
```

Equivalent direct command:

```sh
go test -tags=e2e ./test/e2e -count=1 -timeout=45m -postgres-versions="14,15,16,17"
```

## Runtime

The image is based on `postgres:16-alpine`, so it includes `pg_dump` 16. This is the pragmatic v1 approach; exact client/server version extraction from the cluster image is not implemented.

Configuration can be set with flags or environment variables:

- `--listen-address`, e.g. `:50051` for TCP or `unix:///plugins` for same-pod socket setups
- `PGDUMP_TIMEOUT`, default `12h`
- `PGDUMP_WORKDIR`, default OS temp dir

S3 configuration belongs to the `ScheduledBackup`. The plugin does not read S3 settings from Deployment environment variables.

## ScheduledBackup

Use the standard CNPG `ScheduledBackup` with `method: plugin`:

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

`bucket`, `path`, and `object_key_template` are configured per `ScheduledBackup`, so each CNPG cluster can use its own bucket or object layout. `path` is an optional prefix. `object_key_template` defaults to `{namespace}/{cluster}/{database}/{backup_id}.dump` and supports these placeholders:

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
