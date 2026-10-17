package ui

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestSidePanelKeepsMenuVisibleAndFits(t *testing.T) {
	for _, size := range [][2]int{{120, 30}, {80, 24}, {50, 24}, {20, 12}, {8, 5}} {
		m := New(&fakeClient{})
		m.width, m.height = size[0], size[1]
		m.now = NowPlaying{ID: "song", Title: "Playing", Position: 10}
		m.lyrics = []lyricLine{{start: 1, end: 20, text: "Lyric sample"}}
		m.lists[Queue].rows = []Item{{ID: "other", Title: "Browse song", Type: "songs"}}
		frame := m.View()
		lines := strings.Split(frame, "\n")
		if len(lines) > m.height {
			t.Fatalf("%v: too tall: %d", size, len(lines))
		}
		for _, line := range lines {
			if lipgloss.Width(line) > m.width {
				t.Fatalf("%v: too wide: %q", size, line)
			}
		}
		if m.width >= 50 {
			for _, text := range []string{"Browse song", "Lyrics", "Lyric sample"} {
				if !strings.Contains(frame, text) {
					t.Errorf("%v: missing %q:\n%s", size, text, frame)
				}
			}
		}
	}
}

func TestLyricsBandSitsBelowNowPlayingAndSpansWidth(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 30}, {200, 40}} {
		m := New(&fakeClient{})
		m.width, m.height = size[0], size[1]
		m.now = NowPlaying{ID: "song", Title: "Playing", ArtistName: "Artist"}
		m.lyrics = []lyricLine{{start: 1, end: 20, text: "Lyric sample"}}
		m.lists[Queue] = list{loaded: true, rows: []Item{{ID: "other", Title: "Browse song", Type: "songs"}}}
		frame := m.View()
		if !strings.Contains(frame, "Lyric sample") {
			t.Fatalf("%v: lyrics missing from view:\n%s", size, frame)
		}
		if !strings.Contains(frame, "Browse song") {
			t.Fatalf("%v: queue list missing from view:\n%s", size, frame)
		}
		// Lyrics band sits directly below the now-playing metadata; both
		// render above the queue/bottom panel.
		lines := strings.Split(frame, "\n")
		nowAt, lyricsAt, seekAt := -1, -1, -1
		for i, raw := range lines {
			plain := ansi.Strip(raw)
			switch {
			case nowAt == -1 && strings.Contains(plain, "Playing"):
				nowAt = i
			case lyricsAt == -1 && strings.Contains(plain, "Lyric sample"):
				lyricsAt = i
			case seekAt == -1 && strings.HasPrefix(plain, " ["):
				seekAt = i
			}
		}
		if nowAt < 0 || lyricsAt < 0 || lyricsAt <= nowAt {
			t.Fatalf("%v: lyrics should sit below now-playing: now=%d lyrics=%d\n%s", size, nowAt, lyricsAt, frame)
		}
		if seekAt >= 0 && lyricsAt >= seekAt {
			t.Fatalf("%v: lyrics should sit above seek bar: lyrics=%d seek=%d\n%s", size, lyricsAt, seekAt, frame)
		}
		// The band should span the full terminal width.
		for _, raw := range lines[lyricsAt:] {
			plain := strings.TrimRight(ansi.Strip(raw), " ")
			if plain == "" {
				break
			}
			if ansi.StringWidth(raw) != m.width {
				t.Fatalf("%v: lyrics row width %d, want %d: %q", size, ansi.StringWidth(raw), m.width, raw)
			}
		}
	}
}

func TestEmptyLyricsDoNotPanic(t *testing.T) {
	m := New(&fakeClient{})
	m.now.ID = "song"
	if !strings.Contains(strings.Join(m.lyricsLines(30, 3), " "), "No lyrics") {
		t.Fatal("empty lyrics should have a placeholder")
	}
}

func TestLyricsWrapInsidePanelAndKeepCurrentLineVisible(t *testing.T) {
	m := New(&fakeClient{})
	m.now = NowPlaying{ID: "song", Position: 12}
	m.lyrics = []lyricLine{
		{start: 1, text: "Earlier line with several words to wrap"},
		{start: 10, text: "Current line with several words to wrap across rows"},
		{start: 20, text: "Upcoming line with several words to wrap"},
	}
	rows := m.lyricsLines(24, 8)
	plain := make([]string, len(rows))
	for i, row := range rows {
		if ansi.StringWidth(row) > 24 {
			t.Fatalf("row %d exceeds panel: %q", i, row)
		}
		plain[i] = ansi.Strip(row)
	}
	joined := strings.Join(plain, "\n")
	if !strings.Contains(joined, "  ▸ Current line with") || !strings.Contains(joined, "    several words to") || !strings.Contains(joined, "    wrap across rows") {
		t.Fatalf("current lyric did not wrap with indented continuations:\n%s", joined)
	}
	if !strings.Contains(joined, "Upcoming line with") {
		t.Fatalf("wrapping hid the upcoming lyric:\n%s", joined)
	}
}

func TestTimedWordsWrapWithoutLosingHighlight(t *testing.T) {
	m := New(&fakeClient{})
	m.now = NowPlaying{ID: "song", Position: 12}
	m.lyrics = []lyricLine{{start: 10, end: 20, words: []lyricWord{
		{start: 10, text: "Hello"}, {start: 14, text: "beautiful"}, {start: 18, text: "world"},
	}}}
	rows := m.lyricsLines(14, 5)
	joined := strings.Join(rows, "\n")
	for _, word := range []string{"Hello", "beautiful", "world"} {
		if !strings.Contains(ansi.Strip(joined), word) {
			t.Fatalf("lost timed word %q: %q", word, joined)
		}
	}
	for _, row := range rows {
		if ansi.StringWidth(row) > 14 {
			t.Fatalf("timed word exceeds panel: %q", row)
		}
	}
}

func TestQueueStartupAndAlwaysVisibleLyrics(t *testing.T) {
	fake := &fakeClient{}
	m := New(fake)
	if m.tab != Queue || !strings.Contains(m.View(), "Lyrics") {
		t.Fatal("startup should show Queue beside lyrics")
	}
	batch := m.Init()().(tea.BatchMsg)
	queue, clock := false, false
	for _, cmd := range batch {
		switch cmd().(type) {
		case queueRefreshMsg:
			updated, load := m.Update(queueRefreshMsg{})
			m = updated.(Model)
			_, queue = load().(queueMsg)
		case lyricsFrameMsg:
			clock = true
		}
	}
	if !queue || !clock {
		t.Fatal("startup did not load the queue and start the lyrics clock")
	}
}

func TestYTogglesLyricsBandAndDoesNotHideQueue(t *testing.T) {
	for _, size := range [][2]int{{120, 30}, {80, 24}, {40, 16}, {8, 5}} {
		for _, withViz := range []bool{false, true} {
			m := New(&fakeClient{})
			m.width, m.height = size[0], size[1]
			m.now = NowPlaying{ID: "song", Title: "Playing"}
			m.lyrics = []lyricLine{{text: "Full width lyric line"}}
			m.lists[Queue] = list{loaded: true, rows: []Item{{ID: "other", Title: "Browse song"}}}
			if withViz {
				m = m.WithViz(make(chan []float64))
			}
			m, _ = sendKey(t, m, runeKey('y'))
			if !m.lyricsHidden {
				t.Fatalf("%v: y did not set lyricsHidden", size)
			}
			frame := m.View()
			if size[0] >= 24 && strings.Contains(frame, "Full width lyric line") {
				t.Fatalf("%v: lyrics still rendered after y:\n%s", size, frame)
			}
			// Tiny terminals cut the row down to "Brow…" (the column is
			// already truncated to 8 cells, then `fit()` truncates the whole
			// row to the terminal width); the row is still rendered, just
			// with a tiny prefix. The visible prefix proves the list
			// survived `y`.
			want := "Browse song"
			if size[0] < 12 {
				want = "Brow"
			}
			if !strings.Contains(frame, want) {
				t.Fatalf("%v: queue list hidden after y:\n%s", size, frame)
			}
			if len(strings.Split(frame, "\n")) > m.height {
				t.Fatal("frame exceeded terminal height after y")
			}
			for _, line := range strings.Split(frame, "\n") {
				if lipgloss.Width(line) > m.width {
					t.Fatalf("%v: line exceeded width: %q", size, line)
				}
			}
			m, _ = sendKey(t, m, runeKey('y'))
			if m.lyricsHidden {
				t.Fatalf("%v: y did not restore lyrics", size)
			}
			if !strings.Contains(m.View(), want) {
				t.Fatalf("%v: queue list vanished after restoring lyrics:\n%s", size, m.View())
			}
		}
	}
}

func TestDetailsExpandFollowSelectionAndCollapse(t *testing.T) {
	fake := &fakeClient{}
	m := New(fake)
	m.lists[Queue] = list{loaded: true, rows: []Item{
		{ID: "alpha-id", Type: "songs", Title: "First title", Subtitle: "First artist"},
		{ID: "beta-id", Type: "songs", Title: "Second title", Subtitle: "Second artist"},
	}}
	m, _ = sendKey(t, m, runeKey('d'))
	frame := m.View()
	if !m.expanded || !strings.Contains(frame, "alpha-id") || !strings.Contains(frame, "Lyrics") {
		t.Fatalf("selected item did not expand beside lyrics:\n%s", frame)
	}
	m, _ = sendKey(t, m, runeKey('j'))
	frame = m.View()
	if !strings.Contains(frame, "beta-id") || strings.Contains(frame, "alpha-id") {
		t.Fatalf("details did not follow selection:\n%s", frame)
	}
	m, _ = sendKey(t, m, runeKey('d'))
	if m.expanded || strings.Contains(m.View(), "beta-id") {
		t.Fatal("details did not collapse")
	}
	if len(fake.calls) != 0 {
		t.Fatalf("inspecting details made API calls: %+v", fake.calls)
	}
}

func TestMetadataHasDedicatedTitleAndDetailsRows(t *testing.T) {
	m := New(&fakeClient{})
	m.now = NowPlaying{Title: "Song title", ArtistName: "Artist", AlbumName: "Album"}
	rows := m.nowLines()
	if len(rows) != 2 || !strings.Contains(rows[0], "Song title") || strings.Contains(rows[0], "Artist") {
		t.Fatalf("title not given its own row: %q", rows)
	}
	if !strings.Contains(rows[1], "Artist") || !strings.Contains(rows[1], "Album") {
		t.Fatalf("details missing: %q", rows)
	}
	if m.fixedRows() < 6+len(m.footerLines(m.bottomWidth())) {
		t.Fatal("layout did not reserve both metadata rows")
	}
}

func TestLyricsLoadingResetsAndLateMetadataCanRetry(t *testing.T) {
	m := New(&fakeClient{})
	m.lyricsLoading = true
	m.lyricsID = "old"
	m.now.ID = "new"
	if m.ensureLyrics() != nil || m.lyricsLoading || m.lyricsErr != errNoLyrics {
		t.Fatal("track without lyrics kept the old loading state")
	}
	m.now.HasLyrics, m.now.CatalogID = true, "123"
	if m.ensureLyrics() == nil || !m.lyricsLoading {
		t.Fatal("late lyrics metadata could not start a fetch")
	}
}

func TestRemoteTextCannotEmitTerminalControls(t *testing.T) {
	unsafe := "\x1b]0;injected title\aSong\x1b[2J\n\t"
	if got := displayText(unsafe); got != "Song" {
		t.Fatalf("terminal controls survived: %q", got)
	}
	m := New(&fakeClient{})
	m.now = NowPlaying{ID: "song", Title: unsafe, ArtistName: unsafe, AlbumName: unsafe}
	m.lyrics = []lyricLine{{text: unsafe}}
	frame := m.View()
	if strings.Contains(frame, "injected title") || strings.Contains(frame, "\x1b[2J") {
		t.Fatal("metadata or lyrics emitted terminal controls")
	}
}

func TestColoredThemeFrameFitsPanels(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		_ = applyTheme("Terminal")
		lipgloss.SetColorProfile(profile)
	})
	m := New(&fakeClient{})
	m.now.ID = "song"
	for _, theme := range themeNames {
		if err := applyTheme(theme); err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(m.View(), "\n") {
			if lipgloss.Width(line) != m.width {
				t.Fatalf("%s: frame width %d, want %d", theme, lipgloss.Width(line), m.width)
			}
		}
	}
}

func TestLyricsClockRunsWithoutVisualizer(t *testing.T) {
	m := New(&fakeClient{})
	m.now.Playing, m.lyrics = true, []lyricLine{{}}
	_, cmd := m.Update(lyricsFrameMsg{})
	if cmd == nil {
		t.Fatal("no lyrics clock without visualizer")
	}
	if _, ok := cmd().(lyricsFrameMsg); !ok {
		t.Fatal("clock did not schedule the next lyrics frame")
	}
}

func TestLiveWordProgress(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	word := lyricWord{start: 10, text: "Hello"}
	if got := liveWord(word, 15, 12); got != selectedStyle.Render("He")+textStyle.Render("llo") {
		t.Fatalf("not progressively highlighted: %q", got)
	}
	for _, pos := range []float64{9, 10, 12, 15, 16} {
		if got := ansi.Strip(liveWord(word, 15, pos)); got != "Hello" {
			t.Fatalf("word changed at %v: %q", pos, got)
		}
	}
}

func TestOldPollCannotRewindLyrics(t *testing.T) {
	m := New(&fakeClient{})
	m.polledAt = time.Now()
	m.now = NowPlaying{ID: "new", Position: 30}
	updated, _ := m.Update(pollMsg{at: m.polledAt.Add(-time.Second), now: NowPlaying{ID: "old", Position: 10}})
	if got := updated.(Model).now; got.ID != "new" || got.Position != 30 {
		t.Fatalf("old poll rewound lyrics: %+v", got)
	}
}

func TestSettingsKeyAppliesAndSavesTheme(t *testing.T) {
	t.Cleanup(func() { _ = applyTheme("Terminal") })
	m := New(&fakeClient{})
	m.settingsPath = filepath.Join(t.TempDir(), "settings.json")
	m, _ = sendKey(t, m, runeKey(','))
	if !m.settings || !strings.Contains(m.View(), "Settings") {
		t.Fatal("settings did not open")
	}
	m, _ = sendKey(t, m, runeKey('j'))
	m, _ = sendKey(t, m, specialKey(tea.KeyEnter))
	name, err := loadTheme(m.settingsPath)
	if err != nil || name != themeNames[1] || m.theme != name {
		t.Fatalf("theme not persisted: %q %v", name, err)
	}
}

func TestSettingsSelectsAndRestoresWaveform(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := New(&fakeClient{})
	m, _ = sendKey(t, m, runeKey(','))
	for range len(themeNames) + 1 {
		m, _ = sendKey(t, m, runeKey('j'))
	}
	if !strings.Contains(m.View(), "Visualizer: Spectrum") {
		t.Fatal("visualizer setting not visible")
	}
	m, _ = sendKey(t, m, specialKey(tea.KeyEnter))
	if !m.waveform || !strings.Contains(m.View(), "Visualizer: Waveform") {
		t.Fatal("visualizer did not switch")
	}
	reloaded := New(&fakeClient{}).WithSettings()
	if !reloaded.waveform {
		t.Fatal("waveform preference not restored")
	}
}

func TestSettingsTogglesAndPersistsAlbumArt(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := New(&fakeClient{})
	m, _ = sendKey(t, m, runeKey(','))
	for range len(themeNames) + 2 {
		m, _ = sendKey(t, m, runeKey('j'))
	}
	if !strings.Contains(m.View(), "Album art: Off") {
		t.Fatal("album art setting not visible")
	}
	m, _ = sendKey(t, m, specialKey(tea.KeyEnter))
	if !m.artwork || !strings.Contains(m.View(), "Album art: On") {
		t.Fatal("album art did not switch on")
	}
	reloaded := New(&fakeClient{}).WithSettings()
	if !reloaded.artwork {
		t.Fatal("album art preference not restored")
	}
}

func TestArtworkToggleRemovesCoverFromView(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	orig := artworkMode
	artworkMode = artBlock
	defer func() { artworkMode = orig }()

	m := New(&fakeClient{})
	m.artwork = true
	m.width = 90
	m.height = 30
	m.now = NowPlaying{ID: "i.1", CatalogID: "123", Title: "Song", ArtistName: "Artist", AlbumName: "Album"}
	m.coverLines = renderArtwork(image.NewRGBA(image.Rect(0, 0, artworkWide, artworkHigh)))

	if !strings.Contains(m.View(), "█") {
		t.Fatalf("cover missing before toggle:\n%s", m.View())
	}
	// Open settings, move to Album art, press enter to switch it off.
	m.settingsPath = filepath.Join(t.TempDir(), "settings.json")
	m, _ = sendKey(t, m, runeKey(','))
	for range len(themeNames) + 2 {
		m, _ = sendKey(t, m, runeKey('j'))
	}
	m, _ = sendKey(t, m, specialKey(tea.KeyEnter))
	if m.artwork {
		t.Fatal("artwork did not switch off")
	}
	m.settings = false
	if view := m.View(); strings.Contains(view, "█") {
		t.Fatalf("cover still present after toggle off:\n%s", view)
	}
}

func TestInvalidSavedThemeRestoresTerminalPalette(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Cleanup(func() { _ = applyTheme("Terminal") })
	path, err := themeSettingsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := saveTheme(path, "Dark"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"theme":"unknown"}`), 0600); err != nil {
		t.Fatal(err)
	}
	_ = applyTheme("Dark")
	m := New(&fakeClient{}).WithSettings()
	if m.theme != "Terminal" || themeBackground != nil || themeForeground != nil {
		t.Fatal("invalid theme did not restore terminal palette")
	}
}

func TestLyricsWrapAtWordsWithMargin(t *testing.T) {
	m := Model{}
	m.now.ID = "x"
	m.lyrics = []lyricLine{{text: "I've been staring at the edge of the water long as I can remember"}}
	for _, row := range m.lyricsLines(26, 10) {
		plain := strings.TrimRight(ansi.Strip(row), " ")
		if ansi.StringWidth(plain) > 25 {
			t.Fatalf("row touches right edge: %q", plain)
		}
		for _, frag := range []string{"th", "wat", "remem"} {
			if strings.HasSuffix(plain, " "+frag) {
				t.Fatalf("word split mid-way: %q", plain)
			}
		}
	}
}

func TestWrappedCurrentLyricStaysStyled(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		_ = applyTheme("Terminal")
		lipgloss.SetColorProfile(profile)
	})
	if err := applyTheme(themeNames[1]); err != nil {
		t.Fatal(err)
	}
	m := Model{}
	m.now.ID, m.now.Position = "x", 5
	m.lyrics = []lyricLine{
		{text: "I've been staring at the edge of the water long as I can", start: 1, end: 100},
		{text: "next", start: 100, end: 200},
	}
	want := selectedStyle.Render("x")
	want = want[:strings.Index(want, "x")] // the style's opening sequence
	var wrapped int
	for _, row := range m.lyricsLines(26, 6) {
		if ansi.Strip(row) == "" || strings.Contains(row, "next") {
			continue
		}
		wrapped++
		if strings.Count(row, want) < 2 { // indent and text both styled
			t.Fatalf("current-line row lost its style: %q", row)
		}
	}
	if wrapped < 2 {
		t.Fatalf("expected the line to wrap, got %d rows", wrapped)
	}
}
