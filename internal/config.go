package internal

import (
	"encoding/json"
	"io"
	"os"
)

// Config holds all configuration for librato
type Config struct {
	// Global settings
	Library  string `json:"library"`   // Path to music library (required)
	LogLevel string `json:"log_level"` // Logging level (debug, info, warn, error)

	// Processing settings
	Replacements map[string]string `json:"replacements"`
	Pattern      Pattern           `json:"pattern"`

	// Daemon-specific settings
	Daemon *DaemonConfig `json:"daemon,omitempty"`
}

// DaemonConfig holds daemon-specific configuration
type DaemonConfig struct {
	WatchDir         string `json:"watch_dir"`
	QuarantineDir    string `json:"quarantine_dir"`
	DebounceTime     string `json:"debounce_time"`     // Duration string e.g. "2s"
	StateFile        string `json:"state_file"`
	PIDFile          string `json:"pid_file"`
	ScanOnStartup    bool   `json:"scan_on_startup"`
	CleanupEmptyDirs bool   `json:"cleanup_empty_dirs"`
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() Config {
	return Config{
		LogLevel:     "info",
		Replacements: make(map[string]string),
		Pattern:      DefaultPattern(),
		Daemon:       nil, // Daemon config is optional
	}
}

// DefaultDaemonConfig returns daemon config with sensible defaults
func DefaultDaemonConfig() *DaemonConfig {
	return &DaemonConfig{
		DebounceTime:     "2s",
		StateFile:        "/var/lib/librato/state.json",
		PIDFile:          "/var/run/librato.pid",
		ScanOnStartup:    true,
		CleanupEmptyDirs: true,
	}
}

// LoadConfig loads configuration from a JSON file
func LoadConfig(path string) (Config, error) {
	config := DefaultConfig()

	file, err := os.Open(path)
	if err != nil {
		return config, err
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return config, err
	}

	if err := json.Unmarshal(b, &config); err != nil {
		return config, err
	}

	// Merge daemon config with defaults if present
	if config.Daemon != nil {
		defaults := DefaultDaemonConfig()
		if config.Daemon.DebounceTime == "" {
			config.Daemon.DebounceTime = defaults.DebounceTime
		}
		if config.Daemon.StateFile == "" {
			config.Daemon.StateFile = defaults.StateFile
		}
		if config.Daemon.PIDFile == "" {
			config.Daemon.PIDFile = defaults.PIDFile
		}
	}

	return config, nil
}
