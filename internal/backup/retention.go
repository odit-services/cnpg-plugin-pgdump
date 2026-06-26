package backup

import (
	"context"
	"path"
	"strings"
	"time"
)

func ApplyRetention(ctx context.Context, uploader Uploader, prefix string, retentionDays int, now time.Time) error {
	if retentionDays <= 0 {
		return nil
	}

	objects, err := uploader.List(ctx, prefix)
	if err != nil {
		return err
	}

	cutoff := now.AddDate(0, 0, -retentionDays)
	for _, object := range objects {
		timestamp, ok := timestampFromKey(object.Key)
		if !ok || !timestamp.Before(cutoff) {
			continue
		}
		if err := uploader.Delete(ctx, object.Key); err != nil {
			return err
		}
	}

	return nil
}

func timestampFromKey(key string) (time.Time, bool) {
	base := strings.TrimSuffix(path.Base(key), ".dump")
	parts := strings.Split(base, "-")
	if len(parts) < 2 {
		return time.Time{}, false
	}

	stamp := parts[len(parts)-1]
	parsed, err := time.Parse("20060102T150405Z", stamp)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}
