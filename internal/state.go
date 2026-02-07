package internal

import (
	"sync"
	"time"
)

// DaemonState tracks in-memory stats and known files for the daemon.
// It does not persist to disk — stats reset on restart.
type DaemonState struct {
	mu    sync.RWMutex
	known map[string]struct{} // set of file paths seen this session
	Stats ProcessingStats
}

// ProcessingStats tracks processing counters
type ProcessingStats struct {
	TotalProcessed int
	TotalSuccess   int
	TotalFailed    int
	StartTime      time.Time
}

// NewDaemonState creates a new in-memory state
func NewDaemonState() *DaemonState {
	return &DaemonState{
		known: make(map[string]struct{}),
		Stats: ProcessingStats{
			StartTime: time.Now(),
		},
	}
}

// IsKnown returns true if the file path has been processed in this session
func (s *DaemonState) IsKnown(filePath string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.known[filePath]
	return ok
}

// MarkProcessed records that a file was processed and updates stats
func (s *DaemonState) MarkProcessed(filePath string, success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.known[filePath] = struct{}{}
	s.Stats.TotalProcessed++
	if success {
		s.Stats.TotalSuccess++
	} else {
		s.Stats.TotalFailed++
	}
}

// GetStats returns a copy of the current stats
func (s *DaemonState) GetStats() ProcessingStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Stats
}
