package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	musicBrainzBaseURL    = "https://musicbrainz.org/ws/2"
	coverArtArchiveURL    = "https://coverartarchive.org"
	userAgent             = "librato/1.0 (https://github.com/pkazmierczak/librato)"
	defaultRequestTimeout = 30 * time.Second
)

// CoverFetcher handles downloading album artwork from MusicBrainz/Cover Art Archive
type CoverFetcher struct {
	client *http.Client
	logger *log.Logger
}

// NewCoverFetcher creates a new CoverFetcher instance
func NewCoverFetcher(logger *log.Logger) *CoverFetcher {
	return &CoverFetcher{
		client: &http.Client{
			Timeout: defaultRequestTimeout,
		},
		logger: logger,
	}
}

// FetchCover downloads cover art for an album if missing.
// It checks for existing cover.jpg or cover.png files first.
// Returns nil if cover already exists, was successfully fetched, or couldn't be found.
func (c *CoverFetcher) FetchCover(targetDir, artist, album string) error {
	if c.coverExists(targetDir) {
		c.logger.Debugf("cover already exists in %s, skipping", targetDir)
		return nil
	}

	if artist == "" || album == "" {
		c.logger.Debugf("missing artist or album metadata, skipping cover fetch")
		return nil
	}

	c.logger.Infof("searching for cover art: %s - %s", artist, album)

	mbid, err := c.searchMusicBrainz(artist, album)
	if err != nil {
		return fmt.Errorf("MusicBrainz search failed: %w", err)
	}
	if mbid == "" {
		c.logger.Debugf("no MusicBrainz release found for %s - %s", artist, album)
		return nil
	}

	c.logger.Debugf("found MusicBrainz release: %s", mbid)

	coverData, contentType, err := c.fetchCoverArt(mbid)
	if err != nil {
		return fmt.Errorf("failed to fetch cover art: %w", err)
	}
	if coverData == nil {
		c.logger.Debugf("no cover art available for release %s", mbid)
		return nil
	}

	filename := c.determineFilename(contentType)
	coverPath := filepath.Join(targetDir, filename)

	if err := os.WriteFile(coverPath, coverData, 0644); err != nil {
		return fmt.Errorf("failed to write cover file: %w", err)
	}

	c.logger.Infof("saved cover art to %s", coverPath)
	return nil
}

// coverExists checks if a cover image already exists in the directory
func (c *CoverFetcher) coverExists(dir string) bool {
	for _, name := range []string{"cover.jpg", "cover.jpeg", "cover.png"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

// musicBrainzSearchResponse represents the MusicBrainz release search response
type musicBrainzSearchResponse struct {
	Releases []struct {
		ID    string `json:"id"`
		Score int    `json:"score"`
	} `json:"releases"`
}

// searchMusicBrainz searches for a release and returns its MBID
func (c *CoverFetcher) searchMusicBrainz(artist, album string) (string, error) {
	query := fmt.Sprintf(`artist:"%s" AND release:"%s"`, artist, album)
	searchURL := fmt.Sprintf("%s/release?query=%s&fmt=json&limit=1",
		musicBrainzBaseURL, url.QueryEscape(query))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("MusicBrainz returned status %d", resp.StatusCode)
	}

	var result musicBrainzSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse MusicBrainz response: %w", err)
	}

	if len(result.Releases) == 0 {
		return "", nil
	}

	return result.Releases[0].ID, nil
}

// fetchCoverArt fetches the front cover from Cover Art Archive
// Returns the image data, content type, and any error
func (c *CoverFetcher) fetchCoverArt(mbid string) ([]byte, string, error) {
	coverURL := fmt.Sprintf("%s/release/%s/front", coverArtArchiveURL, mbid)

	req, err := http.NewRequest("GET", coverURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, "", nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("Cover Art Archive returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read cover data: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	return data, contentType, nil
}

// determineFilename returns the appropriate filename based on content type
func (c *CoverFetcher) determineFilename(contentType string) string {
	switch contentType {
	case "image/png":
		return "cover.png"
	case "image/jpeg", "image/jpg":
		return "cover.jpg"
	default:
		return "cover.jpg"
	}
}
