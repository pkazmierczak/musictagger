package main

import (
	"encoding/json"
	"flag"
	"io"
	"os"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/pkazmierczak/librato/internal"
)

const defaultConfigPath = "/etc/librato/config.json"

var (
	// Primary config flag
	configFile = flag.String("config", defaultConfigPath, "Path to the configuration file")

	// Mode flags
	daemonMode = flag.Bool("daemon", false, "Run as background daemon")
	source     = flag.String("source", ".", "Source directory for one-shot mode (defaults to current dir)")
	dryRun     = flag.Bool("dry", false, "Dry run (no files moved)")

	// Legacy flag (deprecated)
	replacements = flag.String("replacements", "", "Path to the json file containing a map of replacements (deprecated, use config file)")
)

func main() {
	flag.Parse()

	// Load configuration from file
	config := loadConfiguration()

	// Setup logging based on config
	logLevel, err := log.ParseLevel(config.LogLevel)
	if err != nil {
		logLevel = log.InfoLevel
		log.Warnf("invalid log_level %q in config, defaulting to info", config.LogLevel)
	}
	log.SetLevel(logLevel)

	// Validate required config
	if config.Library == "" {
		log.Fatal("config: 'library' is required (path to the music library)")
	}

	log.Infof("using patterns: dir=%s, file=%s", config.Pattern.DirPattern, config.Pattern.FilePattern)

	// Mode selection
	if *daemonMode {
		runDaemon(config)
	} else {
		runOneShot(config)
	}
}

// loadConfiguration loads configuration from the config file
func loadConfiguration() internal.Config {
	config := internal.DefaultConfig()

	// Load config file
	if *configFile != "" {
		if _, err := os.Stat(*configFile); err == nil {
			loadedConfig, err := internal.LoadConfig(*configFile)
			if err != nil {
				log.Fatalf("failed to load config file %s: %v", *configFile, err)
			}
			config = loadedConfig
			log.Debugf("loaded config from %s", *configFile)
		} else if *configFile != defaultConfigPath {
			// Only fatal if a non-default config path was explicitly specified
			log.Fatalf("config file not found: %s", *configFile)
		} else {
			log.Debugf("default config file %s not found, using defaults", *configFile)
		}
	}

	// Legacy: support old replacements.json file (deprecated)
	if *replacements != "" {
		log.Warn("the -replacements flag is deprecated, use the config file instead")
		replacementsFile, err := os.Open(*replacements)
		if err != nil {
			log.Fatalf("failed to read replacements file: %v", err)
		}
		defer replacementsFile.Close()

		b, _ := io.ReadAll(replacementsFile)
		if err = json.Unmarshal(b, &config.Replacements); err != nil {
			log.Fatal(err)
		}
		log.Debugf("loaded replacements from %s", *replacements)
	}

	return config
}

// runOneShot runs the traditional one-shot processing mode
func runOneShot(config internal.Config) {
	processor := internal.NewProcessor(config, config.Library, *dryRun, log.StandardLogger())

	if err := processor.ProcessDirectory(*source); err != nil {
		log.Fatalf("failed to process directory: %v", err)
	}

	log.Info("processing complete")
}

// runDaemon runs the background daemon mode
func runDaemon(config internal.Config) {
	// Validate daemon config exists
	if config.Daemon == nil {
		log.Fatal("daemon mode requires 'daemon' section in config file")
	}

	dc := config.Daemon
	if dc.WatchDir == "" {
		log.Fatal("config: daemon.watch_dir is required")
	}
	if dc.QuarantineDir == "" {
		log.Fatal("config: daemon.quarantine_dir is required")
	}

	log.Infof("starting daemon mode: watching %s", dc.WatchDir)

	// Create processor
	processor := internal.NewProcessor(config, config.Library, *dryRun, log.StandardLogger())

	// Parse debounce time
	debounceTime := 2 * time.Second
	if dc.DebounceTime != "" {
		parsed, err := time.ParseDuration(dc.DebounceTime)
		if err != nil {
			log.Warnf("invalid debounce_time %q, using default 2s", dc.DebounceTime)
		} else {
			debounceTime = parsed
		}
	}

	// Create daemon options from config
	daemonOpts := DaemonOptions{
		WatchDir:      dc.WatchDir,
		QuarantineDir: dc.QuarantineDir,
		PIDFile:       dc.PIDFile,
		StateFile:     dc.StateFile,
		DebounceTime:  debounceTime,
		ScanOnStartup: dc.ScanOnStartup,
		CleanupEmpty:  dc.CleanupEmptyDirs,
	}

	daemon, err := NewDaemon(processor, daemonOpts)
	if err != nil {
		log.Fatalf("failed to create daemon: %v", err)
	}

	// Start daemon
	if err := daemon.Start(); err != nil {
		log.Fatalf("failed to start daemon: %v", err)
	}

	// Run daemon event loop
	if err := daemon.Run(); err != nil {
		log.Fatalf("daemon error: %v", err)
	}
}
