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

// createID3v1File creates a minimal file with an ID3v1 tag that tag.ReadFrom
// can parse. ID3v1 is a 128-byte structure appended to the end of the file.
func createID3v1File(t *testing.T, path, title, artist, album string) {
	t.Helper()

	// ID3v1 tag: 128 bytes total
	buf := make([]byte, 128)
	copy(buf[0:3], "TAG")
	copy(buf[3:33], padRight(title, 30))
	copy(buf[33:63], padRight(artist, 30))
	copy(buf[63:93], padRight(album, 30))
	copy(buf[93:97], "2024")

	// Prepend some dummy audio data so the file is > 128 bytes
	// (tag.ReadFrom seeks to -128 from end)
	data := append(make([]byte, 128), buf...)
	must.NoError(t, os.WriteFile(path, data, 0644))
}

func padRight(s string, length int) string {
	if len(s) >= length {
		return s[:length]
	}
	return s + string(make([]byte, length-len(s)))
}

func TestProcessFile_CompanionFileMovedWithSiblingMusic(t *testing.T) {
	watchDir := t.TempDir()
	libraryDir := t.TempDir()
	albumDir := filepath.Join(watchDir, "album")
	must.NoError(t, os.Mkdir(albumDir, 0755))

	// Create a music file with ID3v1 tags
	createID3v1File(t, filepath.Join(albumDir, "track.mp3"), "Song", "Artist", "Album")

	// Create a companion file (PDF)
	companionFile := filepath.Join(albumDir, "liner_notes.pdf")
	must.NoError(t, os.WriteFile(companionFile, []byte("fake pdf"), 0644))

	p := newTestProcessor(t, libraryDir)
	opts := ProcessorOptions{}

	err := p.ProcessFile(companionFile, opts)
	must.NoError(t, err)

	// The companion file should have been moved to the library alongside music
	// Default pattern: {{artist}}-{{album}}/
	expectedDir := filepath.Join(libraryDir, "artist-album")
	expectedPath := filepath.Join(expectedDir, "liner_notes.pdf")

	content, err := os.ReadFile(expectedPath)
	must.NoError(t, err)
	must.Eq(t, "fake pdf", string(content))

	// Original should be gone
	_, err = os.Stat(companionFile)
	must.True(t, os.IsNotExist(err))
}

func TestProcessFile_CompanionFileQuarantinedWithoutSiblings(t *testing.T) {
	watchDir := t.TempDir()
	libraryDir := t.TempDir()

	// A lone non-music file with no sibling music files
	companionFile := filepath.Join(watchDir, "random.pdf")
	must.NoError(t, os.WriteFile(companionFile, []byte("fake pdf"), 0644))

	quarantineDir := filepath.Join(t.TempDir(), "quarantine")

	p := newTestProcessor(t, libraryDir)
	opts := ProcessorOptions{
		QuarantineDir: quarantineDir,
	}

	err := p.ProcessFile(companionFile, opts)
	must.NoError(t, err)

	// Should have been quarantined
	entries, err := os.ReadDir(quarantineDir)
	must.NoError(t, err)
	must.Eq(t, 1, len(entries))
	must.StrContains(t, entries[0].Name(), "random.pdf")
}

func TestProcessFile_CompanionFileErrorsWithoutSiblingsOrQuarantine(t *testing.T) {
	watchDir := t.TempDir()
	libraryDir := t.TempDir()

	companionFile := filepath.Join(watchDir, "random.pdf")
	must.NoError(t, os.WriteFile(companionFile, []byte("fake pdf"), 0644))

	p := newTestProcessor(t, libraryDir)
	opts := ProcessorOptions{}

	err := p.ProcessFile(companionFile, opts)
	must.Error(t, err)
	must.StrContains(t, err.Error(), "no sibling music files")
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
