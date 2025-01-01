package internal

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dhowden/tag"
)

func ComputeTargetPath(source tag.Metadata, originalPath string, replacementsTable map[string]string) string {
	var outputDir, outputFile string

	if source == nil {
		return ""
	}

	track, _ := source.Track()

	artist := source.Artist()
	if source.AlbumArtist() != "" {
		artist = source.AlbumArtist()
	}

	outputDir += strings.ToLower(fmt.Sprintf("%s-%s", artist, source.Album()))

	outputFile += strings.ToLower(fmt.Sprintf("%s-%s%s",
		fmt.Sprintf("%02d", track),
		source.Title(),
		filepath.Ext(originalPath),
	))

	// is this a multi-album? prepend the file with album number
	// if source.
	disc, discs := source.Disc()
	if discs > 1 {
		d := strconv.Itoa(disc)
		outputFile = fmt.Sprintf("%s-%s", d, outputFile)
	}

	// get rid of weird characters
	for k, v := range replacementsTable {
		outputDir = strings.ReplaceAll(outputDir, k, v)
		outputFile = strings.ReplaceAll(outputFile, k, v)
	}

	// in some cases a file name may contain a directory separator symbol.
	// Remove it.
	outputFile = strings.ReplaceAll(outputFile, "/", "_")
	outputDir = strings.ReplaceAll(outputDir, "/", "_")

	// outputDir should not be too long, otherwise it becomes annoying.
	if len(outputDir) > 40 {
		outputDir = outputDir[:40]
	}

	return filepath.Join(outputDir, outputFile)
}
