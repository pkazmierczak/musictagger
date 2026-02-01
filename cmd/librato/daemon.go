package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/pkazmierczak/librato/internal"
)

// Daemon manages the background service lifecycle
type Daemon struct {
	processor  *internal.Processor
	watcher    *Watcher
	state      *internal.DaemonState
	pidFile    string
	shutdownCh chan os.Signal
}

// DaemonOptions configures the daemon
type DaemonOptions struct {
	WatchDir      string
	QuarantineDir string
	PIDFile       string
	StateFile     string
	DebounceTime  time.Duration
	ScanOnStartup bool
	CleanupEmpty  bool
}

// NewDaemon creates a new daemon instance
func NewDaemon(processor *internal.Processor, opts DaemonOptions) (*Daemon, error) {
	// Validate options
	if opts.WatchDir == "" {
		return nil, fmt.Errorf("watch directory is required")
	}
	if opts.QuarantineDir == "" {
		return nil, fmt.Errorf("quarantine directory is required")
	}
	if opts.StateFile == "" {
		opts.StateFile = "/var/lib/librato/state.json"
	}
	if opts.PIDFile == "" {
		opts.PIDFile = "/var/run/librato.pid"
	}
	if opts.DebounceTime == 0 {
		opts.DebounceTime = 2 * time.Second
	}

	// Load or create state
	state, err := internal.LoadState(opts.StateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	// Create watcher
	watcherOpts := WatcherOptions{
		WatchDir:      opts.WatchDir,
		QuarantineDir: opts.QuarantineDir,
		DebounceTime:  opts.DebounceTime,
		CleanupEmpty:  opts.CleanupEmpty,
	}
	watcher, err := NewWatcher(processor, state, watcherOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	d := &Daemon{
		processor:  processor,
		watcher:    watcher,
		state:      state,
		pidFile:    opts.PIDFile,
		shutdownCh: make(chan os.Signal, 1),
	}

	return d, nil
}

// Start initializes the daemon
func (d *Daemon) Start() error {
	log.Info("starting librato daemon")

	// Write PID file
	if err := d.writePIDFile(); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Setup signal handlers
	signal.Notify(d.shutdownCh, syscall.SIGTERM, syscall.SIGINT)

	// Start watcher
	if err := d.watcher.Start(); err != nil {
		return fmt.Errorf("failed to start watcher: %w", err)
	}

	// Scan existing files on startup
	if err := d.watcher.ScanExisting(); err != nil {
		log.Warnf("failed to scan existing files: %v", err)
	}

	log.Info("daemon started successfully")
	return nil
}

// Run executes the main daemon event loop
func (d *Daemon) Run() error {
	log.Info("daemon running, waiting for events")

	// Periodic state cleanup
	cleanupTicker := time.NewTicker(24 * time.Hour)
	defer cleanupTicker.Stop()

	// Periodic stats logging
	statsTicker := time.NewTicker(1 * time.Hour)
	defer statsTicker.Stop()

	for {
		select {
		case sig := <-d.shutdownCh:
			log.Infof("received signal %v, shutting down", sig)
			return d.Shutdown()

		case <-cleanupTicker.C:
			// Clean up old state entries (30 days)
			d.state.Cleanup(30 * 24 * time.Hour)
			if err := d.state.Save(); err != nil {
				log.Errorf("failed to save state after cleanup: %v", err)
			}

		case <-statsTicker.C:
			// Log statistics
			stats := d.state.GetStats()
			log.Infof("daemon stats: processed=%d success=%d failed=%d uptime=%v",
				stats.TotalProcessed, stats.TotalSuccess, stats.TotalFailed,
				time.Since(stats.StartTime))
		}
	}
}

// Shutdown performs graceful shutdown
func (d *Daemon) Shutdown() error {
	log.Info("shutting down daemon")

	// Stop watcher (waits for in-flight operations)
	if err := d.watcher.Stop(); err != nil {
		log.Errorf("error stopping watcher: %v", err)
	}

	// Save final state
	if err := d.state.Save(); err != nil {
		log.Errorf("failed to save final state: %v", err)
	}

	// Remove PID file
	if err := d.removePIDFile(); err != nil {
		log.Errorf("failed to remove PID file: %v", err)
	}

	log.Info("daemon shutdown complete")
	return nil
}

// writePIDFile writes the process ID to a file
func (d *Daemon) writePIDFile() error {
	// Check if PID file already exists
	if data, err := os.ReadFile(d.pidFile); err == nil {
		// Parse existing PID
		existingPID, err := strconv.Atoi(string(data))
		if err == nil {
			// Check if process is still running
			if processExists(existingPID) {
				return fmt.Errorf("daemon already running with PID %d", existingPID)
			}
			log.Warnf("stale PID file found (PID %d not running), overwriting", existingPID)
		}
	}

	// Write current PID
	pid := os.Getpid()
	pidStr := strconv.Itoa(pid)

	if err := os.WriteFile(d.pidFile, []byte(pidStr), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	log.Infof("wrote PID %d to %s", pid, d.pidFile)
	return nil
}

// removePIDFile removes the PID file
func (d *Daemon) removePIDFile() error {
	if err := os.Remove(d.pidFile); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// processExists checks if a process with given PID exists
func processExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
