package internal

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/shoenig/test/must"
)

// newTestCoverFetcher creates a CoverFetcher wired to mock MusicBrainz and
// Cover Art Archive servers. The caller is responsible for closing the servers.
func newTestCoverFetcher(t *testing.T, mbHandler, caaHandler http.Handler) *CoverFetcher {
	t.Helper()
	logger := log.New()
	logger.SetOutput(os.Stderr)
	logger.SetLevel(log.DebugLevel)

	mbServer := httptest.NewServer(mbHandler)
	t.Cleanup(mbServer.Close)

	caaServer := httptest.NewServer(caaHandler)
	t.Cleanup(caaServer.Close)

	return &CoverFetcher{
		client:             &http.Client{},
		logger:             logger,
		musicBrainzBaseURL: mbServer.URL + "/ws/2",
		coverArtArchiveURL: caaServer.URL,
	}
}

func TestCoverFetcher_coverExists(t *testing.T) {
	logger := log.New()
	logger.SetOutput(os.Stderr)
	fetcher := NewCoverFetcher(logger)

	t.Run("no cover exists", func(t *testing.T) {
		dir := t.TempDir()
		must.False(t, fetcher.coverExists(dir))
	})

	t.Run("cover.jpg exists", func(t *testing.T) {
		dir := t.TempDir()
		must.NoError(t, os.WriteFile(filepath.Join(dir, "cover.jpg"), []byte("fake"), 0644))
		must.True(t, fetcher.coverExists(dir))
	})

	t.Run("cover.png exists", func(t *testing.T) {
		dir := t.TempDir()
		must.NoError(t, os.WriteFile(filepath.Join(dir, "cover.png"), []byte("fake"), 0644))
		must.True(t, fetcher.coverExists(dir))
	})

	t.Run("cover.jpeg exists", func(t *testing.T) {
		dir := t.TempDir()
		must.NoError(t, os.WriteFile(filepath.Join(dir, "cover.jpeg"), []byte("fake"), 0644))
		must.True(t, fetcher.coverExists(dir))
	})
}

func TestCoverFetcher_determineFilename(t *testing.T) {
	logger := log.New()
	fetcher := NewCoverFetcher(logger)

	tests := []struct {
		contentType string
		want        string
	}{
		{"image/jpeg", "cover.jpg"},
		{"image/jpg", "cover.jpg"},
		{"image/png", "cover.png"},
		{"application/octet-stream", "cover.jpg"},
		{"", "cover.jpg"},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			got := fetcher.determineFilename(tt.contentType)
			must.Eq(t, tt.want, got)
		})
	}
}

func TestCoverFetcher_searchMusicBrainz(t *testing.T) {
	t.Run("successful search", func(t *testing.T) {
		fetcher := newTestCoverFetcher(t,
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				must.Eq(t, "/ws/2/release", r.URL.Path)
				must.StrContains(t, r.URL.RawQuery, "query=")
				must.StrContains(t, r.URL.RawQuery, "fmt=json")
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"releases":[{"id":"12345678-1234-1234-1234-123456789012","score":100}]}`))
			}),
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		)

		mbid, err := fetcher.searchMusicBrainz("Pink Floyd", "The Dark Side of the Moon")
		must.NoError(t, err)
		must.Eq(t, "12345678-1234-1234-1234-123456789012", mbid)
	})

	t.Run("no results", func(t *testing.T) {
		fetcher := newTestCoverFetcher(t,
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"releases":[]}`))
			}),
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		)

		mbid, err := fetcher.searchMusicBrainz("Unknown", "Nonexistent")
		must.NoError(t, err)
		must.Eq(t, "", mbid)
	})
}

func TestCoverFetcher_FetchCover_SkipsExisting(t *testing.T) {
	logger := log.New()
	logger.SetOutput(os.Stderr)
	fetcher := NewCoverFetcher(logger)

	dir := t.TempDir()
	must.NoError(t, os.WriteFile(filepath.Join(dir, "cover.jpg"), []byte("existing"), 0644))

	// Should return nil without making any HTTP requests
	err := fetcher.FetchCover(dir, "Artist", "Album")
	must.NoError(t, err)

	// Verify the existing cover wasn't modified
	content, err := os.ReadFile(filepath.Join(dir, "cover.jpg"))
	must.NoError(t, err)
	must.Eq(t, "existing", string(content))
}

func TestCoverFetcher_FetchCover_EmptyMetadata(t *testing.T) {
	logger := log.New()
	logger.SetOutput(os.Stderr)
	fetcher := NewCoverFetcher(logger)

	dir := t.TempDir()

	// Should return nil without errors when artist/album is empty
	must.NoError(t, fetcher.FetchCover(dir, "", "Album"))
	must.NoError(t, fetcher.FetchCover(dir, "Artist", ""))
	must.NoError(t, fetcher.FetchCover(dir, "", ""))

	// Verify no cover was created
	must.False(t, fetcher.coverExists(dir))
}

func TestCoverFetcher_fetchCoverArt_NotFound(t *testing.T) {
	fetcher := newTestCoverFetcher(t,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}),
	)

	data, contentType, err := fetcher.fetchCoverArt("some-mbid")
	must.NoError(t, err)
	must.Nil(t, data)
	must.Eq(t, "", contentType)
}

func TestCoverFetcher_FetchCover_FullFlow(t *testing.T) {
	fakeCover := []byte("fake-jpeg-data")

	fetcher := newTestCoverFetcher(t,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"releases":[{"id":"test-mbid","score":100}]}`))
		}),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(fakeCover)
		}),
	)

	dir := t.TempDir()
	must.NoError(t, fetcher.FetchCover(dir, "Artist", "Album"))
	must.True(t, fetcher.coverExists(dir))

	content, err := os.ReadFile(filepath.Join(dir, "cover.jpg"))
	must.NoError(t, err)
	must.Eq(t, fakeCover, content)
}

func TestCoverFetcher_FetchCover_DeduplicatesConcurrentRequests(t *testing.T) {
	var mbRequests atomic.Int32
	fakeCover := []byte("fake-jpeg-data")

	fetcher := newTestCoverFetcher(t,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mbRequests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"releases":[{"id":"test-mbid","score":100}]}`))
		}),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(fakeCover)
		}),
	)

	dir := t.TempDir()

	// Simulate 10 concurrent FetchCover calls for the same directory,
	// as would happen in daemon mode when an album's files arrive together
	done := make(chan error, 10)
	for range 10 {
		go func() {
			done <- fetcher.FetchCover(dir, "Artist", "Album")
		}()
	}
	for range 10 {
		must.NoError(t, <-done)
	}

	// Only one request should have hit MusicBrainz
	must.Eq(t, int32(1), mbRequests.Load())
	must.True(t, fetcher.coverExists(dir))
}

func TestCoverFetcher_FetchCover_DifferentDirsNotDeduplicated(t *testing.T) {
	var mbRequests atomic.Int32
	fakeCover := []byte("fake-jpeg-data")

	fetcher := newTestCoverFetcher(t,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mbRequests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"releases":[{"id":"test-mbid","score":100}]}`))
		}),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(fakeCover)
		}),
	)

	dir1 := t.TempDir()
	dir2 := t.TempDir()

	must.NoError(t, fetcher.FetchCover(dir1, "Artist", "Album 1"))
	must.NoError(t, fetcher.FetchCover(dir2, "Artist", "Album 2"))

	// Each directory should trigger its own fetch
	must.Eq(t, int32(2), mbRequests.Load())
	must.True(t, fetcher.coverExists(dir1))
	must.True(t, fetcher.coverExists(dir2))
}

func TestCoverFetcher_Integration(t *testing.T) {
	// Skip integration tests unless explicitly enabled
	if os.Getenv("LIBRATO_INTEGRATION_TESTS") == "" {
		t.Skip("skipping integration test; set LIBRATO_INTEGRATION_TESTS=1 to run")
	}

	logger := log.New()
	logger.SetLevel(log.DebugLevel)
	fetcher := NewCoverFetcher(logger)

	dir := t.TempDir()

	// Test with a well-known album that should have cover art
	err := fetcher.FetchCover(dir, "Pink Floyd", "The Dark Side of the Moon")
	must.NoError(t, err)

	// Check if cover was downloaded
	if fetcher.coverExists(dir) {
		t.Log("Successfully downloaded cover art")
	} else {
		t.Log("No cover art found (this may be expected for rate-limited requests)")
	}
}
