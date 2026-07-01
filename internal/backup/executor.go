package backup

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type Connection struct {
	Host     string
	Port     int
	User     string
	Password string
	Major    string
}

type DumpExecutor interface {
	ServerMajor(ctx context.Context, conn Connection) (string, error)
	ListDatabases(ctx context.Context, conn Connection, skipInaccessible bool) ([]string, error)
	Dump(ctx context.Context, conn Connection, database, backupID, workDir string) (string, int64, error)
}

type PGDumpExecutor struct {
	BinaryTemplate string
	Timeout        time.Duration
}

const listDatabasesQuery = "SELECT datname FROM pg_database WHERE datallowconn AND NOT datistemplate ORDER BY datname"
const listAccessibleDatabasesQuery = "SELECT datname FROM pg_database WHERE datallowconn AND NOT datistemplate AND has_database_privilege(datname, 'CONNECT') ORDER BY datname"

func NewPGDumpExecutor(timeout time.Duration) *PGDumpExecutor {
	return &PGDumpExecutor{BinaryTemplate: "/usr/local/bin/pg_dump-%s", Timeout: timeout}
}

func (e *PGDumpExecutor) ServerMajor(ctx context.Context, conn Connection) (string, error) {
	db, err := openPostgres(ctx, conn)
	if err != nil {
		return "", err
	}
	defer db.Close()

	var versionNum int
	if err := db.QueryRowContext(ctx, "SHOW server_version_num").Scan(&versionNum); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", versionNum/10000), nil
}

func (e *PGDumpExecutor) ListDatabases(ctx context.Context, conn Connection, skipInaccessible bool) ([]string, error) {
	db, err := openPostgres(ctx, conn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := listDatabasesQuery
	if skipInaccessible {
		query = listAccessibleDatabasesQuery
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var database string
		if err := rows.Scan(&database); err != nil {
			return nil, err
		}
		databases = append(databases, database)
	}

	return databases, rows.Err()
}

func openPostgres(ctx context.Context, conn Connection) (*sql.DB, error) {
	dsn := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(conn.User, conn.Password),
		Host:   fmt.Sprintf("%s:%d", conn.Host, conn.Port),
		Path:   "postgres",
	}
	query := dsn.Query()
	query.Set("sslmode", "disable")
	dsn.RawQuery = query.Encode()

	db, err := sql.Open("postgres", dsn.String())
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func (e *PGDumpExecutor) Dump(ctx context.Context, conn Connection, database, backupID, workDir string) (string, int64, error) {
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		return "", 0, err
	}

	ctx, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()

	fileName := fmt.Sprintf("%s-%s.dump", backupID, sanitizeName(database))
	path := filepath.Join(workDir, fileName)
	binary, err := e.binaryForConnection(ctx, conn)
	if err != nil {
		return "", 0, err
	}

	cmd := exec.CommandContext(ctx, binary,
		"-Fc",
		"-h", conn.Host,
		"-p", fmt.Sprintf("%d", conn.Port),
		"-U", conn.User,
		"-d", database,
		"-f", path,
	)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+conn.Password)

	output, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(path)
		return "", 0, fmt.Errorf("pg_dump failed for database %s: %w: %s", database, err, strings.TrimSpace(string(output)))
	}

	stat, err := os.Stat(path)
	if err != nil {
		return "", 0, err
	}

	return path, stat.Size(), nil
}

func (e *PGDumpExecutor) binaryForConnection(ctx context.Context, conn Connection) (string, error) {
	major := conn.Major
	if major == "" {
		var err error
		major, err = e.ServerMajor(ctx, conn)
		if err != nil {
			return "", err
		}
	}
	binary := fmt.Sprintf(e.BinaryTemplate, major)
	if _, err := os.Stat(binary); err != nil {
		return "", fmt.Errorf("unsupported PostgreSQL major version %s: pg_dump binary %s is not available", major, binary)
	}
	return binary, nil
}

func sanitizeName(value string) string {
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, "\\", "_")
	return value
}
