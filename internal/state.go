package internal

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// DaemonState manages the state of the daemon including processed files
type DaemonState struct {
	ProcessedFiles map[string]ProcessedFile
	LastRun        time.Time
	Stats          ProcessingStats
	mutex          sync.RWMutex
	filePath       string
}

// ProcessedFile represents metadata about a processed file
type ProcessedFile struct {
	Path        string
	Hash        string
	ProcessedAt time.Time
	TargetPath  string
	Success     bool
}

// ProcessingStats tracks daemon statistics
type ProcessingStats struct {
	TotalProcessed int
	TotalSuccess   int
	TotalFailed    int
	StartTime      time.Time
}

// NewDaemonState creates a new empty state
func NewDaemonState(filePath string) *DaemonState {
	return &DaemonState{
		ProcessedFiles: make(map[string]ProcessedFile),
		LastRun:        time.Now(),
		Stats: ProcessingStats{
			StartTime: time.Now(),
		},
		filePath: filePath,
	}
}

// LoadState loads state from disk, or creates new state if file doesn't exist
func LoadState(filePath string) (*DaemonState, error) {
	// Try to open existing state file
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Infof("no existing state file at %s, starting with fresh state", filePath)
			return NewDaemonState(filePath), nil
		}
		return nil, fmt.Errorf("failed to open state file: %w", err)
	}
	defer file.Close()

	var state DaemonState
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&state); err != nil {
		log.Warnf("corrupted state file at %s, starting with fresh state: %v", filePath, err)
		return NewDaemonState(filePath), nil
	}

	state.filePath = filePath
	state.LastRun = time.Now()

	log.Infof("loaded state from %s (%d processed files)", filePath, len(state.ProcessedFiles))
	return &state, nil
}

// Save persists state to disk using atomic write (temp file + rename)
func (s *DaemonState) Save() error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Ensure directory exists
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Write to temporary file
	tempFile := s.filePath + ".tmp"
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp state file: %w", err)
	}

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(s); err != nil {
		file.Close()
		os.Remove(tempFile)
		return fmt.Errorf("failed to encode state: %w", err)
	}

	if err := file.Close(); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to close temp state file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, s.filePath); err != nil {
		os.Remove(tempFile) // Clean up temp file on error
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	return nil
}

// IsProcessed checks if a file has already been processed
func (s *DaemonState) IsProcessed(filePath string, hash string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	processed, exists := s.ProcessedFiles[filePath]
	if !exists {
		return false
	}

	// Check if hash matches (file might have changed)
	return processed.Hash == hash
}

// MarkProcessed records that a file has been processed
func (s *DaemonState) MarkProcessed(file ProcessedFile) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.ProcessedFiles[file.Path] = file
	s.Stats.TotalProcessed++
	if file.Success {
		s.Stats.TotalSuccess++
	} else {
		s.Stats.TotalFailed++
	}
}

// Cleanup removes old entries from the state
func (s *DaemonState) Cleanup(maxAge time.Duration) int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for path, file := range s.ProcessedFiles {
		if file.ProcessedAt.Before(cutoff) {
			delete(s.ProcessedFiles, path)
			removed++
		}
	}

	if removed > 0 {
		log.Infof("cleaned up %d old entries from state (older than %v)", removed, maxAge)
	}

	return removed
}

// GetStats returns current statistics
func (s *DaemonState) GetStats() ProcessingStats {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.Stats
}

// GetProcessedCount returns the number of processed files in state
func (s *DaemonState) GetProcessedCount() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return len(s.ProcessedFiles)
}
