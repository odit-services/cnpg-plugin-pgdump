package backup

import (
	"context"
	"testing"
	"time"
)

type fakeUploader struct {
	objects []Object
	deleted []string
}

func (u *fakeUploader) Upload(context.Context, string, string) (int64, error) { return 0, nil }
func (u *fakeUploader) List(context.Context, string) ([]Object, error)        { return u.objects, nil }
func (u *fakeUploader) Delete(_ context.Context, key string) error {
	u.deleted = append(u.deleted, key)
	return nil
}

func TestApplyRetentionDeletesBackupsOlderThanRetentionDays(t *testing.T) {
	uploader := &fakeUploader{objects: []Object{
		{Key: "logical/ns/cluster/app/backup-20260101T000000Z.dump"},
		{Key: "logical/ns/cluster/app/backup-20260620T000000Z.dump"},
		{Key: "logical/ns/cluster/app/not-a-backup.dump"},
	}}
	now := time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC)

	if err := ApplyRetention(context.Background(), uploader, "logical/ns/cluster/app", 30, now); err != nil {
		t.Fatal(err)
	}

	if len(uploader.deleted) != 1 {
		t.Fatalf("deleted %d objects, want 1", len(uploader.deleted))
	}
	if uploader.deleted[0] != "logical/ns/cluster/app/backup-20260101T000000Z.dump" {
		t.Fatalf("deleted %q", uploader.deleted[0])
	}
}
