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
}

type DumpExecutor interface {
	ListDatabases(ctx context.Context, conn Connection) ([]string, error)
	Dump(ctx context.Context, conn Connection, database, backupID, workDir string) (string, int64, error)
}

type PGDumpExecutor struct {
	Binary  string
	Timeout time.Duration
}

func NewPGDumpExecutor(timeout time.Duration) *PGDumpExecutor {
	return &PGDumpExecutor{Binary: "pg_dump", Timeout: timeout}
}

func (e *PGDumpExecutor) ListDatabases(ctx context.Context, conn Connection) ([]string, error) {
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
	defer db.Close()

	rows, err := db.QueryContext(ctx, "SELECT datname FROM pg_database WHERE datallowconn AND NOT datistemplate ORDER BY datname")
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

func (e *PGDumpExecutor) Dump(ctx context.Context, conn Connection, database, backupID, workDir string) (string, int64, error) {
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		return "", 0, err
	}

	ctx, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()

	fileName := fmt.Sprintf("%s-%s.dump", backupID, sanitizeName(database))
	path := filepath.Join(workDir, fileName)
	cmd := exec.CommandContext(ctx, e.Binary,
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

func sanitizeName(value string) string {
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, "\\", "_")
	return value
}
