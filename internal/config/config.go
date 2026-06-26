package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	PluginName        = "pgdump-backup.cloudnative-pg.io"
	DefaultRegion     = "us-east-1"
	DefaultTargetType = "s3"
	DefaultRetention  = 30
)

type Config struct {
	Version           string
	S3Endpoint        string
	S3Region          string
	S3AccessKeyID     string
	S3SecretAccessKey string
	DumpTimeout       time.Duration
	WorkDir           string
}

type BackupConfig struct {
	TargetType    string
	Bucket        string
	Path          string
	RetentionDays int
	EndpointURL   string
	Region        string
}

func FromEnv(version string) Config {
	dumpTimeout := 12 * time.Hour
	if value := os.Getenv("PGDUMP_TIMEOUT"); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			dumpTimeout = parsed
		}
	}

	workDir := os.Getenv("PGDUMP_WORKDIR")
	if workDir == "" {
		workDir = os.TempDir()
	}

	region := os.Getenv("S3_REGION")
	if region == "" {
		region = DefaultRegion
	}

	return Config{
		Version:           version,
		S3Endpoint:        os.Getenv("S3_ENDPOINT"),
		S3Region:          region,
		S3AccessKeyID:     os.Getenv("S3_ACCESS_KEY_ID"),
		S3SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"),
		DumpTimeout:       dumpTimeout,
		WorkDir:           workDir,
	}
}

func ParseBackupConfig(parameters map[string]string, defaults Config) (BackupConfig, error) {
	retention := DefaultRetention
	if value := parameters["retention_days"]; value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			return BackupConfig{}, errInvalidParameter("retention_days", value)
		}
		retention = parsed
	}

	cfg := BackupConfig{
		TargetType:    withDefault(parameters["target_type"], DefaultTargetType),
		Bucket:        parameters["bucket"],
		Path:          strings.Trim(parameters["path"], "/"),
		RetentionDays: retention,
		EndpointURL:   withDefault(parameters["endpoint_url"], defaults.S3Endpoint),
		Region:        withDefault(parameters["region"], defaults.S3Region),
	}

	if cfg.TargetType != DefaultTargetType {
		return BackupConfig{}, errInvalidParameter("target_type", cfg.TargetType)
	}
	if cfg.Bucket == "" {
		return BackupConfig{}, errMissingParameter("bucket")
	}

	return cfg, nil
}

type parameterError string

func (e parameterError) Error() string { return string(e) }

func errMissingParameter(name string) error {
	return parameterError("missing required parameter: " + name)
}

func errInvalidParameter(name, value string) error {
	return parameterError("invalid parameter " + name + ": " + value)
}

func withDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
