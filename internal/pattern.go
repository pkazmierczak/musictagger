package internal

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dhowden/tag"
)

// Pattern defines the directory and file naming patterns
type Pattern struct {
	DirPattern  string `json:"dir_pattern"`
	FilePattern string `json:"file_pattern"`
}

// DefaultPattern returns the default pattern (matches current behavior)
func DefaultPattern() Pattern {
	return Pattern{
		DirPattern:  "{{artist}}-{{album}}",
		FilePattern: "{{track}}-{{title}}",
	}
}

// FormatPath applies the pattern to the metadata and returns the complete path
func (p Pattern) FormatPath(metadata tag.Metadata, originalPath string, replacementsTable map[string]string) string {
	if metadata == nil {
		return ""
	}

	// Build context from metadata
	context := buildContext(metadata)

	// Format directory and file
	dir := formatTemplate(p.DirPattern, context)
	file := formatTemplate(p.FilePattern, context)

	// Add file extension
	file += filepath.Ext(originalPath)

	// Apply replacements
	dir = applyReplacements(dir, replacementsTable)
	file = applyReplacements(file, replacementsTable)

	// Remove directory separators from individual components
	dir = strings.ReplaceAll(dir, "/", "_")
	file = strings.ReplaceAll(file, "/", "_")

	// Convert to lowercase
	dir = strings.ToLower(dir)
	file = strings.ToLower(file)

	// Limit directory length
	if len(dir) > 40 {
		dir = dir[:40]
	}

	return filepath.Join(dir, file)
}

// buildContext creates a map of available template variables from metadata
func buildContext(metadata tag.Metadata) map[string]string {
	track, totalTracks := metadata.Track()
	disc, totalDiscs := metadata.Disc()

	artist := metadata.Artist()
	if metadata.AlbumArtist() != "" {
		artist = metadata.AlbumArtist()
	}

	context := map[string]string{
		"artist":       artist,
		"album":        metadata.Album(),
		"title":        metadata.Title(),
		"track":        fmt.Sprintf("%02d", track),
		"track_raw":    strconv.Itoa(track),
		"total_tracks": strconv.Itoa(totalTracks),
		"disc":         strconv.Itoa(disc),
		"discs":        strconv.Itoa(totalDiscs),
		"year":         strconv.Itoa(metadata.Year()),
		"genre":        metadata.Genre(),
	}

	// Add disc prefix for multi-disc albums
	if totalDiscs > 1 {
		context["disc_prefix"] = fmt.Sprintf("%d-", disc)
	} else {
		context["disc_prefix"] = ""
	}

	return context
}

// formatTemplate replaces {{key}} placeholders with values from context
func formatTemplate(template string, context map[string]string) string {
	result := template
	for key, value := range context {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// applyReplacements applies character replacements to a string
func applyReplacements(s string, replacementsTable map[string]string) string {
	result := s
	for k, v := range replacementsTable {
		result = strings.ReplaceAll(result, k, v)
	}
	return result
}
