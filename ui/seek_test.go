package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestSeekProgressSubcellSteps(t *testing.T) {
	for i, glyph := range []string{" ", "╸", "━"} {
		got := ansi.Strip(seekProgress(float64(i)/16, 8))
		if want := glyph + strings.Repeat(" ", 7); got != want {
			t.Fatalf("step %d = %q, want %q", i, got, want)
		}
	}
}

func TestSeekProgressClampsAndFits(t *testing.T) {
	for _, w := range []int{0, 1, 8, 80} {
		for _, progress := range []float64{-1, 0, 0.3, 1, 2} {
			got := ansi.Strip(seekProgress(progress, w))
			if ansi.StringWidth(got) != w {
				t.Fatalf("progress %v width %d: %q", progress, w, got)
			}
			if progress >= 1 && got != strings.Repeat("━", w) {
				t.Fatal("completed bar was not full")
			}
		}
	}
}

func TestSeekBorderAndOuterPadding(t *testing.T) {
	m := New(&fakeClient{})
	m.now = NowPlaying{Position: 65, Duration: 200}
	for _, w := range []int{20, 50, 80, 120} {
		rows := m.seekLines(w)
		for _, row := range rows {
			if ansi.StringWidth(row) > w {
				t.Fatalf("time bar overflows width %d", w)
			}
		}
		if w >= 50 {
			if len(rows) != 1 || !strings.HasPrefix(ansi.Strip(rows[0]), " [") ||
				strings.Count(ansi.Strip(rows[0]), "]") != 1 || strings.ContainsAny(rows[0], "█─") ||
				!strings.HasSuffix(ansi.Strip(rows[0]), " ") || ansi.StringWidth(rows[0]) != w {
				t.Fatalf("border or outer padding missing at width %d: %q", w, rows)
			}
		}
	}
}

func TestPlayerLayoutPreview(t *testing.T) {
	for _, art := range []bool{false, true} {
		m := New(&fakeClient{})
		m.width, m.height, m.artwork = 80, 24, art
		m.now = NowPlaying{ID: "1", Title: "Song title", ArtistName: "Artist", AlbumName: "Album", Position: 65, Duration: 200, Volume: 0.75}
		m.coverLines = make([]string, artworkRows)
		for i := range m.coverLines {
			m.coverLines[i] = strings.Repeat("█", artworkCols)
		}
		m.lyrics = []lyricLine{{start: 10, end: 60, text: "Previous lyric"}, {start: 60, end: 80, text: "Current lyric"}, {start: 80, end: 90, text: "Next lyric"}}
		m.lists[Queue] = list{loaded: true, rows: []Item{{ID: "1", Title: "Song title", Subtitle: "Artist"}, {ID: "2", Title: "Next song", Subtitle: "Artist"}}}
		view := ansi.Strip(m.View())
		if !strings.Contains(view, "╭─") || !strings.Contains(view, "1:05 / 3:20") || !strings.Contains(view, "Next song") {
			t.Fatalf("art=%v: player controls or browsing missing", art)
		}
		t.Logf("art=%v\n%s", art, view)
	}
}
