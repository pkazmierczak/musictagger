package internal

import (
	"os"
	"path/filepath"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/shoenig/test/must"
)

func newTestProcessor(t *testing.T, musicLibrary string) *Processor {
	t.Helper()
	logger := log.New()
	logger.SetOutput(os.Stderr)
	logger.SetLevel(log.DebugLevel)
	config := DefaultConfig()
	return NewProcessor(config, musicLibrary, false, logger, nil)
}

func TestCleanupEmptyDir_RemovesEmptyDir(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "subdir")
	must.NoError(t, os.Mkdir(sub, 0755))

	p := newTestProcessor(t, root)
	must.NoError(t, p.cleanupEmptyDir(sub, root))

	_, err := os.Stat(sub)
	must.True(t, os.IsNotExist(err))
}

func TestCleanupEmptyDir_SkipsNonEmptyDir(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "subdir")
	must.NoError(t, os.Mkdir(sub, 0755))
	must.NoError(t, os.WriteFile(filepath.Join(sub, "file.txt"), []byte("data"), 0644))

	p := newTestProcessor(t, root)
	must.NoError(t, p.cleanupEmptyDir(sub, root))

	_, err := os.Stat(sub)
	must.NoError(t, err)
}

func TestCleanupEmptyDir_NeverRemovesRootDir(t *testing.T) {
	root := t.TempDir()

	p := newTestProcessor(t, root)

	// The root dir is empty, but cleanupEmptyDir must refuse to remove it
	must.NoError(t, p.cleanupEmptyDir(root, root))

	_, err := os.Stat(root)
	must.NoError(t, err)
}

func TestCleanupEmptyDir_NeverRemovesRootDir_RelativePaths(t *testing.T) {
	root := t.TempDir()

	// Use a path with a trailing component that resolves to the same directory
	dirWithDot := filepath.Join(root, "subdir", "..")

	p := newTestProcessor(t, root)
	must.NoError(t, p.cleanupEmptyDir(dirWithDot, root))

	_, err := os.Stat(root)
	must.NoError(t, err)
}

func TestCleanupEmptyDir_EmptyRootDir_AllowsSubdirRemoval(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "subdir")
	must.NoError(t, os.Mkdir(sub, 0755))

	p := newTestProcessor(t, root)

	// With empty rootDir string, the guard is skipped and empty dirs are removed
	must.NoError(t, p.cleanupEmptyDir(sub, ""))

	_, err := os.Stat(sub)
	must.True(t, os.IsNotExist(err))
}

func TestProcessFile_CleanupEmptyDirs_ProtectsWatchDir(t *testing.T) {
	// Setup: watchDir has a single music file directly in it (loose file)
	watchDir := t.TempDir()
	libraryDir := t.TempDir()

	// Create a minimal FLAC-like file with valid ID3 tags.
	// Since tag.ReadFrom will fail on a dummy file, we test via the
	// quarantine path which also triggers cleanupEmptyDirs.
	dummyFile := filepath.Join(watchDir, "notatag.bin")
	must.NoError(t, os.WriteFile(dummyFile, []byte("not a music file"), 0644))

	quarantineDir := filepath.Join(t.TempDir(), "quarantine")

	p := newTestProcessor(t, libraryDir)
	opts := ProcessorOptions{
		QuarantineDir:    quarantineDir,
		CleanupEmptyDirs: true,
		WatchDir:         watchDir,
	}

	// ProcessFile will quarantine the file (no valid tags), then attempt cleanup
	err := p.ProcessFile(dummyFile, opts)
	must.NoError(t, err)

	// The watch directory must still exist
	_, err = os.Stat(watchDir)
	must.NoError(t, err)
}

func TestCleanupEmptyDir_RemovesSubdir_ButNotWatchDir(t *testing.T) {
	// Simulates what happens after a file is moved out of a subdirectory
	// inside the watch directory: the subdir is removed but the watch dir stays.
	watchDir := t.TempDir()
	subDir := filepath.Join(watchDir, "album")
	must.NoError(t, os.Mkdir(subDir, 0755))

	p := newTestProcessor(t, watchDir)

	// Subdir is empty — should be removed
	must.NoError(t, p.cleanupEmptyDir(subDir, watchDir))
	_, err := os.Stat(subDir)
	must.True(t, os.IsNotExist(err))

	// Watch dir is now empty too — but must NOT be removed
	must.NoError(t, p.cleanupEmptyDir(watchDir, watchDir))
	_, err = os.Stat(watchDir)
	must.NoError(t, err)
}
