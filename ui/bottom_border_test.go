package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestBottomBorderEnclosesAllTabsAndFooter(t *testing.T) {
	old := artworkMode
	artworkMode = artBlock
	t.Cleanup(func() { artworkMode = old })
	for _, size := range [][2]int{{120, 40}, {80, 24}, {50, 22}} {
		for _, tab := range []Tab{Search, Library, Playlists, Queue} {
			for _, art := range []bool{false, true} {
				m := New(&fakeClient{})
				m.width, m.height, m.tab, m.artwork = size[0], size[1], tab, art
				m.now = NowPlaying{Title: "Song", ArtistName: "Artist", Duration: 200}
				m.coverLines = make([]string, artworkRows)
				m.lists[tab] = list{loaded: true, rows: []Item{{ID: "1", Title: "Browse row"}}}
				rows := strings.Split(ansi.Strip(m.View()), "\n")
				// Order: header, [viz], nowLines, [artwork gap OR lyrics band + breathing], seek, bottom panel.
				lyricsRows := 0
				if m.lyricsBandShown() {
					lyricsRows = m.lyricsBandRows() + 1 // +1 for the breathing row between lyrics and seek
				}
				top := 1 + len(m.nowLines()) + btoi(m.artworkShown()) + lyricsRows + len(m.seekLines(m.width))
				if !strings.HasPrefix(rows[top], "╭─") || !strings.Contains(rows[top], "Search") ||
					!strings.HasSuffix(rows[top], "╮") || ansi.StringWidth(rows[top]) != m.width ||
					!strings.HasPrefix(rows[len(rows)-1], "╰─ ") ||
					!strings.Contains(rows[len(rows)-1], "quit") ||
					!strings.HasSuffix(rows[len(rows)-1], "─╯") ||
					ansi.StringWidth(rows[len(rows)-1]) != m.width {
					t.Fatalf("size %v tab=%v art=%v: shared border missing", size, tab, art)
				}
				for _, row := range rows[top+1 : len(rows)-1] {
					if !strings.HasPrefix(row, "│") || !strings.HasSuffix(row, "│") || ansi.StringWidth(row) != m.width {
						t.Fatalf("bottom content escaped the shared border: %q", row)
					}
				}
				bottom := strings.Join(rows[top:], "\n")
				for _, text := range []string{"Search", "Browse row", "quit"} {
					if !strings.Contains(bottom, text) {
						t.Fatalf("size %v: %q missing inside border", size, text)
					}
				}
				if len(rows) > m.height || strings.Contains(bottom, "╭─ Queue") {
					t.Fatal("shared border overflows or retained the Queue-only frame")
				}
				_, listRows := m.layout()
				if len(rows) != top+listRows+2 {
					t.Fatalf("size %v: hints consumed a separate panel row", size)
				}
			}
		}
	}
}

func TestBottomBorderKeepsSmallAndFullVisualizerViewsUsable(t *testing.T) {
	m := New(&fakeClient{})
	m.width, m.height = 8, 5
	if m.bottomBordered() || len(strings.Split(m.View(), "\n")) > m.height {
		t.Fatal("tiny view reserved space for a bottom border")
	}
	m.width, m.height, m.fullViz = 80, 24, true
	m.viz = make(chan []float64)
	if m.bottomBordered() || m.bottomWidth() != m.width {
		t.Fatal("full-screen visualizer reserved a bottom panel")
	}
}

func TestBottomBorderCarriesStatusAtNarrowWidth(t *testing.T) {
	m := New(&fakeClient{})
	m.width, m.height = 12, 10
	m.status = strings.Repeat("warning ", 8)
	rows := strings.Split(ansi.Strip(m.View()), "\n")
	last := rows[len(rows)-1]
	if len(rows) > m.height || ansi.StringWidth(last) != m.width ||
		!strings.HasPrefix(last, "╰─ ") || !strings.HasSuffix(last, "─╯") {
		t.Fatalf("status should fit in the lower border: %q", last)
	}
}
