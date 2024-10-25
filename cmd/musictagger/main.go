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
	replacements = flag.String("replacements", "replacements.json", "Path to the json file containing a map of replacements")
	musicLib     = flag.String("library", "", "Path to the music library")
	source       = flag.String("source", ".", "source directory, defaults to current dir")
	loglvl       = flag.String("log-level", "info", "The log level")
)

func main() {
	flag.Parse()

	if *musicLib == "" {
		log.Fatal("must provide an absolute path to the music library")
	}

	var replacementsMap map[string]string
	if *replacements != "" {
		replacementsFile, err := os.Open(*replacements)
		if err != nil {
			log.Fatalf("failed to read replacements file: %v", err)
		}
		defer replacementsFile.Close()

		b, _ := io.ReadAll(replacementsFile)
		if err = json.Unmarshal(b, &replacementsMap); err != nil {
			log.Fatal(err)
		}
	}

	// setup logging
	logLevel, err := log.ParseLevel(*loglvl)
	if err != nil {
		logLevel = log.InfoLevel
		log.Warnf("invalid log-level %s, set to %v", *loglvl, log.InfoLevel)
	}
	log.SetLevel(logLevel)

	musicLibrary, err := musictagger.GetAllTags(*source)
	if err != nil {
		log.Fatal(err)
	}

	for originalDir, music := range musicLibrary {
		var newDir string
		for _, m := range music {
			computedPath := internal.ComputeTargetPath(m.Metadata, m.Path, replacementsMap)
			newDir = filepath.Dir(filepath.Join(*musicLib, computedPath))
			newPath := filepath.Join(*musicLib, computedPath)

			if m.Path == newPath {
				continue
			}

			log.Infof("renaming %s to %s\n", m.Path, filepath.Join(*musicLib, computedPath))

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
