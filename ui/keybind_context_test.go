package ui

// Tests for CID-30: keybind context polish.

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// TestEscapeLeavesFullscreenViz: pressing esc in full-screen visualizer
// mode should return to the normal layout. It sits behind help/expanded
// in priority — if the help overlay is open, that closes first; only
// afterwards does esc drop the visualizer.
func TestEscapeLeavesFullscreenViz(t *testing.T) {
	for _, first := range []tea.KeyMsg{runeKey('v'), specialKey(tea.KeyEsc)} {
		t.Run(first.String(), func(t *testing.T) {
			m := New(&fakeClient{}).WithViz(make(chan []float64))
			m.width, m.height = 80, 24
			m.now = NowPlaying{Title: "Song"}
			m, _ = sendKey(t, m, runeKey('v')) // enter fullscreen
			if !m.fullScreenViz() {
				t.Fatal("v did not enter fullscreen")
			}
			m, _ = sendKey(t, m, first) // esc / v should exit
			if m.fullScreenViz() {
				t.Fatalf("%s did not leave fullscreen", first.String())
			}
			if m.fullViz {
				t.Fatalf("%s did not clear fullViz flag", first.String())
			}
		})
	}
}

// TestHelpBlockedFromListActions: while help is open, list-acting keys
// must not change state. A stray j/k must not move the cursor, a stray
// enter must not play a row, and so on. Only ?, esc and transport keys
// pass through.
func TestHelpBlockedFromListActions(t *testing.T) {
	m := New(&fakeClient{})
	m.width, m.height = 80, 24
	m.now = NowPlaying{ID: "song", Title: "Playing"}
	m.lists[Queue] = list{loaded: true, rows: []Item{
		{ID: "1", Title: "Row one", Type: "songs"},
		{ID: "2", Title: "Row two", Type: "songs"},
		{ID: "3", Title: "Row three", Type: "songs"},
	}}
	m, _ = sendKey(t, m, runeKey('?'))
	if !m.help {
		t.Fatal("? did not open help")
	}
	before := m.lists[Queue].cursor
	for _, blocked := range []string{"j", "k", "d", "x", "a", "P", "Y", "l", "o", "y", "v", "b", "tab", "1"} {
		before2 := m.lists[Queue].cursor
		m, _ = sendKey(t, m, runeKey(rune(blocked[0])))
		if m.lists[Queue].cursor != before2 {
			t.Fatalf("%q moved cursor %d -> %d while help was open", blocked, before2, m.lists[Queue].cursor)
		}
	}
	// ? and esc still close the overlay.
	m, _ = sendKey(t, m, runeKey('?'))
	if m.help {
		t.Fatal("? did not close help")
	}
	m.help = true
	m, _ = sendKey(t, m, specialKey(tea.KeyEsc))
	if m.help {
		t.Fatal("esc did not close help")
	}
	// Sanity: with help closed, j moves the cursor.
	m, _ = sendKey(t, m, runeKey('j'))
	if m.lists[Queue].cursor != before+1 {
		t.Fatalf("j did not move cursor after help closed: %d -> %d", before, m.lists[Queue].cursor)
	}
}

// TestHelpReachableInFullscreenViz: pressing ? in full-screen visualizer
// mode renders the help overlay instead of an empty frame. The test covers
// the default configuration (lyrics on, full screen, viz available) at the
// common 80×24 size — that's the case where the help was being clipped
// because the lyrics band was eating rows the help expected.
func TestHelpReachableInFullscreenViz(t *testing.T) {
	for _, size := range [][2]int{{120, 30}, {80, 24}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m := New(&fakeClient{}).WithViz(make(chan []float64))
			m.width, m.height = size[0], size[1]
			// Default config: keep lyrics on and supply synced lyrics
			// so the band actually renders, the way a user sees it.
			m.now = NowPlaying{ID: "song", Title: "Song", ArtistName: "Artist"}
			m.lyrics = []lyricLine{{start: 1, end: 20, text: "Sample lyric"}}
			m, _ = sendKey(t, m, runeKey('v'))
			m, _ = sendKey(t, m, runeKey('?'))
			if !m.help {
				t.Fatal("? did not open help inside fullscreen")
			}
			frame := m.View()
			plain := ansi.Strip(frame)
			// Help lists every key; the bottom rows (copy, search, quit)
			// must remain visible. Check the trailing entries — those were
			// the ones clipped before the row-count math was fixed.
			for _, want := range []string{"play / pause", "settings", "spectrum", "quit"} {
				if !strings.Contains(plain, want) {
					t.Fatalf("fullscreen help missing %q:\n%s", want, plain)
				}
			}
			if len(strings.Split(frame, "\n")) > m.height {
				t.Fatalf("frame exceeded terminal height: %d > %d", len(strings.Split(frame, "\n")), m.height)
			}
		})
	}
}

// TestFooterExplainsFilteredQueue: with a filter on the Queue tab, the
// footer must drop the now-dead jump/remove/clear hints and tell the
// user how to get back.
func TestFooterExplainsFilteredQueue(t *testing.T) {
	m := New(&fakeClient{})
	m.width, m.height = 120, 30
	m.now = NowPlaying{Title: "Song"}
	m.tab = Queue
	m.lists[Queue] = list{loaded: true, rows: []Item{
		{ID: "1", Title: "First", Type: "songs"},
		{ID: "2", Title: "Second", Type: "songs"},
	}}
	// The Queue tab without a filter advertises jump/remove/clear; that
	// baseline makes the after-filter check meaningful.
	unfiltered := stripANSI(strings.Join(m.footerLines(m.bottomWidth()), "\n"))
	if !strings.Contains(unfiltered, "jump") || !strings.Contains(unfiltered, "remove") || !strings.Contains(unfiltered, "clear") {
		t.Fatalf("unfiltered Queue footer should mention jump/remove/clear: %s", unfiltered)
	}
	// Apply a filter by pressing / and typing.
	m, _ = sendKey(t, m, runeKey('/'))
	m, _ = sendKey(t, m, runeKey('f'))
	m, _ = sendKey(t, m, runeKey('i'))
	m, _ = sendKey(t, m, runeKey('r'))
	m, _ = sendKey(t, m, specialKey(tea.KeyEnter))
	if m.lists[Queue].filter != "fir" {
		t.Fatalf("filter not applied: %q", m.lists[Queue].filter)
	}
	filtered := stripANSI(strings.Join(m.footerLines(m.bottomWidth()), "\n"))
	if !strings.Contains(filtered, "clear filter") {
		t.Fatalf("filtered footer missing the clear-filter hint: %s", filtered)
	}
	for _, banned := range []string{"jump", "remove", "clear queue"} {
		if strings.Contains(filtered, banned) {
			t.Fatalf("filtered footer still mentions %q: %s", banned, filtered)
		}
	}
}

// TestClearQueueIgnoredWhenFiltered: the `c` key shouldn't reach Cider
// when the Queue tab has an active filter; queueKey already no-ops on
// `x` and `enter` while filtered, but `c` was previously unguarded.
func TestClearQueueIgnoredWhenFiltered(t *testing.T) {
	fake := &fakeClient{}
	m := New(fake)
	m.width, m.height = 80, 24
	m.now = NowPlaying{Title: "Song"}
	m.tab = Queue
	m.lists[Queue] = list{loaded: true, rows: []Item{{ID: "1", Title: "Row", Type: "songs"}}}
	m.lists[Queue].filter = "x"
	m, _ = sendKey(t, m, runeKey('c'))
	if len(fake.calls) != 0 {
		t.Fatalf("c reached the client while filtered: %+v", fake.calls)
	}
}

// TestFooterOmitsInactiveControls: the footer's forced `?` and `q`
// hints only apply when those keys actually work. While typing into a
// search/filter field, `?` and `q` are typed into the query and `?` is
// not the help key. Settings mode shows its own nav keys; the
// unconditional essential-list also made `q` redundant alongside the
// up/down/enter hints there.
func TestFooterOmitsInactiveControls(t *testing.T) {
	for _, tc := range []struct {
		name     string
		setup    func() Model
		contains []string
		missing  []string
	}{
		{
			name: "search input",
			setup: func() Model {
				m := New(&fakeClient{})
				m.width, m.height = 120, 30
				m.now = NowPlaying{Title: "Song"}
				m.tab = Search
				m.input = true
				return m
			},
			contains: []string{"search", "esc"},
			missing:  []string{"keys", "quit"},
		},
		{
			name: "settings mode",
			setup: func() Model {
				m := New(&fakeClient{})
				m.width, m.height = 120, 30
				m.now = NowPlaying{Title: "Song"}
				m.settings = true
				return m
			},
			contains: []string{"select", "apply", "back"},
			missing:  []string{"keys"},
		},
		{
			name: "normal mode keeps ? and q",
			setup: func() Model {
				m := New(&fakeClient{})
				m.width, m.height = 120, 30
				m.now = NowPlaying{Title: "Song"}
				return m
			},
			contains: []string{"keys", "quit"},
			missing:  nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.setup()
			plain := ansi.Strip(strings.Join(m.footerLines(m.bottomWidth()), "\n"))
			for _, want := range tc.contains {
				if !strings.Contains(plain, want) {
					t.Fatalf("%s footer missing %q: %s", tc.name, want, plain)
				}
			}
			for _, banned := range tc.missing {
				if strings.Contains(plain, banned) {
					t.Fatalf("%s footer still shows %q: %s", tc.name, banned, plain)
				}
			}
		})
	}
}

// TestHelpYLabelIsAccurate: the help view's `y` line must say what the
// key actually does — toggle the lyrics band's hidden state — not the
// stale "expand / restore".
func TestHelpYLabelIsAccurate(t *testing.T) {
	m := New(&fakeClient{})
	m.width, m.height = 120, 30
	m.now = NowPlaying{Title: "Song"}
	m.help = true
	frame := m.View()
	plain := stripANSI(frame)
	if strings.Contains(plain, "expand / restore lyrics") {
		t.Fatalf("stale help label still present:\n%s", plain)
	}
	if !strings.Contains(plain, "hide / restore lyrics") {
		t.Fatalf("help missing the y label:\n%s", plain)
	}
}

func stripANSI(s string) string { return ansi.Strip(s) }
