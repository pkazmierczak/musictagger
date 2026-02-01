package internal

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shoenig/test/must"
)

func TestNewDaemonState(t *testing.T) {
	filePath := "/tmp/test-state.gob"
	state := NewDaemonState(filePath)

	must.NotNil(t, state)
	must.NotNil(t, state.ProcessedFiles)
	must.MapEmpty(t, state.ProcessedFiles)
	must.Eq(t, filePath, state.filePath)
	must.False(t, state.Stats.StartTime.IsZero())
}

func TestSaveAndLoad(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test-state.gob")

	// Create state with some data
	state := NewDaemonState(filePath)
	state.ProcessedFiles["test1.mp3"] = ProcessedFile{
		Path:        "test1.mp3",
		Hash:        "hash1",
		ProcessedAt: time.Now(),
		TargetPath:  "/target/test1.mp3",
		Success:     true,
	}
	state.ProcessedFiles["test2.mp3"] = ProcessedFile{
		Path:        "test2.mp3",
		Hash:        "hash2",
		ProcessedAt: time.Now(),
		TargetPath:  "/target/test2.mp3",
		Success:     false,
	}
	state.Stats.TotalProcessed = 10
	state.Stats.TotalSuccess = 8
	state.Stats.TotalFailed = 2

	// Save state
	must.NoError(t, state.Save())

	// Load state
	loaded, err := LoadState(filePath)
	must.NoError(t, err)

	// Verify loaded state
	must.MapLen(t, 2, loaded.ProcessedFiles)

	file1, ok := loaded.ProcessedFiles["test1.mp3"]
	must.True(t, ok)
	must.Eq(t, "hash1", file1.Hash)
	must.True(t, file1.Success)

	must.Eq(t, 10, loaded.Stats.TotalProcessed)
	must.Eq(t, 8, loaded.Stats.TotalSuccess)
	must.Eq(t, 2, loaded.Stats.TotalFailed)
}

func TestLoadStateNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "nonexistent.gob")

	// Load non-existent state should create new one
	state, err := LoadState(filePath)
	must.NoError(t, err)
	must.NotNil(t, state)
	must.MapEmpty(t, state.ProcessedFiles)
}

func TestLoadStateCorrupted(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "corrupted.gob")

	// Write corrupted data
	must.NoError(t, os.WriteFile(filePath, []byte("not a gob file"), 0644))

	// Load should return fresh state without error
	state, err := LoadState(filePath)
	must.NoError(t, err)
	must.NotNil(t, state)
	must.MapEmpty(t, state.ProcessedFiles)
}

func TestIsProcessed(t *testing.T) {
	state := NewDaemonState("/tmp/test.gob")

	// Add a processed file
	state.ProcessedFiles["test.mp3"] = ProcessedFile{
		Path: "test.mp3",
		Hash: "abc123",
	}

	tests := []struct {
		name     string
		path     string
		hash     string
		expected bool
	}{
		{"exact match", "test.mp3", "abc123", true},
		{"wrong hash", "test.mp3", "wrong", false},
		{"not found", "other.mp3", "abc123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := state.IsProcessed(tt.path, tt.hash)
			must.Eq(t, tt.expected, result)
		})
	}
}

func TestMarkProcessed(t *testing.T) {
	state := NewDaemonState("/tmp/test.gob")

	// Mark successful file
	state.MarkProcessed(ProcessedFile{
		Path:    "success.mp3",
		Hash:    "hash1",
		Success: true,
	})

	// Mark failed file
	state.MarkProcessed(ProcessedFile{
		Path:    "failed.mp3",
		Hash:    "hash2",
		Success: false,
	})

	// Verify state
	must.MapLen(t, 2, state.ProcessedFiles)
	must.Eq(t, 2, state.Stats.TotalProcessed)
	must.Eq(t, 1, state.Stats.TotalSuccess)
	must.Eq(t, 1, state.Stats.TotalFailed)

	// Verify files are in state
	must.MapContainsKey(t, state.ProcessedFiles, "success.mp3")
	must.MapContainsKey(t, state.ProcessedFiles, "failed.mp3")
}

func TestCleanup(t *testing.T) {
	state := NewDaemonState("/tmp/test.gob")

	now := time.Now()

	// Add old file (2 days ago)
	state.ProcessedFiles["old.mp3"] = ProcessedFile{
		Path:        "old.mp3",
		Hash:        "hash1",
		ProcessedAt: now.Add(-48 * time.Hour),
	}

	// Add recent file (1 hour ago)
	state.ProcessedFiles["recent.mp3"] = ProcessedFile{
		Path:        "recent.mp3",
		Hash:        "hash2",
		ProcessedAt: now.Add(-1 * time.Hour),
	}

	// Cleanup files older than 24 hours
	removed := state.Cleanup(24 * time.Hour)

	must.Eq(t, 1, removed)
	must.MapLen(t, 1, state.ProcessedFiles)
	must.MapNotContainsKey(t, state.ProcessedFiles, "old.mp3")
	must.MapContainsKey(t, state.ProcessedFiles, "recent.mp3")
}

func TestGetStats(t *testing.T) {
	state := NewDaemonState("/tmp/test.gob")

	state.Stats.TotalProcessed = 100
	state.Stats.TotalSuccess = 95
	state.Stats.TotalFailed = 5

	stats := state.GetStats()

	must.Eq(t, 100, stats.TotalProcessed)
	must.Eq(t, 95, stats.TotalSuccess)
	must.Eq(t, 5, stats.TotalFailed)
}

func TestGetProcessedCount(t *testing.T) {
	state := NewDaemonState("/tmp/test.gob")

	must.Eq(t, 0, state.GetProcessedCount())

	state.ProcessedFiles["test1.mp3"] = ProcessedFile{Path: "test1.mp3"}
	state.ProcessedFiles["test2.mp3"] = ProcessedFile{Path: "test2.mp3"}
	state.ProcessedFiles["test3.mp3"] = ProcessedFile{Path: "test3.mp3"}

	must.Eq(t, 3, state.GetProcessedCount())
}

func TestAtomicSave(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "state.gob")

	state := NewDaemonState(filePath)
	state.ProcessedFiles["test.mp3"] = ProcessedFile{Path: "test.mp3", Hash: "hash1"}

	// Save state
	must.NoError(t, state.Save())

	// Verify temp file was cleaned up
	tempFile := filePath + ".tmp"
	_, err := os.Stat(tempFile)
	must.Error(t, err)
	must.True(t, os.IsNotExist(err))

	// Verify actual file exists
	_, err = os.Stat(filePath)
	must.NoError(t, err)
}
