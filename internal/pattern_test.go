package internal

import (
	"path/filepath"
	"testing"

	"github.com/shoenig/test/must"
)

func TestPattern_FormatPath(t *testing.T) {
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
		name    string
		pattern Pattern
		source  mockTag
		want    string
	}{
		{
			"default pattern - basic",
			DefaultPattern(),
			mockTag{album: "Test Album", artist: "Test Artist", track: 1, title: "Test Song"},
			filepath.Join("test_artist-test_album", "01-test_song.flac"),
		},
		{
			"default pattern - polish diacritics",
			DefaultPattern(),
			mockTag{album: "zażółć", artist: "gęślą", track: 1, title: "jaźń"},
			filepath.Join("gesla-zazolc", "01-jazn.flac"),
		},
		{
			"default pattern - multi-disc",
			Pattern{
				DirPattern:  "{{artist}}-{{album}}",
				FilePattern: "{{disc_prefix}}{{track}}-{{title}}",
			},
			mockTag{album: "Test Album", artist: "Test Artist", track: 10, disc: 2, discs: 3, title: "Test Song"},
			filepath.Join("test_artist-test_album", "2-10-test_song.flac"),
		},
		{
			"custom pattern - year in directory",
			Pattern{
				DirPattern:  "{{year}}/{{artist}}-{{album}}",
				FilePattern: "{{track}}-{{title}}",
			},
			mockTag{album: "Test Album", artist: "Test Artist", track: 5, title: "Test Song", year: 2023},
			filepath.Join("2023_test_artist-test_album", "05-test_song.flac"),
		},
		{
			"custom pattern - genre based organization",
			Pattern{
				DirPattern:  "{{genre}}/{{artist}}",
				FilePattern: "{{album}}-{{track}}-{{title}}",
			},
			mockTag{album: "Test Album", artist: "Test Artist", track: 3, title: "Test Song", genre: "Rock"},
			filepath.Join("rock_test_artist", "test_album-03-test_song.flac"),
		},
		{
			"custom pattern - flat structure",
			Pattern{
				DirPattern:  "{{artist}}",
				FilePattern: "{{album}}-{{track}}-{{title}}",
			},
			mockTag{album: "Test Album", artist: "Test Artist", track: 7, title: "Test Song"},
			filepath.Join("test_artist", "test_album-07-test_song.flac"),
		},
		{
			"forward slash replacement",
			DefaultPattern(),
			mockTag{album: "Test/Album", artist: "Test/Artist", track: 1, title: "Test/Song"},
			filepath.Join("test_artist-test_album", "01-test_song.flac"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pattern.FormatPath(tt.source, originalPath, replacementsTable)
			must.Eq(t, tt.want, got)
		})
	}
}

func TestPattern_FormatPath_NilMetadata(t *testing.T) {
	pattern := DefaultPattern()
	got := pattern.FormatPath(nil, "/test/path.flac", map[string]string{})
	must.Eq(t, "", got)
}

func TestDefaultPattern(t *testing.T) {
	pattern := DefaultPattern()
	must.Eq(t, "{{artist}}-{{album}}", pattern.DirPattern)
	must.Eq(t, "{{track}}-{{title}}", pattern.FilePattern)
}

func TestBuildContext(t *testing.T) {
	metadata := mockTag{
		album:       "Test Album",
		artist:      "Test Artist",
		albumArtist: "Album Artist",
		track:       5,
		tracks:      12,
		disc:        2,
		discs:       3,
		title:       "Test Title",
		year:        2023,
		genre:       "Rock",
	}

	context := buildContext(metadata)

	tests := []struct {
		key   string
		want  string
	}{
		{"artist", "Album Artist"}, // Should prefer album artist
		{"album", "Test Album"},
		{"title", "Test Title"},
		{"track", "05"},
		{"track_raw", "5"},
		{"total_tracks", "12"},
		{"disc", "2"},
		{"discs", "3"},
		{"disc_prefix", "2-"},
		{"year", "2023"},
		{"genre", "Rock"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got, exists := context[tt.key]
			must.True(t, exists)
			must.Eq(t, tt.want, got)
		})
	}
}

func TestBuildContext_SingleDisc(t *testing.T) {
	metadata := mockTag{
		album:  "Test Album",
		artist: "Test Artist",
		track:  1,
		disc:   1,
		discs:  1,
		title:  "Test Title",
	}

	context := buildContext(metadata)

	must.Eq(t, "", context["disc_prefix"])
}
