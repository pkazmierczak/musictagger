package main

import (
	"encoding/json"
	"flag"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/pkazmierczak/musictagger"
	"github.com/pkazmierczak/musictagger/internal"
)

var (
	configFile   = flag.String("config", "config.json", "Path to the configuration file (optional)")
	replacements = flag.String("replacements", "", "Path to the json file containing a map of replacements (legacy, prefer config file)")
	dirPattern   = flag.String("dir-pattern", "", "Directory naming pattern (e.g., '{{artist}}-{{album}}')")
	filePattern  = flag.String("file-pattern", "", "File naming pattern (e.g., '{{disc_prefix}}{{track}}-{{title}}')")
	musicLib     = flag.String("library", "", "Path to the music library")
	source       = flag.String("source", ".", "source directory, defaults to current dir")
	dry          = flag.Bool("dry", false, "Dry run (no actual files moved)")
	loglvl       = flag.String("log-level", "info", "The log level")
)

func main() {
	flag.Parse()

	if *musicLib == "" {
		log.Fatal("must provide an absolute path to the music library")
	}

	// setup logging first
	logLevel, err := log.ParseLevel(*loglvl)
	if err != nil {
		logLevel = log.InfoLevel
		log.Warnf("invalid log-level %s, set to %v", *loglvl, log.InfoLevel)
	}
	log.SetLevel(logLevel)

	// Load configuration
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

	log.Infof("using patterns: dir=%s, file=%s", config.Pattern.DirPattern, config.Pattern.FilePattern)

	musicLibrary, err := musictagger.GetAllTags(*source)
	if err != nil {
		log.Fatal(err)
	}

	for originalDir, music := range musicLibrary {
		var newDir string
		for _, m := range music {
			computedPath := config.Pattern.FormatPath(m.Metadata, m.Path, config.Replacements)
			newDir = filepath.Dir(filepath.Join(*musicLib, computedPath))
			newPath := filepath.Join(*musicLib, computedPath)

			if m.Path == newPath {
				continue
			}

			log.Infof("renaming %s to %s\n", m.Path, filepath.Join(*musicLib, computedPath))

			if *dry {
				continue
			}

			if _, err := os.Stat(newDir); os.IsNotExist(err) {
				err := os.Mkdir(newDir, 0755)
				if err != nil {
					log.Fatal(err)
				}
			}

			if err := os.Rename(m.Path, newPath); err != nil {
				log.Warn(err)
			}
		}

		if originalDir == newDir {
			continue
		}

		// if there's any other files in the directory, copy them
		if err := filepath.WalkDir(originalDir, func(s string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				log.Infof("renaming %s to %s\n", filepath.Join(
					originalDir, d.Name()),
					filepath.Join(newDir, d.Name()),
				)
				if *dry {
					return nil
				}
				if err := os.Rename(
					filepath.Join(originalDir, d.Name()),
					filepath.Join(newDir, d.Name()),
				); err != nil {
					log.Warn(err)
				}
			}
			return nil
		}); err != nil {
			log.Warn(err)
		}
	}
}
