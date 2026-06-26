package backup

import "testing"

func TestObjectKeyUsesTemplate(t *testing.T) {
	key := ObjectKey("logical", "{cluster}/{database}/{backup_id}.dump", "app", "pg16", "postgres", "backup-1")
	if key != "logical/pg16/postgres/backup-1.dump" {
		t.Fatalf("key %q", key)
	}
}

func TestObjectKeySupportsTimestampPlaceholder(t *testing.T) {
	key := ObjectKey("logical", "{cluster}/{timestamp}/{database}/{backup_id}.dump", "app", "pg16", "postgres", "logical-pg16-20260626T120000Z")
	if key != "logical/pg16/20260626T120000Z/postgres/logical-pg16-20260626T120000Z.dump" {
		t.Fatalf("key %q", key)
	}
}

func TestDatabasePrefixUsesTemplateBeforeBackupID(t *testing.T) {
	prefix := DatabasePrefix("logical", "{namespace}/{cluster}/{database}/{backup_id}.dump", "app", "pg16", "postgres")
	if prefix != "logical/app/pg16/postgres/" {
		t.Fatalf("prefix %q", prefix)
	}
}
