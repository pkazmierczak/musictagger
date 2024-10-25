package internal

import (
	"path/filepath"
	"testing"

	"github.com/dhowden/tag"
)

func TestComputeTargetPath(t *testing.T) {
	replacementsTable := map[string]string{
		"ą": "a",
		"æ": "ae",
		"å": "o",
		"ä": "ae",
		"ć": "c",
		"ę": "e",
		"ł": "l",
		"ń": "n",
		"ó": "o",
		"ø": "o",
		"ö": "oe",
		"ś": "s",
		"ß": "ss",
		"ü": "ue",
		"ź": "z",
		"ż": "z",
		" ": "_",
	}

	originalPath := "/home/user/track.flac"

	tests := []struct {
		name   string
		source tag.Metadata
		want   string
	}{
		{
			"missing meta",
			nil,
			"",
		},
		{
			"polish diacritics",
			mockTag{album: "zażółć", artist: "gęślą", track: 1, title: "jaźń"},
			filepath.Join("gesla-zazolc", "01-jazn.flac"),
		},
		{
			"forward slash and space",
			mockTag{album: "zażółć/gęślą", artist: "jaźń", track: 10, title: "już dziś"},
			filepath.Join("jazn-zazolc_gesla", "10-juz_dzis.flac"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ComputeTargetPath(tt.source, originalPath, replacementsTable); got != tt.want {
				t.Errorf("ComputeTargetPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
