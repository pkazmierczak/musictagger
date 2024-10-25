package musictagger

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dhowden/tag"
)

type Music struct {
	Path     string
	Metadata tag.Metadata
}

// GetAllTags traverses a given directory recursively and extracts all tags it
// can find. It returns a map of album directory to music.
func GetAllTags(dir string) (map[string][]Music, error) {
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
