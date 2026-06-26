package backup

import (
	"sync"
	"time"
)

type Result struct {
	BackupID          string    `json:"backup_id"`
	StartedAt         time.Time `json:"started_at"`
	StoppedAt         time.Time `json:"stopped_at"`
	LastBackupSize    int64     `json:"last_backup_size"`
	DatabasesBackedUp []string  `json:"databases_backed_up"`
	Objects           []string  `json:"objects"`
	ErrorMessage      string    `json:"error_message,omitempty"`
}

type Store struct {
	mu   sync.Mutex
	last map[string]Result
}

func (s *Store) Set(clusterKey string, result Result) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last == nil {
		s.last = map[string]Result{}
	}
	if result.BackupID == "" {
		delete(s.last, clusterKey)
		return
	}
	s.last[clusterKey] = result
}

func (s *Store) Last(clusterKey string) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last[clusterKey]
}
