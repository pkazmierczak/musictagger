package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dhowden/tag"
	log "github.com/sirupsen/logrus"

	"github.com/pkazmierczak/musictagger"
)

func computeTargetPath(source tag.Metadata, originalPath string, replacementsTable map[string]string) string {
	var outputDir, outputFile string

	track, _ := source.Track()

	outputDir += fmt.Sprintf("%s-%s",
		source.Artist(),
		source.Album(),
	)

	outputFile += fmt.Sprintf("%s-%s%s",
		fmt.Sprintf("%02d", track),
		source.Title(),
		strings.ToLower(filepath.Ext(originalPath)),
	)

	// get rid of weird characters
	for k, v := range replacementsTable {
		outputDir = strings.ReplaceAll(outputDir, k, v)
		outputFile = strings.ReplaceAll(outputFile, k, v)
	}

	// in some cases a file name may contain a directory separator symbol.
	// Remove it.
	outputFile = strings.ReplaceAll(outputFile, "/", "_")
	outputDir = strings.ReplaceAll(outputDir, "/", "_")

	return filepath.Join(outputDir, outputFile)
}

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
			computedPath := computeTargetPath(m.Metadata, m.Path, replacementsMap)
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
