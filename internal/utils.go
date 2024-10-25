package internal

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dhowden/tag"
)

func ComputeTargetPath(source tag.Metadata, originalPath string, replacementsTable map[string]string) string {
	var outputDir, outputFile string

	if source == nil {
		return ""
	}

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
