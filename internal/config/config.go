package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	PluginName               = "pgdump-backup.cloudnative-pg.io"
	DefaultRegion            = "us-east-1"
	DefaultTargetType        = "s3"
	DefaultRetention         = 30
	DefaultObjectKeyTemplate = "{namespace}/{cluster}/{database}/{backup_id}.dump"
	DefaultBackupUser        = "app"
	DefaultSkipInaccessible  = false
)

type Config struct {
	Version     string
	DumpTimeout time.Duration
	WorkDir     string
	// SidecarImage is the image of the backup sidecar injected in the PostgreSQL pods.
	SidecarImage string
	// SidecarResources holds the default requests and limits of the sidecar, keyed by Cluster plugin parameter.
	SidecarResources map[string]string
}

// SidecarResourceParameters maps the Cluster plugin parameters setting the sidecar requests and limits to the
// environment variables of the plugin holding their defaults.
var SidecarResourceParameters = map[string]string{
	"sidecar_cpu_request":    "SIDECAR_CPU_REQUEST",
	"sidecar_memory_request": "SIDECAR_MEMORY_REQUEST",
	"sidecar_cpu_limit":      "SIDECAR_CPU_LIMIT",
	"sidecar_memory_limit":   "SIDECAR_MEMORY_LIMIT",
}

type BackupConfig struct {
	TargetType            string
	Bucket                string
	Path                  string
	ObjectKeyTemplate     string
	RetentionDays         int
	EndpointURL           string
	Region                string
	AccessKeyID           string
	SecretAccessKey       string
	AccessKeyIDSecret     SecretKeyRef
	SecretAccessKeySecret SecretKeyRef
	EndpointURLSecret     SecretKeyRef
	RegionSecret          SecretKeyRef
	BucketSecret          SecretKeyRef
	BackupUser            string
	SkipInaccessible      bool
}

type SecretKeyRef struct {
	Name string
	Key  string
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

	sidecarResources := map[string]string{}
	for parameter, env := range SidecarResourceParameters {
		if value := os.Getenv(env); value != "" {
			sidecarResources[parameter] = value
		}
	}

	return Config{
		Version:          version,
		DumpTimeout:      dumpTimeout,
		WorkDir:          workDir,
		SidecarImage:     os.Getenv("SIDECAR_IMAGE"),
		SidecarResources: sidecarResources,
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
	skipInaccessible := DefaultSkipInaccessible
	if value := parameters["skip_inaccessible_databases"]; value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return BackupConfig{}, errInvalidParameter("skip_inaccessible_databases", value)
		}
		skipInaccessible = parsed
	}

	cfg := BackupConfig{
		TargetType:            withDefault(parameters["target_type"], DefaultTargetType),
		Bucket:                parameters["bucket"],
		Path:                  strings.Trim(parameters["path"], "/"),
		ObjectKeyTemplate:     strings.Trim(parameters["object_key_template"], "/"),
		RetentionDays:         retention,
		EndpointURL:           parameters["endpoint_url"],
		Region:                withDefault(parameters["region"], DefaultRegion),
		AccessKeyIDSecret:     secretRef(parameters, "access_key_id_secret", "access-key-id"),
		SecretAccessKeySecret: secretRef(parameters, "secret_access_key_secret", "secret-access-key"),
		EndpointURLSecret:     secretRef(parameters, "endpoint_url_secret", "endpoint"),
		RegionSecret:          secretRef(parameters, "region_secret", "region"),
		BucketSecret:          secretRef(parameters, "bucket_secret", "bucket"),
		BackupUser:            withDefault(parameters["backup_user"], DefaultBackupUser),
		SkipInaccessible:      skipInaccessible,
	}

	if cfg.TargetType != DefaultTargetType {
		return BackupConfig{}, errInvalidParameter("target_type", cfg.TargetType)
	}
	if cfg.Bucket == "" && cfg.BucketSecret.Name == "" {
		return BackupConfig{}, errMissingParameter("bucket")
	}
	if cfg.ObjectKeyTemplate == "" {
		cfg.ObjectKeyTemplate = DefaultObjectKeyTemplate
	}
	if !strings.Contains(cfg.ObjectKeyTemplate, "{database}") {
		return BackupConfig{}, errInvalidParameter("object_key_template", cfg.ObjectKeyTemplate)
	}
	if !strings.Contains(cfg.ObjectKeyTemplate, "{backup_id}") {
		return BackupConfig{}, errInvalidParameter("object_key_template", cfg.ObjectKeyTemplate)
	}

	return cfg, nil
}

func secretRef(parameters map[string]string, prefix, defaultKey string) SecretKeyRef {
	return SecretKeyRef{
		Name: parameters[prefix+"_name"],
		Key:  withDefault(parameters[prefix+"_key"], defaultKey),
	}
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
