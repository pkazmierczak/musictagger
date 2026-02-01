package main

import (
	"encoding/json"
	"flag"
	"io"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/pkazmierczak/librato/internal"
)

var (
	// Existing flags
	configFile   = flag.String("config", "config.json", "Path to the configuration file (optional)")
	replacements = flag.String("replacements", "", "Path to the json file containing a map of replacements (legacy, prefer config file)")
	dirPattern   = flag.String("dir-pattern", "", "Directory naming pattern (e.g., '{{artist}}-{{album}}')")
	filePattern  = flag.String("file-pattern", "", "File naming pattern (e.g., '{{disc_prefix}}{{track}}-{{title}}')")
	musicLib     = flag.String("library", "", "Path to the music library")
	source       = flag.String("source", ".", "source directory, defaults to current dir")
	dry          = flag.Bool("dry", false, "Dry run (no actual files moved)")
	loglvl       = flag.String("log-level", "info", "The log level")

	// New daemon-specific flags
	daemon        = flag.Bool("daemon", false, "Run as background daemon")
	watchDir      = flag.String("watch-dir", "", "Directory to watch (daemon mode)")
	quarantineDir = flag.String("quarantine-dir", "", "Directory for untagged files (daemon mode)")
	pidFile       = flag.String("pid-file", "/var/run/librato.pid", "PID file path (daemon mode)")
	stateFile     = flag.String("state-file", "/var/lib/librato/state.json", "State file path (daemon mode)")
)

func main() {
	flag.Parse()

	if *musicLib == "" {
		log.Fatal("must provide an absolute path to the music library")
	}

	// Setup logging
	logLevel, err := log.ParseLevel(*loglvl)
	if err != nil {
		logLevel = log.InfoLevel
		log.Warnf("invalid log-level %s, set to %v", *loglvl, log.InfoLevel)
	}
	log.SetLevel(logLevel)

	// Load configuration
	config := loadConfiguration()

	log.Infof("using patterns: dir=%s, file=%s", config.Pattern.DirPattern, config.Pattern.FilePattern)

	// Mode selection
	if *daemon {
		runDaemon(config)
	} else {
		runOneShot(config)
	}
}

// loadConfiguration loads and merges configuration from multiple sources
func loadConfiguration() internal.Config {
	config := internal.DefaultConfig()

	// Try to load config file if it exists
	if *configFile != "" {
		if _, err := os.Stat(*configFile); err == nil {
			loadedConfig, err := internal.LoadConfig(*configFile)
			if err != nil {
				log.Warnf("failed to load config file %s: %v, using defaults", *configFile, err)
			} else {
				config = loadedConfig
				log.Debugf("loaded config from %s", *configFile)
			}
		} else {
			log.Debugf("config file %s not found, using defaults", *configFile)
		}
	}

	// Legacy: support old replacements.json file
	if *replacements != "" {
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

	// CLI flags override config file
	if *dirPattern != "" {
		config.Pattern.DirPattern = *dirPattern
		log.Debugf("using dir pattern from CLI: %s", *dirPattern)
	}
	if *filePattern != "" {
		config.Pattern.FilePattern = *filePattern
		log.Debugf("using file pattern from CLI: %s", *filePattern)
	}

	return config
}

// runOneShot runs the traditional one-shot processing mode
func runOneShot(config internal.Config) {
	processor := internal.NewProcessor(config, *musicLib, *dry, log.StandardLogger())

	if err := processor.ProcessDirectory(*source); err != nil {
		log.Fatalf("failed to process directory: %v", err)
	}

	log.Info("processing complete")
}

// runDaemon runs the background daemon mode
func runDaemon(config internal.Config) {
	// Validate daemon-specific requirements
	if *watchDir == "" {
		log.Fatal("daemon mode requires -watch-dir flag")
	}
	if *quarantineDir == "" {
		log.Fatal("daemon mode requires -quarantine-dir flag")
	}

	log.Infof("starting daemon mode: watching %s", *watchDir)

	// Create processor
	processor := internal.NewProcessor(config, *musicLib, *dry, log.StandardLogger())

	// Create daemon
	daemonOpts := DaemonOptions{
		WatchDir:      *watchDir,
		QuarantineDir: *quarantineDir,
		PIDFile:       *pidFile,
		StateFile:     *stateFile,
		ScanOnStartup: true,
		CleanupEmpty:  true,
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
