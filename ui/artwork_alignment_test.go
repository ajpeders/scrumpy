package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestArtworkCellSizesKeepCoverSquare(t *testing.T) {
	for _, cell := range [][2]int{{9, 19}, {10, 20}, {10, 21}, {8, 17}, {11, 22}} {
		cols, rows := artworkSizeForCells(cell[0], cell[1])
		if cols*cell[0] != rows*cell[1] || cols <= 0 || rows <= 0 || cols > 30 || rows > 15 {
			t.Fatalf("cell %v: artwork %dx%d does not fit a square", cell, cols, rows)
		}
	}
	cols, rows := artworkSizeForCells(9, 19)
	if cols != 19 || rows != 9 {
		t.Fatalf("Kitty cover size = %dx%d, want 19x9", cols, rows)
	}
	for _, cell := range [][2]int{{0, 0}, {1, 100}, {100, 1}} {
		cols, rows := artworkSizeForCells(cell[0], cell[1])
		if cols != artworkCols || rows != artworkRows {
			t.Fatal("unsupported cell dimensions did not use safe defaults")
		}
	}
}

func TestKittyCoverAndLyricsBottomAlign(t *testing.T) {
	old := artworkMode
	artworkMode = artKitty
	t.Cleanup(func() { artworkMode = old })
	m := New(&fakeClient{})
	m.artwork = true
	m.width, m.height = 90, 30
	m.now = NowPlaying{Title: "Song", ArtistName: "Artist"}
	m.coverCols, m.coverRows = artworkSizeForCells(9, 19)
	m.artworkID = "1"
	updated, _ := m.Update(artworkMsg{catalogID: "1", cover: &artwork{png: []byte("image")}})
	m = updated.(Model)
	band := m.nowLines()
	if len(band) != 9 || !strings.Contains(band[0], "c=19,r=9,") ||
		!strings.Contains(ansi.Strip(band[len(band)-1]), "╯") {
		t.Fatal("cover placement and lyrics panel do not share their bottom row")
	}
	for _, row := range band {
		if ansi.StringWidth(row) != m.width {
			t.Fatal("aligned artwork band no longer fits the frame")
		}
	}
}

func TestArtworkHasSpaceAboveTimeBar(t *testing.T) {
	old := artworkMode
	t.Cleanup(func() { artworkMode = old })
	for _, mode := range []artProtocol{artBlock, artKitty} {
		artworkMode = mode
		m := New(&fakeClient{})
		m.width, m.height = 90, 30
		m.artwork = true
		m.now = NowPlaying{Title: "Song", ArtistName: "Artist", Duration: 200}
		m.coverLines = make([]string, artworkRows)
		lines := strings.Split(ansi.Strip(m.View()), "\n")
		gap := 1 + len(m.nowLines())
		if strings.TrimSpace(lines[gap]) != "" || !strings.HasPrefix(lines[gap+1], " [") {
			t.Fatal("artwork and time bar lack a blank separator row")
		}
		// Regular mode: lyrics band sits below the visualizer and the now-playing
		// line still sits directly above the seek bar (no rogue separator row).
		m.artwork = false
		m.lyricsOn = false
		lines = strings.Split(ansi.Strip(m.View()), "\n")
		if !strings.HasPrefix(lines[1+len(m.nowLines())], " [") {
			t.Fatal("regular mode gained an unnecessary separator row")
		}
	}
}

func TestVisualizerHasSpaceAboveArtwork(t *testing.T) {
	old := artworkMode
	artworkMode = artBlock
	t.Cleanup(func() { artworkMode = old })
	for _, full := range []bool{false, true} {
		m := New(&fakeClient{}).WithViz(make(chan []float64))
		m.width, m.height, m.artwork, m.fullViz = 90, 35, true, full
		m.now = NowPlaying{Title: "Song"}
		vizRows, _ := m.layout()
		if vizRows == 0 {
			t.Fatal("test requires a visible visualizer")
		}
		lines := strings.Split(ansi.Strip(m.View()), "\n")
		if strings.TrimSpace(lines[1+vizRows]) != "" || !strings.HasPrefix(lines[2+vizRows], "╭") {
			t.Fatal("visualizer and artwork lack their separator row")
		}
		if len(lines) > m.height {
			t.Fatal("artwork separator exceeded terminal height")
		}
	}
}
