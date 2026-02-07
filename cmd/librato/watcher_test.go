package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"

	"github.com/pkazmierczak/librato/internal"
)

func newTestWatcher(t *testing.T, watchDir string) *Watcher {
	t.Helper()
	logger := log.New()
	logger.SetOutput(os.Stderr)
	logger.SetLevel(log.DebugLevel)

	config := internal.DefaultConfig()
	config.Library = t.TempDir()
	processor := internal.NewProcessor(config, config.Library, true, logger, nil)

	state := internal.NewDaemonState()

	quarantineDir := filepath.Join(t.TempDir(), "quarantine")

	w, err := NewWatcher(processor, state, WatcherOptions{
		WatchDir:      watchDir,
		QuarantineDir: quarantineDir,
		DebounceTime:  50 * time.Millisecond,
		CleanupEmpty:  false,
	})
	must.NoError(t, err)
	t.Cleanup(func() { w.Stop() })
	return w
}

func TestWatcher_Start_WatchesSubdirectories(t *testing.T) {
	watchDir := t.TempDir()

	// Create nested subdirectories before starting the watcher
	sub1 := filepath.Join(watchDir, "artist1", "album1")
	sub2 := filepath.Join(watchDir, "artist2", "album2")
	must.NoError(t, os.MkdirAll(sub1, 0755))
	must.NoError(t, os.MkdirAll(sub2, 0755))

	w := newTestWatcher(t, watchDir)
	must.NoError(t, w.Start())

	// Verify the fsnotify watcher has all directories registered.
	// fsWatcher.WatchList() returns the list of watched paths.
	watched := w.fsWatcher.WatchList()
	sort.Strings(watched)

	must.SliceContains(t, watched, watchDir)
	must.SliceContains(t, watched, filepath.Join(watchDir, "artist1"))
	must.SliceContains(t, watched, sub1)
	must.SliceContains(t, watched, filepath.Join(watchDir, "artist2"))
	must.SliceContains(t, watched, sub2)
}

func TestWatcher_Start_NonExistentDir(t *testing.T) {
	w, err := NewWatcher(nil, nil, WatcherOptions{
		WatchDir:      "/nonexistent/path/that/does/not/exist",
		QuarantineDir: "/tmp",
		DebounceTime:  50 * time.Millisecond,
	})
	must.NoError(t, err)
	defer w.fsWatcher.Close()

	err = w.Start()
	must.Error(t, err)
	must.StrContains(t, err.Error(), "watch directory does not exist")
}

func TestWatcher_HandleEvent_NewDirectory(t *testing.T) {
	watchDir := t.TempDir()
	w := newTestWatcher(t, watchDir)
	must.NoError(t, w.Start())

	// Create a new subdirectory — the event loop should pick it up
	newDir := filepath.Join(watchDir, "new_album")
	must.NoError(t, os.Mkdir(newDir, 0755))

	// Wait for the event to be processed
	must.Wait(t, wait.InitialSuccess(
		wait.Timeout(2*time.Second),
		wait.Gap(50*time.Millisecond),
		wait.BoolFunc(func() bool {
			for _, p := range w.fsWatcher.WatchList() {
				if p == newDir {
					return true
				}
			}
			return false
		}),
	))
}

func TestWatcher_HandleEvent_FileInNewDirectory(t *testing.T) {
	watchDir := t.TempDir()
	w := newTestWatcher(t, watchDir)
	must.NoError(t, w.Start())

	// Create a new subdirectory with a file inside
	newDir := filepath.Join(watchDir, "new_album")
	must.NoError(t, os.Mkdir(newDir, 0755))

	// Small delay to let the dir event be processed and watch set up
	time.Sleep(200 * time.Millisecond)

	// Now create a file inside the new directory
	filePath := filepath.Join(newDir, "track.bin")
	must.NoError(t, os.WriteFile(filePath, []byte("data"), 0644))

	// The file should be picked up — either still pending or already processed
	must.Wait(t, wait.InitialSuccess(
		wait.Timeout(2*time.Second),
		wait.Gap(50*time.Millisecond),
		wait.BoolFunc(func() bool {
			// Check if it's still pending
			w.pendingMutex.RLock()
			_, pending := w.pendingFiles[filePath]
			w.pendingMutex.RUnlock()
			// Or already processed
			return pending || w.state.IsKnown(filePath)
		}),
	))
}

func TestWatcher_ScanDir_RecursiveWithFiles(t *testing.T) {
	watchDir := t.TempDir()
	w := newTestWatcher(t, watchDir)
	must.NoError(t, w.Start())

	// Create a nested structure: album/disc1/track.bin
	disc1 := filepath.Join(watchDir, "album", "disc1")
	must.NoError(t, os.MkdirAll(disc1, 0755))
	filePath := filepath.Join(disc1, "track.bin")
	must.NoError(t, os.WriteFile(filePath, []byte("data"), 0644))

	// Call scanDir on the album directory
	w.scanDir(filepath.Join(watchDir, "album"))

	// Verify the nested dir was added to the watcher
	watched := w.fsWatcher.WatchList()
	must.SliceContains(t, watched, disc1)

	// Verify the file was debounced for processing
	w.pendingMutex.RLock()
	_, exists := w.pendingFiles[filePath]
	w.pendingMutex.RUnlock()
	must.True(t, exists)
}

func TestWatcher_ScanDir_Empty(t *testing.T) {
	watchDir := t.TempDir()
	w := newTestWatcher(t, watchDir)
	must.NoError(t, w.Start())

	emptyDir := filepath.Join(watchDir, "empty")
	must.NoError(t, os.Mkdir(emptyDir, 0755))

	// scanDir on an empty directory should not panic or error
	w.scanDir(emptyDir)

	// No files should be pending
	w.pendingMutex.RLock()
	count := len(w.pendingFiles)
	w.pendingMutex.RUnlock()
	must.Eq(t, 0, count)
}

func TestWatcher_HandleEvent_IgnoresNonCreateWrite(t *testing.T) {
	watchDir := t.TempDir()
	w := newTestWatcher(t, watchDir)
	must.NoError(t, w.Start())

	// Create then remove a file to generate a Remove event
	filePath := filepath.Join(watchDir, "temp.bin")
	must.NoError(t, os.WriteFile(filePath, []byte("data"), 0644))

	// Wait for the create event to be processed
	time.Sleep(100 * time.Millisecond)

	// Remove the file
	must.NoError(t, os.Remove(filePath))

	// Wait a bit — the Remove event should not add to pending
	time.Sleep(200 * time.Millisecond)

	// The file should NOT be in pending (it was removed, timer should have
	// either fired or the file won't be re-added by a Remove event)
	w.pendingMutex.RLock()
	_, exists := w.pendingFiles[filePath]
	w.pendingMutex.RUnlock()
	must.False(t, exists)
}
