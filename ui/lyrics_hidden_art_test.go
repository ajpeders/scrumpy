package ui

import (
	"strings"
	"testing"
)

func TestYHidesLyricsBoxBesideArtwork(t *testing.T) {
	old := artworkMode
	artworkMode = artBlock
	t.Cleanup(func() { artworkMode = old })
	m := New(&fakeClient{})
	m.width, m.height, m.artwork = 120, 40, true
	m.now = NowPlaying{Title: "Song", ArtistName: "Artist", Duration: 200}
	m.coverLines = make([]string, artworkRows)
	if !m.artworkShown() || !strings.Contains(strings.Join(m.artBand([]string{"Song"}), "\n"), "Lyrics") {
		t.Fatal("precondition: artwork band shows a Lyrics box")
	}
	m.lyricsHidden = true
	if strings.Contains(strings.Join(m.artBand([]string{"Song"}), "\n"), "Lyrics") {
		t.Fatal("y should hide the Lyrics box beside the artwork")
	}
}
