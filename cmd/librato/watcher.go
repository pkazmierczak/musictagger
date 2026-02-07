package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"

	"github.com/pkazmierczak/librato/internal"
)

// Watcher monitors a directory for new music files
type Watcher struct {
	watchDir      string
	quarantineDir string
	debounceTime  time.Duration
	cleanupEmpty  bool

	fsWatcher *fsnotify.Watcher
	processor *internal.Processor
	state     *internal.DaemonState

	pendingFiles map[string]*FileEvent
	pendingMutex sync.RWMutex

	stopCh chan struct{}
	doneCh chan struct{}
}

// FileEvent represents a pending file to process
type FileEvent struct {
	Path     string
	LastSeen time.Time
	Timer    *time.Timer
}

// WatcherOptions configures the Watcher
type WatcherOptions struct {
	WatchDir      string
	QuarantineDir string
	DebounceTime  time.Duration
	CleanupEmpty  bool
}

// NewWatcher creates a new file watcher
func NewWatcher(processor *internal.Processor, state *internal.DaemonState, opts WatcherOptions) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	if opts.DebounceTime == 0 {
		opts.DebounceTime = 2 * time.Second
	}

	w := &Watcher{
		fsWatcher:     fsWatcher,
		watchDir:      opts.WatchDir,
		quarantineDir: opts.QuarantineDir,
		processor:     processor,
		state:         state,
		debounceTime:  opts.DebounceTime,
		cleanupEmpty:  opts.CleanupEmpty,
		pendingFiles:  make(map[string]*FileEvent),
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}

	return w, nil
}

// Start begins watching the directory
func (w *Watcher) Start() error {
	// Verify watch directory exists
	if _, err := os.Stat(w.watchDir); err != nil {
		return fmt.Errorf("watch directory does not exist: %w", err)
	}

	// Recursively add watch directory and all subdirectories
	if err := filepath.WalkDir(w.watchDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if err := w.fsWatcher.Add(path); err != nil {
				return fmt.Errorf("failed to watch directory %s: %w", path, err)
			}
			log.Debugf("watching directory: %s", path)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to set up watches: %w", err)
	}

	log.Infof("watching directory: %s (recursive)", w.watchDir)

	// Start event loop in goroutine
	go w.eventLoop()

	return nil
}

// eventLoop processes file system events
func (w *Watcher) eventLoop() {
	defer close(w.doneCh)

	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			log.Errorf("watcher error: %v", err)

		case <-w.stopCh:
			log.Debug("watcher stopping")
			return
		}
	}
}

// handleEvent processes a single file system event
func (w *Watcher) handleEvent(event fsnotify.Event) {
	// Only care about Create and Write events
	if event.Op&fsnotify.Create != fsnotify.Create && event.Op&fsnotify.Write != fsnotify.Write {
		return
	}

	info, err := os.Stat(event.Name)
	if err != nil {
		log.Debugf("failed to stat %s: %v", event.Name, err)
		return
	}

	// New directory: add it to the watcher and scan its files
	if info.IsDir() {
		if err := w.fsWatcher.Add(event.Name); err != nil {
			log.Warnf("failed to watch new directory %s: %v", event.Name, err)
			return
		}
		log.Debugf("watching new directory: %s", event.Name)
		w.scanDir(event.Name)
		return
	}

	log.Debugf("file event: %s %s", event.Op, event.Name)

	// Apply debouncing
	w.debounceFile(event.Name)
}

// scanDir scans a directory for files and debounces each one for processing.
// Used when a new subdirectory appears in the watch tree.
func (w *Watcher) scanDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Warnf("failed to read new directory %s: %v", dir, err)
		return
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if e.IsDir() {
			if err := w.fsWatcher.Add(path); err != nil {
				log.Warnf("failed to watch new directory %s: %v", path, err)
				continue
			}
			log.Debugf("watching new directory: %s", path)
			w.scanDir(path)
		} else {
			w.debounceFile(path)
		}
	}
}

// debounceFile implements debouncing for file events
func (w *Watcher) debounceFile(filePath string) {
	w.pendingMutex.Lock()
	defer w.pendingMutex.Unlock()

	// Check if file is already pending
	if pending, exists := w.pendingFiles[filePath]; exists {
		// Cancel existing timer
		pending.Timer.Stop()
		pending.LastSeen = time.Now()

		// Create new timer
		pending.Timer = time.AfterFunc(w.debounceTime, func() {
			w.processFile(filePath)
		})
	} else {
		// Create new pending file
		timer := time.AfterFunc(w.debounceTime, func() {
			w.processFile(filePath)
		})

		w.pendingFiles[filePath] = &FileEvent{
			Path:     filePath,
			LastSeen: time.Now(),
			Timer:    timer,
		}
	}
}

// processFile processes a debounced file
func (w *Watcher) processFile(filePath string) {
	// Remove from pending
	w.pendingMutex.Lock()
	delete(w.pendingFiles, filePath)
	w.pendingMutex.Unlock()

	log.Infof("processing file: %s", filePath)

	// Verify file still exists and is readable
	if err := w.verifyFileReady(filePath); err != nil {
		log.Warnf("file not ready, skipping: %v", err)
		return
	}

	// Compute hash for duplicate detection
	hash, err := internal.ComputeFileHash(filePath)
	if err != nil {
		log.Errorf("failed to compute hash for %s: %v", filePath, err)
		return
	}

	// Check if already processed
	if w.state.IsProcessed(filePath, hash) {
		log.Infof("file %s already processed, skipping", filePath)
		return
	}

	// Process the file
	opts := internal.ProcessorOptions{
		QuarantineDir:    w.quarantineDir,
		CleanupEmptyDirs: w.cleanupEmpty,
		WatchDir:         w.watchDir,
	}

	targetPath := ""
	success := false
	err = w.processor.ProcessFile(filePath, opts)
	if err != nil {
		log.Errorf("failed to process %s: %v", filePath, err)
	} else {
		success = true
		log.Infof("successfully processed %s", filePath)
	}

	// Record in state
	w.state.MarkProcessed(internal.ProcessedFile{
		Path:        filePath,
		Hash:        hash,
		ProcessedAt: time.Now(),
		TargetPath:  targetPath,
		Success:     success,
	})

	// Save state
	if err := w.state.Save(); err != nil {
		log.Errorf("failed to save state: %v", err)
	}
}

// verifyFileReady checks if a file is ready to process
func (w *Watcher) verifyFileReady(filePath string) error {
	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("file does not exist: %w", err)
	}

	// Check if it's a regular file
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file")
	}

	// Try to open for reading
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	defer f.Close()

	return nil
}

// Stop stops the watcher gracefully
func (w *Watcher) Stop() error {
	log.Info("stopping watcher")

	// Cancel all pending timers
	w.pendingMutex.Lock()
	for _, pending := range w.pendingFiles {
		pending.Timer.Stop()
	}
	w.pendingFiles = make(map[string]*FileEvent)
	w.pendingMutex.Unlock()

	// Signal stop
	close(w.stopCh)

	// Close fsnotify watcher
	if err := w.fsWatcher.Close(); err != nil {
		return fmt.Errorf("failed to close watcher: %w", err)
	}

	// Wait for event loop to finish
	<-w.doneCh

	log.Info("watcher stopped")
	return nil
}

// ScanExisting scans the watch directory for existing files
func (w *Watcher) ScanExisting() error {
	log.Infof("scanning existing files in %s", w.watchDir)

	count := 0
	err := filepath.WalkDir(w.watchDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Process file directly (no debouncing for startup scan)
		w.processFile(path)
		count++

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to scan directory: %w", err)
	}

	log.Infof("scanned %d existing files", count)
	return nil
}
