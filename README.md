# CNPG pg_dump Backup Plugin

CNPG-I plugin for CloudNativePG v1.26+ that performs logical PostgreSQL backups with `pg_dump -Fc` and uploads one dump per database to S3.

The plugin uses `ReconcilerHooks.Pre` for `Backup` reconciliation. On success it returns `BEHAVIOR_TERMINATE`, so the CNPG operator skips its physical backup flow. On failure it logs/stores the error and returns `BEHAVIOR_CONTINUE`.

## Build

```sh
make build
make test
make docker-build IMAGE=platform/cnpg-plugin-pgdump:latest
```

## E2E Tests

The Cucumber/Godog E2E suite creates a Kind cluster, installs CloudNativePG, runs RustFS as the S3 target, deploys the plugin image, and triggers a CNPG `ScheduledBackup` for each configured PostgreSQL major version.

Required local CLIs:

- `docker`
- `kind`
- `kubectl`

Run for one version:

```sh
make e2e POSTGRES_VERSIONS=16
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
- `S3_ENDPOINT` / `--s3-endpoint`
- `S3_REGION` / `--s3-region`
- `S3_ACCESS_KEY_ID` / `--s3-access-key-id`
- `S3_SECRET_ACCESS_KEY` / `--s3-secret-access-key`
- `PGDUMP_TIMEOUT`, default `12h`
- `PGDUMP_WORKDIR`, default OS temp dir

## ScheduledBackup

Use the standard CNPG `ScheduledBackup` with `method: plugin`:

```yaml
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
      endpoint_url: https://minio.platform.svc:9000
      region: us-east-1
  cluster:
    name: my-app-db
```

Object keys are written as:

```text
<path>/<namespace>/<cluster-name>/<db-name>/<backup-name>-<timestamp>.dump
```

## Notes

- The plugin connects through the CNPG RW service: `<cluster>-rw.<namespace>.svc:5432`.
- Credentials are read from the CNPG application secret: `<cluster>-app`.
- Databases are discovered with `SELECT datname FROM pg_database WHERE datallowconn AND NOT datistemplate`.
- Retention deletes objects whose backup timestamp in the filename is older than `retention_days`.
