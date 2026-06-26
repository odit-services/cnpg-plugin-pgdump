package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBinaryForConnectionUsesServerMajor(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "pg_dump-17")
	if err := createExecutable(binary); err != nil {
		t.Fatal(err)
	}

	executor := &PGDumpExecutor{BinaryTemplate: filepath.Join(dir, "pg_dump-%s"), Timeout: time.Minute}
	got, err := executor.binaryForConnection(context.Background(), Connection{Major: "17"})
	if err != nil {
		t.Fatal(err)
	}
	if got != binary {
		t.Fatalf("binary %q", got)
	}
}

func TestBinaryForConnectionRejectsUnavailableMajor(t *testing.T) {
	executor := &PGDumpExecutor{BinaryTemplate: filepath.Join(t.TempDir(), "pg_dump-%s"), Timeout: time.Minute}
	if _, err := executor.binaryForConnection(context.Background(), Connection{Major: "19"}); err == nil {
		t.Fatal("expected error")
	}
}

func createExecutable(path string) error {
	return os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755)
}
