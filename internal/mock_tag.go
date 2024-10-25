package internal

import (
	"github.com/dhowden/tag"
)

var _ tag.Metadata = (*mockTag)(nil)

type mockTag struct {
	album  string
	artist string
	track  int
	title  string
}

func (mockTag) Format() tag.Format            { return "" }
func (mockTag) FileType() tag.FileType        { return tag.FLAC }
func (m mockTag) Raw() map[string]interface{} { return nil }

func (m mockTag) Title() string         { return m.title }
func (m mockTag) Album() string         { return m.album }
func (m mockTag) Artist() string        { return m.artist }
func (m mockTag) Genre() string         { return "" }
func (m mockTag) Year() int             { return 2024 }
func (m mockTag) Track() (int, int)     { return m.track, 0 }
func (m mockTag) AlbumArtist() string   { return "" }
func (m mockTag) Composer() string      { return "" }
func (mockTag) Disc() (int, int)        { return 0, 0 }
func (m mockTag) Picture() *tag.Picture { return nil }
func (m mockTag) Lyrics() string        { return "" }
func (m mockTag) Comment() string       { return "" }
