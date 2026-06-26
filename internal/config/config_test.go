package config

import "testing"

func TestParseBackupConfigDefaults(t *testing.T) {
	cfg, err := ParseBackupConfig(map[string]string{"bucket": "team-backups"}, Config{S3Region: "eu-central-1"})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.TargetType != "s3" {
		t.Fatalf("target type %q", cfg.TargetType)
	}
	if cfg.RetentionDays != 30 {
		t.Fatalf("retention %d", cfg.RetentionDays)
	}
	if cfg.Region != "eu-central-1" {
		t.Fatalf("region %q", cfg.Region)
	}
}

func TestParseBackupConfigRequiresBucket(t *testing.T) {
	if _, err := ParseBackupConfig(nil, Config{}); err == nil {
		t.Fatal("expected error")
	}
}
