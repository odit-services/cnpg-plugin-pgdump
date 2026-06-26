package config

import "testing"

func TestParseBackupConfigDefaults(t *testing.T) {
	cfg, err := ParseBackupConfig(map[string]string{"bucket": "team-backups"}, Config{})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.TargetType != "s3" {
		t.Fatalf("target type %q", cfg.TargetType)
	}
	if cfg.RetentionDays != 30 {
		t.Fatalf("retention %d", cfg.RetentionDays)
	}
	if cfg.Region != "us-east-1" {
		t.Fatalf("region %q", cfg.Region)
	}
	if cfg.ObjectKeyTemplate != DefaultObjectKeyTemplate {
		t.Fatalf("object key template %q", cfg.ObjectKeyTemplate)
	}
}

func TestParseBackupConfigRegionParameter(t *testing.T) {
	cfg, err := ParseBackupConfig(map[string]string{"bucket": "team-backups", "region": "eu-central-1"}, Config{})
	if err != nil {
		t.Fatal(err)
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

func TestParseBackupConfigValidatesObjectKeyTemplate(t *testing.T) {
	for _, template := range []string{"{cluster}/{backup_id}.dump", "{cluster}/{database}.dump"} {
		if _, err := ParseBackupConfig(map[string]string{
			"bucket":              "team-backups",
			"object_key_template": template,
		}, Config{}); err == nil {
			t.Fatalf("expected error for template %q", template)
		}
	}
}

func TestParseBackupConfigSecretRefs(t *testing.T) {
	cfg, err := ParseBackupConfig(map[string]string{
		"bucket":                        "team-backups",
		"access_key_id_secret_name":     "backup-s3-credentials",
		"secret_access_key_secret_name": "backup-s3-credentials",
		"endpoint_url_secret_name":      "backup-s3-credentials",
		"endpoint_url_secret_key":       "custom-endpoint",
		"region_secret_name":            "backup-s3-credentials",
	}, Config{})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.AccessKeyIDSecret.Name != "backup-s3-credentials" || cfg.AccessKeyIDSecret.Key != "access-key-id" {
		t.Fatalf("access key secret ref %#v", cfg.AccessKeyIDSecret)
	}
	if cfg.SecretAccessKeySecret.Name != "backup-s3-credentials" || cfg.SecretAccessKeySecret.Key != "secret-access-key" {
		t.Fatalf("secret key secret ref %#v", cfg.SecretAccessKeySecret)
	}
	if cfg.EndpointURLSecret.Name != "backup-s3-credentials" || cfg.EndpointURLSecret.Key != "custom-endpoint" {
		t.Fatalf("endpoint secret ref %#v", cfg.EndpointURLSecret)
	}
	if cfg.RegionSecret.Name != "backup-s3-credentials" || cfg.RegionSecret.Key != "region" {
		t.Fatalf("region secret ref %#v", cfg.RegionSecret)
	}
}
