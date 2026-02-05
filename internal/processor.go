package internal

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/dhowden/tag"
	log "github.com/sirupsen/logrus"
)

// Music represents a music file with metadata
type Music struct {
	Path     string
	Metadata tag.Metadata
}

// Processor handles the core logic of organizing music files
type Processor struct {
	config       Config
	musicLibrary string
	dryRun       bool
	logger       *log.Logger
	coverFetcher *CoverFetcher
}

// ProcessorOptions configures the Processor
type ProcessorOptions struct {
	QuarantineDir    string
	CleanupEmptyDirs bool
}

// NewProcessor creates a new Processor instance
func NewProcessor(config Config, musicLibrary string, dryRun bool, logger *log.Logger, coverFetcher *CoverFetcher) *Processor {
	return &Processor{
		config:       config,
		musicLibrary: musicLibrary,
		dryRun:       dryRun,
		logger:       logger,
		coverFetcher: coverFetcher,
	}
}

// ProcessDirectory processes all files in a directory (one-shot mode)
// This preserves the original behavior from main.go
func (p *Processor) ProcessDirectory(sourceDir string) error {
	musicLibrary, err := getAllTags(sourceDir)
	if err != nil {
		return fmt.Errorf("failed to scan directory: %w", err)
	}

	for originalDir, music := range musicLibrary {
		var newDir string
		for _, m := range music {
			computedPath := p.config.Pattern.FormatPath(m.Metadata, m.Path, p.config.Replacements)
			newDir = filepath.Dir(filepath.Join(p.musicLibrary, computedPath))
			newPath := filepath.Join(p.musicLibrary, computedPath)

			if m.Path == newPath {
				continue
			}

			p.logger.Infof("renaming %s to %s\n", m.Path, filepath.Join(p.musicLibrary, computedPath))

			if p.dryRun {
				continue
			}

			if _, err := os.Stat(newDir); os.IsNotExist(err) {
				err := os.MkdirAll(newDir, 0755)
				if err != nil {
					return fmt.Errorf("failed to create directory %s: %w", newDir, err)
				}
			}

			if err := os.Rename(m.Path, newPath); err != nil {
				p.logger.Warn(err)
			}
		}

		// Copy any other files in the directory (only if moving to a new location)
		if originalDir != newDir {
			if err := filepath.WalkDir(originalDir, func(s string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() {
					p.logger.Infof("renaming %s to %s\n", filepath.Join(
						originalDir, d.Name()),
						filepath.Join(newDir, d.Name()),
					)
					if p.dryRun {
						return nil
					}
					if err := os.Rename(
						filepath.Join(originalDir, d.Name()),
						filepath.Join(newDir, d.Name()),
					); err != nil {
						p.logger.Warn(err)
					}
				}
				return nil
			}); err != nil {
				p.logger.Warn(err)
			}
		}

		// Fetch cover art if enabled and not in dry-run mode
		// This runs for all album directories, regardless of whether files were moved
		targetDir := newDir
		if targetDir == "" {
			targetDir = originalDir
		}
		if p.coverFetcher != nil && targetDir != "" && !p.dryRun {
			if len(music) > 0 {
				artist := music[0].Metadata.AlbumArtist()
				if artist == "" {
					artist = music[0].Metadata.Artist()
				}
				album := music[0].Metadata.Album()
				if err := p.coverFetcher.FetchCover(targetDir, artist, album); err != nil {
					p.logger.Warnf("failed to fetch cover for %s: %v", targetDir, err)
				}
			}
		}
	}

	return nil
}

// ProcessFile processes a single music file for daemon mode
func (p *Processor) ProcessFile(filePath string, opts ProcessorOptions) error {
	// Open and read metadata
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	metadata, err := tag.ReadFrom(f)
	if err != nil || metadata == nil {
		// No metadata - handle quarantine if enabled
		if opts.QuarantineDir != "" {
			return p.quarantineFile(filePath, opts.QuarantineDir)
		}
		return fmt.Errorf("no metadata found in file %s", filePath)
	}

	// Compute target path
	computedPath := p.config.Pattern.FormatPath(metadata, filePath, p.config.Replacements)
	targetPath := filepath.Join(p.musicLibrary, computedPath)
	targetDir := filepath.Dir(targetPath)

	// Skip if already in correct location
	if filePath == targetPath {
		p.logger.Debugf("file %s already in correct location", filePath)
		return nil
	}

	p.logger.Infof("processing %s -> %s", filePath, targetPath)

	if p.dryRun {
		return nil
	}

	// Create target directory if needed
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
	}

	// Move file
	if err := os.Rename(filePath, targetPath); err != nil {
		return fmt.Errorf("failed to move file: %w", err)
	}

	// Cleanup empty source directory if enabled
	if opts.CleanupEmptyDirs {
		sourceDir := filepath.Dir(filePath)
		if err := p.cleanupEmptyDir(sourceDir); err != nil {
			p.logger.Warnf("failed to cleanup directory %s: %v", sourceDir, err)
		}
	}

	return nil
}

// quarantineFile moves a file without metadata to the quarantine directory
func (p *Processor) quarantineFile(filePath, quarantineDir string) error {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	originalName := filepath.Base(filePath)
	quarantinePath := filepath.Join(quarantineDir, fmt.Sprintf("%s_%s", timestamp, originalName))

	p.logger.Warnf("no metadata found, quarantining %s -> %s", filePath, quarantinePath)

	if p.dryRun {
		return nil
	}

	// Ensure quarantine directory exists
	if err := os.MkdirAll(quarantineDir, 0755); err != nil {
		return fmt.Errorf("failed to create quarantine directory: %w", err)
	}

	// Move to quarantine
	if err := os.Rename(filePath, quarantinePath); err != nil {
		return fmt.Errorf("failed to quarantine file: %w", err)
	}

	return nil
}

// cleanupEmptyDir removes a directory if it's empty
func (p *Processor) cleanupEmptyDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	// Only remove if empty
	if len(entries) == 0 {
		p.logger.Debugf("removing empty directory %s", dir)
		return os.Remove(dir)
	}

	return nil
}

// ComputeFileHash computes SHA256 hash of a file for duplicate detection
func ComputeFileHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// getAllTags traverses a given directory recursively and extracts all tags it
// can find. It returns a map of album directory to music files.
func getAllTags(dir string) (map[string][]Music, error) {
	tags := map[string][]Music{}
	if err := filepath.WalkDir(dir, func(s string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			f, err := os.Open(s)
			if err != nil {
				return err
			}
			defer f.Close()

			m, _ := tag.ReadFrom(f)
			if m != nil {
				tags[filepath.Dir(s)] = append(tags[filepath.Dir(s)], Music{s, m})
			}
		}
		return nil
	}); err != nil {
		return tags, err
	}

	return tags, nil
}
