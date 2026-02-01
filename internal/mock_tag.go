package internal

import (
	"github.com/dhowden/tag"
)

var _ tag.Metadata = (*mockTag)(nil)

type mockTag struct {
	album       string
	artist      string
	albumArtist string
	track       int
	tracks      int
	title       string
	disc        int
	discs       int
	year        int
	genre       string
}

func (mockTag) Format() tag.Format            { return "" }
func (mockTag) FileType() tag.FileType        { return tag.FLAC }
func (m mockTag) Raw() map[string]interface{} { return nil }

func (m mockTag) Title() string         { return m.title }
func (m mockTag) Album() string         { return m.album }
func (m mockTag) Artist() string        { return m.artist }
func (m mockTag) Genre() string         { return m.genre }
func (m mockTag) Year() int             { return m.year }
func (m mockTag) Track() (int, int)     { return m.track, m.tracks }
func (m mockTag) AlbumArtist() string   { return m.albumArtist }
func (m mockTag) Composer() string      { return "" }
func (m mockTag) Disc() (int, int)      { return m.disc, m.discs }
func (m mockTag) Picture() *tag.Picture { return nil }
func (m mockTag) Lyrics() string        { return "" }
func (m mockTag) Comment() string       { return "" }
