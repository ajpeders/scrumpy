package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestKittyArtworkOffDeletesPlacement(t *testing.T) {
	old := artworkMode
	artworkMode = artKitty
	t.Cleanup(func() { artworkMode = old })
	m := New(&fakeClient{})
	m.artwork = true
	m.now = NowPlaying{Title: "Song", ID: "123", CatalogID: "123"}
	m.artworkID = "123"
	m.coverLines = make([]string, artworkRows)
	m.coverLines[0] = kittyImage([]byte("image"), artworkCols, artworkRows)
	m.settingsPath = filepath.Join(t.TempDir(), "settings.json")
	m.settings = true
	m.themeCursor = len(themeNames) + 2
	if strings.Contains(m.View(), "a=d,d=I,i=1,") {
		t.Fatal("visible cover was deleted")
	}
	band := m.nowLines()
	if !strings.HasPrefix(band[0], "\x1b_G") || strings.HasPrefix(ansi.Strip(band[0]), "╭") {
		t.Fatal("kitty image should use its pixel border, not a text border")
	}
	for _, hide := range []func(*Model){
		func(m *Model) { m.width = 30 },
		func(m *Model) { m.height = 12 },
		func(m *Model) { m.offline = true },
		func(m *Model) { m.now.Title = "" },
	} {
		hidden := m
		hide(&hidden)
		if frame := hidden.View(); !strings.Contains(frame, "a=d,d=I,i=1,") || strings.Contains(frame, "a=T") {
			t.Fatal("hiding artwork did not remove its terminal placement")
		}
	}
	m, _ = sendKey(t, m, specialKey(tea.KeyEnter))
	frame := m.View()
	if m.artwork || !strings.Contains(frame, "a=d,d=I,i=1,q=2;") || strings.Contains(frame, "a=T") {
		t.Fatal("turning art off did not delete the terminal image")
	}
}

func TestArtworkLayoutSizes(t *testing.T) {
	for _, size := range [][2]int{{120, 40}, {80, 24}, {44, 24}, {43, 24}, {80, 15}, {20, 8}} {
		for _, art := range []bool{false, true} {
			for _, lyrics := range []bool{false, true} {
				m := New(&fakeClient{})
				m.width, m.height = size[0], size[1]
				m.artwork, m.lyricsOn = art, lyrics
				m.now = NowPlaying{ID: "1", Title: "Long song title 日本語 🎵", ArtistName: "Artist", AlbumName: "Album"}
				m.coverLines = make([]string, artworkRows)
				for i := range m.coverLines {
					m.coverLines[i] = strings.Repeat("█", artworkCols)
				}
				view := m.View()
				rows := strings.Split(view, "\n")
				if len(rows) > m.height {
					t.Fatalf("size %v art=%v lyrics=%v: frame too tall", size, art, lyrics)
				}
				for _, row := range rows {
					if ansi.StringWidth(row) > m.width {
						t.Fatalf("size %v art=%v lyrics=%v: frame too wide", size, art, lyrics)
					}
				}
				if m.artworkShown() {
					_, listRows := m.layout()
					if listRows < 3 {
						t.Fatal("artwork crowded out browsing")
					}
				} else if strings.Contains(view, strings.Repeat("█", artworkCols)) {
					t.Fatal("hidden artwork still rendered")
				}
			}
		}
	}
}

func TestArtworkFetchSurvivesToggleAndTrackChange(t *testing.T) {
	m := New(&fakeClient{})
	m.artwork = true
	m.now.CatalogID = "first"
	if m.ensureArtwork() == nil {
		t.Fatal("initial fetch missing")
	}
	m.artwork = false
	m.ensureArtwork()
	m.artwork = true
	if m.ensureArtwork() == nil {
		t.Fatal("in-flight fetch prevented re-enabling artwork")
	}
	m.now.CatalogID = "second"
	if m.ensureArtwork() == nil || m.artworkID != "second" {
		t.Fatal("old in-flight fetch prevented fetching the new track")
	}
	updated, _ := m.Update(artworkMsg{catalogID: "first"})
	m = updated.(Model)
	if !m.artworkLoading || m.artworkID != "second" {
		t.Fatal("stale result affected the current fetch")
	}
}

func TestArtworkFailureRetriesWithBackoff(t *testing.T) {
	m := New(&fakeClient{})
	m.artwork = true
	m.now.CatalogID = "1"
	m.ensureArtwork()
	updated, _ := m.Update(artworkMsg{catalogID: "1", err: os.ErrNotExist})
	m = updated.(Model)
	if m.artworkRetryAt.IsZero() || m.ensureArtwork() != nil {
		t.Fatal("failed artwork should wait before retrying")
	}
	m.artworkRetryAt = time.Now().Add(-time.Second)
	if m.ensureArtwork() == nil || !m.artworkLoading {
		t.Fatal("failed artwork was not retried")
	}
}

func TestArtworkTrackChangesKeepVisualizerHeight(t *testing.T) {
	old := artworkMode
	artworkMode = artKitty
	t.Cleanup(func() { artworkMode = old })
	m := New(&fakeClient{})
	m.width, m.height = 90, 35
	m.artwork = true
	m.viz = make(chan []float64)
	m.now = NowPlaying{Title: "First song", CatalogID: "first", ArtistName: "Artist"}
	m.artworkID = "first"
	m.coverLines = make([]string, artworkRows)
	m.coverLines[0] = kittyImage([]byte("image"), artworkCols, artworkRows)
	wantViz, wantList := m.layout()
	_, wantRows := m.artworkBandSize()
	check := func(stage string) {
		t.Helper()
		vizRows, listRows := m.layout()
		if vizRows != wantViz || listRows != wantList || len(m.nowLines()) != wantRows {
			t.Fatalf("%s resized layout: viz=%d list=%d; want %d,%d", stage, vizRows, listRows, wantViz, wantList)
		}
	}
	// Cider can briefly omit metadata between songs.
	m.now = NowPlaying{}
	m.ensureArtwork()
	check("between tracks")
	m.now = NowPlaying{Title: "Second song", CatalogID: "second"}
	m.ensureArtwork()
	check("cover loading")
	if frame := m.View(); !strings.Contains(frame, "a=d,d=I,i=1,") || strings.Contains(frame, "a=T") {
		t.Fatal("old terminal cover remained visible while the new one loaded")
	}
	updated, _ := m.Update(artworkMsg{catalogID: "second", err: os.ErrNotExist})
	m = updated.(Model)
	check("cover unavailable")
	m.coverLines = make([]string, artworkRows)
	check("cover loaded")
}

func TestLyricsTogglePersistsAndHidesPanels(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := New(&fakeClient{}).WithSettings()
	m.settings = true
	m.themeCursor = len(themeNames) + 3
	m.lyricsHidden = true
	m, _ = sendKey(t, m, specialKey(tea.KeyEnter))
	if m.lyricsOn || m.lyricsHidden {
		t.Fatal("lyrics did not turn off")
	}
	if New(&fakeClient{}).WithSettings().lyricsOn {
		t.Fatal("lyrics off was not restored")
	}
	m.settings = false
	m.now = NowPlaying{ID: "123", CatalogID: "123", HasLyrics: true, Title: "Song"}
	if m.ensureLyrics() != nil {
		t.Fatal("disabled lyrics started a fetch")
	}
	if strings.Join(m.menuLines(80, 10), "\n") != strings.Join(m.listLines(80, 10), "\n") {
		t.Fatal("disabled lyrics still reserved sidebar space")
	}
	m, _ = sendKey(t, m, runeKey('y'))
	if m.lyricsHidden {
		t.Fatal("y opened disabled lyrics")
	}
	m.settings = true
	m, _ = sendKey(t, m, specialKey(tea.KeyEnter))
	if !m.lyricsOn || !New(&fakeClient{}).WithSettings().lyricsOn {
		t.Fatal("lyrics did not turn back on and persist")
	}
}

func TestYHidesLyricsBandAndQueuePanel(t *testing.T) {
	m := New(&fakeClient{})
	m.width, m.height = 80, 28
	m.now = NowPlaying{ID: "1", Title: "Song", ArtistName: "Artist", AlbumName: "Album"}
	m.lyrics = []lyricLine{{start: 1, end: 2, text: "Lyric text"}, {start: 2, end: 3, text: "Second line"}}
	m.lists[Queue] = list{loaded: true, rows: []Item{{ID: "q1", Title: "Browse song"}}}
	before := m.View()
	if !strings.Contains(before, "Lyric text") {
		t.Fatalf("lyrics not visible before y:\n%s", before)
	}
	if !strings.Contains(before, "Song") {
		t.Fatalf("metadata not visible before y:\n%s", before)
	}
	// Lyrics band sits below the now-playing line in the new layout.
	lines := strings.Split(before, "\n")
	lyricsAt, songAt := -1, -1
	for i, raw := range lines {
		plain := ansi.Strip(raw)
		if songAt == -1 && strings.Contains(plain, "Song") {
			songAt = i
		}
		if lyricsAt == -1 && strings.Contains(plain, "Lyric text") {
			lyricsAt = i
		}
	}
	if songAt < 0 || lyricsAt < 0 || lyricsAt <= songAt {
		t.Fatalf("lyrics should sit below now-playing: song=%d lyrics=%d\n%s", songAt, lyricsAt, before)
	}
	m, _ = sendKey(t, m, runeKey('y'))
	if !m.lyricsHidden {
		t.Fatal("y did not hide lyrics")
	}
	after := m.View()
	if strings.Contains(after, "Lyric text") || strings.Contains(after, "Lyric ") || strings.Contains(after, "Loading lyrics") {
		t.Fatalf("lyrics still rendered after y:\n%s", after)
	}
	if !strings.Contains(after, "Browse song") {
		t.Fatalf("queue list hidden by y:\n%s", after)
	}
	if !strings.Contains(after, "Song") {
		t.Fatalf("metadata hidden by y:\n%s", after)
	}
	// Pressing y again restores the band.
	m, _ = sendKey(t, m, runeKey('y'))
	if m.lyricsHidden {
		t.Fatal("y did not restore lyrics")
	}
	restored := m.View()
	if !strings.Contains(restored, "Lyric text") {
		t.Fatalf("lyrics missing after second y:\n%s", restored)
	}
}

func TestFullScreenVizExpandsLyricsBand(t *testing.T) {
	for _, height := range []int{24, 30, 40} {
		m := New(&fakeClient{}).WithViz(make(chan []float64))
		m.width, m.height = 120, height
		m.now = NowPlaying{ID: "song", Title: "Playing", ArtistName: "Artist", AlbumName: "Album"}
		m.lyrics = []lyricLine{{start: 1, end: 20, text: "Lyric sample"}}

		normalRows := m.lyricsBandRows()
		m.fullViz = true
		fullRows := m.lyricsBandRows()
		m.fullViz = false

		if fullRows <= normalRows {
			t.Fatalf("height=%d: fullscreen viz did not expand lyrics: normal=%d full=%d", height, normalRows, fullRows)
		}

		m, _ = sendKey(t, m, runeKey('v'))
		if !m.fullViz {
			t.Fatalf("height=%d: v did not enter fullscreen", height)
		}
		frame := m.View()
		if !strings.Contains(frame, "Lyric sample") {
			t.Fatalf("height=%d: lyrics missing in fullscreen viz:\n%s", height, frame)
		}
		lines := strings.Split(frame, "\n")
		if len(lines) > m.height {
			t.Fatalf("height=%d: fullscreen exceeded terminal height: %d", height, len(lines))
		}

		// y still hides lyrics in fullscreen mode.
		m, _ = sendKey(t, m, runeKey('y'))
		if !m.lyricsHidden {
			t.Fatalf("height=%d: y did not hide lyrics in fullscreen", height)
		}
		if strings.Contains(m.View(), "Lyric sample") {
			t.Fatalf("height=%d: lyrics still rendered after y in fullscreen", height)
		}
	}
}

func TestLegacySettingsEnableLyrics(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"theme":"Terminal"}`), 0600); err != nil {
		t.Fatal(err)
	}
	config, err := loadAppearance(path)
	if err != nil || !config.Lyrics {
		t.Fatalf("legacy lyrics default: %+v, %v", config, err)
	}
}
