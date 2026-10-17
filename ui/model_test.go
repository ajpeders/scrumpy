package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type call struct {
	method string
	path   string
	body   any
}

// fakeClient records calls and serves canned AMAPI responses, so model tests
// assert the exact HTTP surface without a running Cider.
type fakeClient struct {
	calls     []call
	getErr    error
	postErr   error
	getBodies map[string]string // path -> JSON served by Get
	amapi     func(path string, out any) error
}

func (f *fakeClient) Get(path string, out any) error {
	f.calls = append(f.calls, call{method: "GET", path: path})
	if body, ok := f.getBodies[path]; ok && out != nil {
		return json.Unmarshal([]byte(body), out)
	}
	return f.getErr
}

func (f *fakeClient) Post(path string, body, out any) error {
	f.calls = append(f.calls, call{method: "POST", path: path, body: body})
	return f.postErr
}

func (f *fakeClient) AMAPI(path string, out any) error {
	f.calls = append(f.calls, call{method: "AMAPI", path: path})
	if f.amapi != nil {
		return f.amapi(path, out)
	}
	return nil
}

func (f *fakeClient) last() call { return f.calls[len(f.calls)-1] }

func runeKey(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

func specialKey(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

// sendKey delivers a key and executes the returned command, returning the
// message it produces (nil when the key has no side effect).
func sendKey(t *testing.T, m Model, k tea.KeyMsg) (Model, tea.Msg) {
	t.Helper()
	updated, cmd := m.Update(k)
	next, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T", updated)
	}
	if cmd == nil {
		return next, nil
	}
	return next, cmd()
}

func TestTransportKeysMapToRoutes(t *testing.T) {
	cases := []struct {
		key   tea.KeyMsg
		route string
	}{
		{specialKey(tea.KeySpace), "/api/v1/playback/playpause"},
		{runeKey('n'), "/api/v1/playback/next"},
		{runeKey('p'), "/api/v1/playback/previous"},
		{runeKey('r'), "/api/v1/playback/toggle-repeat"},
	}
	for _, tc := range cases {
		fake := &fakeClient{}
		_, _ = sendKey(t, New(fake), tc.key)
		if len(fake.calls) != 1 || fake.last().method != "POST" || fake.last().path != tc.route {
			t.Errorf("%q: calls = %+v, want POST %s", tc.key.String(), fake.calls, tc.route)
		}
	}
}

func TestSeekAndVolumeClamp(t *testing.T) {
	seek := func(key tea.KeyMsg, position float64) float64 {
		t.Helper()
		fake := &fakeClient{}
		m := New(fake)
		m.now.Position = position
		_, _ = sendKey(t, m, key)
		body, ok := fake.last().body.(map[string]any)
		if !ok || fake.last().path != "/api/v1/playback/seek" {
			t.Fatalf("seek call = %+v", fake.last())
		}
		value, ok := body["position"].(float64)
		if !ok {
			t.Fatalf("position value = %#v", body["position"])
		}
		return value
	}
	if got := seek(specialKey(tea.KeyLeft), 10); got != 5 {
		t.Errorf("left from 10s = %v, want 5", got)
	}
	if got := seek(specialKey(tea.KeyLeft), 2); got != 0 {
		t.Errorf("left from 2s = %v, want 0 (clamp)", got)
	}
	if got := seek(specialKey(tea.KeyRight), 10); got != 15 {
		t.Errorf("right from 10s = %v, want 15", got)
	}

	volume := func(key tea.KeyMsg, value float64) float64 {
		t.Helper()
		fake := &fakeClient{}
		m := New(fake)
		m.now.Volume = value
		_, _ = sendKey(t, m, key)
		body, ok := fake.last().body.(map[string]any)
		if !ok || fake.last().path != "/api/v1/playback/volume" {
			t.Fatalf("volume call = %+v", fake.last())
		}
		got, ok := body["volume"].(float64)
		if !ok {
			t.Fatalf("volume value = %#v", body["volume"])
		}
		return got
	}
	if got := volume(runeKey('+'), 0.5); got != 0.55 {
		t.Errorf("volume up from 0.5 = %v, want 0.55", got)
	}
	if got := volume(runeKey('+'), 0.98); got != 1 {
		t.Errorf("volume up from 0.98 = %v, want 1 (clamp)", got)
	}
	if got := volume(runeKey('-'), 0.02); got != 0 {
		t.Errorf("volume down from 0.02 = %v, want 0 (clamp)", got)
	}
}

func TestRowKeysMapToItemRoutes(t *testing.T) {
	cases := []struct {
		key   tea.KeyMsg
		route string
	}{
		{runeKey('a'), "/api/v1/playback/play-later"},
		{specialKey(tea.KeyEnter), "/api/v1/playback/play-item"},
	}
	for _, tc := range cases {
		fake := &fakeClient{}
		m := New(fake)
		m.tab = Search
		m.lists[Search].rows = []Item{{ID: "42", Type: "songs", Title: "Song"}}
		_, _ = sendKey(t, m, tc.key)
		last := fake.last()
		body, ok := last.body.(map[string]any)
		if last.method != "POST" || last.path != tc.route || !ok || body["id"] != "42" || body["type"] != "songs" {
			t.Errorf("%q: call = %+v body=%+v, want POST %s {id:42 type:songs}", tc.key.String(), last, body, tc.route)
		}
	}
}

func TestTabsAndQuit(t *testing.T) {
	m := New(&fakeClient{})
	if m.tab != Queue {
		t.Fatal("initial tab")
	}
	m, _ = sendKey(t, m, specialKey(tea.KeyTab))
	if m.tab != Search {
		t.Errorf("tab = %v, want Search", m.tab)
	}
	for i, want := range []Tab{Search, Library, Playlists, Queue} {
		var got Model
		got, _ = sendKey(t, New(&fakeClient{}), runeKey(rune('1'+i)))
		if got.tab != want {
			t.Errorf("key %d: tab = %v, want %v", i+1, got.tab, want)
		}
	}
	_, msg := sendKey(t, New(&fakeClient{}), runeKey('q'))
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("quit message = %#v", msg)
	}
}

func TestSearchUsesStorefrontAndParsesResults(t *testing.T) {
	fake := &fakeClient{amapi: func(path string, out any) error {
		switch path {
		case "/v1/me/storefront":
			return json.Unmarshal([]byte(`{"data":[{"id":"gb"}]}`), out)
		default:
			if !strings.Contains(path, "/v1/catalog/gb/search") {
				t.Errorf("search path = %q, want catalog/gb", path)
			}
			return json.Unmarshal([]byte(`{"results":{"songs":{"data":[{"id":"s1","type":"songs","attributes":{"name":"Track","artistName":"Band"}}]},"albums":{"data":[{"id":"al1","type":"albums","attributes":{"name":"Record"}}]}}}`), out)
		}
	}}
	m := New(fake)
	m.tab = Search
	m.input = true
	m.query = "track"
	// enter runs the search
	_, msg := sendKey(t, m, specialKey(tea.KeyEnter))
	loaded, ok := msg.(loadedMsg)
	if !ok || loaded.err != nil {
		t.Fatalf("search message = %#v", msg)
	}
	rows := items(loaded.page)
	if len(rows) != 2 || rows[0].ID != "s1" || rows[0].Type != "songs" || rows[1].Type != "albums" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestLibraryPagingOnlyNearEnd(t *testing.T) {
	fake := &fakeClient{}
	m := New(fake)
	m.tab = Library
	m.lists[Library] = list{rows: make([]Item, 20), loaded: true, next: "/v1/me/library/songs?offset=20"}
	// Moving while far from the end must not request another page.
	_, _ = sendKey(t, m, runeKey('j'))
	if len(fake.calls) != 0 {
		t.Fatalf("unexpected paging call: %+v", fake.calls)
	}
	// Move to within five rows of the end: a page request is issued.
	m.lists[Library].cursor = 16
	updated, cmd := m.Update(runeKey('j'))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("no load command near the end")
	}
	_ = cmd()
	if len(fake.calls) != 1 || fake.last().method != "AMAPI" || fake.last().path != "/v1/me/library/songs?offset=20" {
		t.Fatalf("paging call = %+v", fake.calls)
	}
}

func TestPlaylistDrillDownLoadsTracks(t *testing.T) {
	fake := &fakeClient{}
	m := New(fake)
	m.tab = Playlists
	m.lists[Playlists] = list{rows: []Item{{ID: "p1", Type: "library-playlists", Title: "Mix"}}, loaded: true}
	next, msg := sendKey(t, m, specialKey(tea.KeyEnter))
	if !next.detail || next.detailKind != Playlists {
		t.Fatalf("detail = %v kind %v", next.detail, next.detailKind)
	}
	if len(fake.calls) != 1 || fake.last().path != "/v1/me/library/playlists/p1/tracks?limit=100" {
		t.Fatalf("tracks call = %+v", fake.calls)
	}
	// The drill-down result replaces nothing until it arrives.
	if _, ok := msg.(loadedMsg); !ok {
		t.Fatalf("message = %#v", msg)
	}
}

func TestQueueRendersCurrentTrack(t *testing.T) {
	fake := &fakeClient{}
	m := New(fake)
	m.tab = Queue
	raw := json.RawMessage(`[{"id":"a","title":"First"},{"id":"b","title":"Second","isCurrent":true}]`)
	rows, current, err := parseQueue(raw)
	if err != nil || len(rows) != 2 || current != "b" {
		t.Fatalf("parseQueue = %+v current=%q err=%v", rows, current, err)
	}
	updated, _ := m.Update(queueMsg{rows: rows, current: current})
	m = updated.(Model)
	view := m.View()
	for _, want := range []string{"First", "Second"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "▸") {
		t.Errorf("current marker missing:\n%s", view)
	}
}

func TestOfflineScreenOnPollError(t *testing.T) {
	fake := &fakeClient{}
	m := New(fake)
	updated, _ := m.Update(pollMsg{err: errors.New("connection refused")})
	m = updated.(Model)
	if !m.offline || !strings.Contains(m.View(), "Cider isn't running") {
		t.Fatalf("offline=%v view=%q", m.offline, m.View())
	}
}

func TestSpectrumRendering(t *testing.T) {
	// A full bar fills every row; an empty one shows only the floor.
	lines := renderSpectrum([]float64{1, 0}, nil, 5, 4)
	if len(lines) != 4 {
		t.Fatalf("rows = %d, want 4", len(lines))
	}
	for i, line := range lines {
		if !strings.HasPrefix(line, "██ ") {
			t.Errorf("row %d = %q, want full bar first", i, line)
		}
	}
	if !strings.HasSuffix(lines[3], "▁▁") || strings.TrimRight(lines[0], " ") != "██" {
		t.Errorf("empty bar: top %q bottom %q", lines[0], lines[3])
	}
	// A half bar on a 2-row display tops out exactly at the lower row; a peak
	// above it is drawn as a cap.
	lines = renderSpectrum([]float64{0.5}, []float64{0.9}, 2, 2)
	if lines[0] != "▀▀" || lines[1] != "██" {
		t.Errorf("half bar with peak = %q", lines)
	}
	if got := fracBlock(0.25, 0, 0.5); got != "▄" {
		t.Errorf("fracBlock half row = %q, want ▄", got)
	}
}

func TestSpectrumTiersByHeight(t *testing.T) {
	fg := func(row float64) any { return spectrumStyle(row).GetForeground() }
	if fg(0) != green || fg(0.45) != yellow || fg(0.8) != red {
		t.Fatal("rows must be green low, yellow middle, red high")
	}
}

func TestMeterEasesAndPeaksHold(t *testing.T) {
	var m meter
	const frame = 1.0 / 60
	for range 10 {
		m.advance([]float64{0.9}, frame)
	}
	if m.levels[0] < 0.85 || m.peaks[0] < m.levels[0] {
		t.Fatalf("rise: level %v peak %v", m.levels[0], m.peaks[0])
	}
	peak, prev := m.peaks[0], m.levels[0]
	for range int(peakHold/frame) - 1 {
		m.advance(nil, frame) // paused or silent: fall to rest
		if m.levels[0] >= prev {
			t.Fatalf("bar did not fall: %v -> %v", prev, m.levels[0])
		}
		prev = m.levels[0]
	}
	if m.peaks[0] != peak {
		t.Fatalf("peak moved during hold: %v", m.peaks[0])
	}
	for range 120 {
		m.advance(nil, frame)
	}
	if m.levels[0] > 0.01 || m.peaks[0] > 0.01 {
		t.Fatalf("did not settle: level %v peak %v", m.levels[0], m.peaks[0])
	}
}

// The frame must never exceed the terminal, whatever the size or mode.
func TestViewFitsTerminal(t *testing.T) {
	frames := make(chan []float64)
	long := strings.Repeat("Very Long Title ", 20)
	for _, size := range [][2]int{{80, 24}, {120, 40}, {40, 10}, {20, 4}} {
		for _, mode := range []string{"viz", "full", "noviz", "offline"} {
			m := New(&fakeClient{})
			if mode != "noviz" {
				m = m.WithViz(frames)
			}
			m.fullViz = mode == "full"
			m.offline = mode == "offline"
			m.width, m.height = size[0], size[1]
			m.now = NowPlaying{Title: long, ArtistName: long, Position: 30, Duration: 120, Volume: 0.82}
			m.meter.advance([]float64{1, 0.5, 0.2, 0.8}, 1)
			m.tab = Library
			for range 50 {
				m.lists[Library].rows = append(m.lists[Library].rows, Item{Title: long, Subtitle: long})
			}
			view := m.View()
			lines := strings.Split(view, "\n")
			if len(lines) > size[1] {
				t.Errorf("%v %s: %d lines", size, mode, len(lines))
			}
			for _, line := range lines {
				if w := lipgloss.Width(line); w > size[0] {
					t.Errorf("%v %s: line width %d: %q", size, mode, w, line)
				}
			}
		}
	}
}

func TestVizFramesDriveVisualizer(t *testing.T) {
	frames := make(chan []float64, 1)
	frames <- []float64{1, 1, 1}
	close(frames)
	m := New(&fakeClient{}).WithViz(frames)
	m.now.Playing = true
	updated, _ := m.Update(waitViz(frames)())
	m = updated.(Model)
	for range 10 {
		updated, _ = m.Update(animMsg(time.Now()))
		m = updated.(Model)
	}
	if !strings.Contains(m.View(), "█") {
		t.Fatalf("view has no bars:\n%s", m.View())
	}
	if vizRows, _ := m.layout(); vizRows < 10 {
		t.Fatalf("visualizer rows = %d at 80x24, want it to dominate", vizRows)
	}
	// A closed analyzer hands its rows to the list.
	updated, _ = m.Update(waitViz(frames)())
	m = updated.(Model)
	if vizRows, listRows := m.layout(); vizRows != 0 || listRows != 24-m.fixedRows()-1 {
		t.Fatalf("after close: viz %d list %d", vizRows, listRows)
	}
}

func TestVTogglesFullScreenViz(t *testing.T) {
	m := New(&fakeClient{}).WithViz(make(chan []float64))
	m, _ = sendKey(t, m, runeKey('v'))
	if vizRows, listRows := m.layout(); listRows != 0 || vizRows != 24-m.fixedRows() {
		t.Fatalf("full screen: viz %d list %d", vizRows, listRows)
	}
}

func TestPollParsesNestedInfo(t *testing.T) {
	fake := &fakeClient{getBodies: map[string]string{
		"/api/v1/playback/now-playing": `{"status":"ok","info":{"name":"A-Punk","artistName":"Vampire Weekend","albumName":"A-Punk - Single of the Week","currentPlaybackTime":30.5,"durationInMillis":137760,"shuffleMode":1,"repeatMode":0,"playParams":{"id":"i.06Qzam2fXKB2KK"}}}`,
		"/api/v1/playback/volume":      `{"status":"ok","volume":0.5}`,
	}}
	msg := poll(fake)()
	pm, ok := msg.(pollMsg)
	if !ok || pm.err != nil {
		t.Fatalf("poll = %#v", msg)
	}
	n := pm.now
	if n.ID != "i.06Qzam2fXKB2KK" || n.Title != "A-Punk" || n.ArtistName != "Vampire Weekend" ||
		n.Position != 30.5 || n.Duration != float64(137760)/1000 || n.Volume != 0.5 ||
		n.ShuffleMode != "on" || n.RepeatMode != "off" {
		t.Fatalf("now = %+v", n)
	}
}

func TestPollUsesCatalogSongIDForLyrics(t *testing.T) {
	for id, want := range map[string]string{"1337791444": "1337791444", "i.06Qzam2fXKB2KK": ""} {
		fake := &fakeClient{getBodies: map[string]string{
			"/api/v1/playback/now-playing": `{"status":"ok","info":{"name":"x","hasLyrics":true,"playParams":{"id":"` + id + `"}}}`,
			"/api/v1/playback/volume":      `{"status":"ok","volume":0.5}`,
		}}
		pm, ok := poll(fake)().(pollMsg)
		if !ok || pm.err != nil || pm.now.CatalogID != want {
			t.Fatalf("id %s: catalog ID = %q, want %q", id, pm.now.CatalogID, want)
		}
	}
}

func TestSearchInputAcceptsSpaces(t *testing.T) {
	m := New(&fakeClient{})
	m.tab, m.input = Search, true
	for _, k := range []tea.KeyMsg{runeKey('a'), specialKey(tea.KeySpace), runeKey('b')} {
		m, _ = sendKey(t, m, k)
	}
	if m.query != "a b" {
		t.Fatalf("query = %q, want %q", m.query, "a b")
	}
}

func TestCtrlCQuitsDuringSearchInput(t *testing.T) {
	m := New(&fakeClient{})
	m.tab, m.input = Search, true
	_, msg := sendKey(t, m, specialKey(tea.KeyCtrlC))
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c during input = %#v, want quit", msg)
	}
}

// A page requested for a list that has since been replaced (songs/albums
// toggle, another playlist opened, a new search) must not be appended to it.
func TestStaleLoadIsDropped(t *testing.T) {
	fake := &fakeClient{}
	m := New(fake)
	m.tab = Library
	stale := m.ensureLoaded()().(loadedMsg) // songs load in flight
	stale.page = page{Data: []appleItem{{ID: "song"}}}
	m, _ = sendKey(t, m, runeKey('l')) // switch to albums before it arrives
	updated, _ := m.Update(stale)
	m = updated.(Model)
	if rows := m.lists[Library].rows; len(rows) != 0 {
		t.Fatalf("stale songs appended to albums list: %+v", rows)
	}
}

func TestPollsDoNotOverlap(t *testing.T) {
	m := New(&fakeClient{})
	_, cmd := m.Update(tickMsg{})
	if cmd == nil {
		t.Fatal("tick did not poll")
	}
	if _, ok := cmd().(pollMsg); !ok {
		t.Fatal("tick must issue only a poll; the next tick follows its result")
	}
	_, cmd = m.Update(pollMsg{})
	if cmd == nil {
		t.Fatal("poll result did not schedule the next tick")
	}
}

func TestFailedVolumeReadKeepsLastVolume(t *testing.T) {
	m := New(&fakeClient{})
	m.now.Volume = 0.8
	updated, _ := m.Update(pollMsg{now: NowPlaying{Title: "x"}})
	m = updated.(Model)
	if m.now.Volume != 0.8 {
		t.Fatalf("volume = %v after failed read, want 0.8", m.now.Volume)
	}
}

func TestQueueReloadClampsCursor(t *testing.T) {
	m := New(&fakeClient{})
	m.lists[Queue].cursor = 5
	updated, _ := m.Update(queueMsg{rows: []Item{{ID: "a"}}})
	m = updated.(Model)
	if c := m.lists[Queue].cursor; c != 0 {
		t.Fatalf("cursor = %d, want 0", c)
	}
}

func TestPausedVisualizerSettles(t *testing.T) {
	m := New(&fakeClient{}).WithViz(make(chan []float64))
	m.target = []float64{1, 1}
	m.now.Playing = false
	for range 120 {
		updated, _ := m.Update(animMsg(time.Now()))
		m = updated.(Model)
	}
	for _, v := range m.meter.levels {
		if v > 0.01 {
			t.Fatalf("paused bars still up: %v", m.meter.levels)
		}
	}
}

func TestPollReadsPlayState(t *testing.T) {
	fake := &fakeClient{getBodies: map[string]string{"/api/v1/playback/is-playing": `{"status":"ok","is_playing":false}`}}
	if pm := poll(fake)().(pollMsg); pm.now.Playing {
		t.Fatal("is_playing false read as playing")
	}
	// Unknown state counts as playing.
	if pm := poll(&fakeClient{})().(pollMsg); !pm.now.Playing {
		t.Fatal("missing play state froze the visualizer")
	}
}

// Volume keys act immediately: quick presses accumulate instead of all
// starting from the last polled value, and a poll already in flight does not
// snap the meter back.
func TestVolumeKeysAccumulate(t *testing.T) {
	fake := &fakeClient{}
	m := New(fake)
	m.now.Volume = 0.5
	for range 3 {
		m, _ = sendKey(t, m, runeKey('+'))
	}
	if m.now.Volume != 0.65 {
		t.Fatalf("local volume = %v after 3 presses, want 0.65", m.now.Volume)
	}
	if got := fake.last().body.(map[string]any)["volume"]; got != 0.65 {
		t.Fatalf("last volume sent = %v, want 0.65", got)
	}
	updated, _ := m.Update(pollMsg{now: NowPlaying{Volume: 0.5}, volOK: true})
	if v := updated.(Model).now.Volume; v != 0.65 {
		t.Fatalf("stale poll reset volume to %v", v)
	}
}

func typeText(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, r := range text {
		k := runeKey(r)
		if r == ' ' {
			k = specialKey(tea.KeySpace)
		}
		m, _ = sendKey(t, m, k)
	}
	return m
}

func TestLibrarySearchAndRestore(t *testing.T) {
	fake := &fakeClient{amapi: func(path string, out any) error {
		if !strings.HasPrefix(path, "/v1/me/library/search?types=library-songs&limit=25&term=vampire") {
			t.Errorf("library search path = %q", path)
		}
		return json.Unmarshal([]byte(`{"results":{"library-songs":{"data":[{"id":"i.1","type":"library-songs","attributes":{"name":"A-Punk","artistName":"Vampire Weekend"}}],"next":"/v1/me/library/search?offset=25"}}}`), out)
	}}
	m := New(fake)
	m.tab = Library
	m.lists[Library] = list{rows: []Item{{Title: "Full 1"}, {Title: "Full 2"}}, loaded: true, cursor: 1}
	m, _ = sendKey(t, m, runeKey('/'))
	m = typeText(t, m, "vampire")
	m, msg := sendKey(t, m, specialKey(tea.KeyEnter))
	updated, _ := m.Update(msg)
	m = updated.(Model)
	if l := m.lists[Library]; len(l.rows) != 1 || l.rows[0].Title != "A-Punk" || l.next == "" {
		t.Fatalf("results = %+v next %q", l.rows, l.next)
	}
	if !strings.Contains(m.View(), "search vampire") {
		t.Errorf("view does not show the search:\n%s", m.View())
	}
	m, _ = sendKey(t, m, specialKey(tea.KeyEsc))
	if l := m.lists[Library]; len(l.rows) != 2 || l.cursor != 1 {
		t.Fatalf("esc did not restore the full list: %+v", l)
	}
}

func TestPlaylistFilterIsLiveAndSelectsFilteredRow(t *testing.T) {
	fake := &fakeClient{}
	m := New(fake)
	m.tab = Playlists
	m.lists[Playlists] = list{loaded: true, rows: []Item{
		{ID: "p1", Title: "Favorite Songs"}, {ID: "p2", Title: "Heaps Indie"}, {ID: "p3", Title: "Gym"},
	}}
	m, _ = sendKey(t, m, runeKey('/'))
	m = typeText(t, m, "heaps")
	if got := m.lists[Playlists].shown(); len(got) != 1 || got[0].ID != "p2" {
		t.Fatalf("filtered = %+v", got)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("filtering called Cider: %+v", fake.calls)
	}
	m, _ = sendKey(t, m, specialKey(tea.KeyEnter)) // done typing
	m, _ = sendKey(t, m, specialKey(tea.KeyEnter)) // open the match
	if !m.detail || m.detailID != "p2" {
		t.Fatalf("opened %q, want p2", m.detailID)
	}
	m, _ = sendKey(t, m, specialKey(tea.KeyEsc)) // back out of the playlist
	m, _ = sendKey(t, m, specialKey(tea.KeyEsc)) // clear the filter
	if f := m.lists[Playlists].filter; f != "" || len(m.lists[Playlists].shown()) != 3 {
		t.Fatalf("filter %q not cleared", f)
	}
}

func TestSlashOnSearchTabStillSearchesCatalog(t *testing.T) {
	m := New(&fakeClient{})
	m.tab = Search
	m, _ = sendKey(t, m, runeKey('/'))
	if !m.input || m.scope() != catalogScope {
		t.Fatalf("input %v scope %v", m.input, m.scope())
	}
}

func TestQueueKeysConfirmAndReloadQueue(t *testing.T) {
	fake := &fakeClient{}
	m := New(fake)
	m.tab = Library
	m.lists[Library] = list{loaded: true, rows: []Item{{ID: "i.1", Type: "library-songs", Title: "A-Punk"}}}
	for key, want := range map[rune]string{'a': "Queued: A-Punk"} {
		next, msg := sendKey(t, m, runeKey(key))
		updated, cmd := next.Update(msg)
		got := updated.(Model)
		if !strings.Contains(got.View(), want) {
			t.Errorf("%c: footer missing %q:\n%s", key, want, got.View())
		}
		if cmd == nil {
			t.Fatalf("%c: no queue reload", key)
		}
		if _, ok := cmd().(queueMsg); !ok {
			t.Errorf("%c: follow-up is not a queue reload", key)
		}
	}
}

func TestClearQueueKey(t *testing.T) {
	fake := &fakeClient{}
	m := New(fake)
	m.tab = Queue
	_, msg := sendKey(t, m, runeKey('c'))
	if len(fake.calls) != 1 || fake.last().path != "/api/v1/playback/queue/clear-queue" {
		t.Fatalf("clear call = %+v", fake.calls)
	}
	if fmt.Sprint(fake.last().body) != "map[]" {
		t.Fatalf("clear body = %#v, want empty JSON object", fake.last().body)
	}
	a, ok := msg.(actionMsg)
	if !ok || a.err != nil || !a.queued || a.notice != "Queue cleared" {
		t.Fatalf("clear msg = %#v", msg)
	}
	// The key is Queue-tab only.
	other := &fakeClient{}
	otherModel := New(other)
	otherModel.tab = Search
	if _, _ = sendKey(t, otherModel, runeKey('c')); len(other.calls) != 0 {
		t.Fatal("c acted off the Queue tab")
	}
}

func TestSkipRefreshesWithoutSecondPollLoop(t *testing.T) {
	fake := &fakeClient{}
	m := New(fake)
	m.tab = Search
	m, msg := sendKey(t, m, runeKey('n'))
	if fake.last().path != "/api/v1/playback/next" {
		t.Fatalf("n posted %q", fake.last().path)
	}
	updated, cmd := m.Update(msg)
	refreshed, ok := cmd().(pollMsg)
	if !ok || !refreshed.extra {
		t.Fatalf("skip follow-up = %#v, want one-off poll", refreshed)
	}
	if _, next := updated.Update(refreshed); next != nil {
		t.Fatal("one-off poll scheduled another tick: polling would double")
	}
}

func TestSortCyclesServerOrder(t *testing.T) {
	fake := &fakeClient{}
	m := New(fake)
	m.tab = Library
	m.lists[Library] = list{loaded: true, rows: []Item{{Title: "x"}}}
	want := []struct{ path, label string }{
		{"/v1/me/library/songs?limit=100&sort=-name", "Z→A"},
		{"/v1/me/library/songs?limit=100&sort=-dateAdded", "newest"},
		{"/v1/me/library/songs?limit=100&sort=dateAdded", "oldest"},
		{"/v1/me/library/songs?limit=100", "A→Z"},
	}
	for _, w := range want {
		var msg tea.Msg
		m, msg = sendKey(t, m, runeKey('o'))
		if fake.last().path != w.path {
			t.Fatalf("o requested %q, want %q", fake.last().path, w.path)
		}
		updated, _ := m.Update(msg)
		m = updated.(Model)
		if !strings.Contains(m.View(), "o "+w.label) {
			t.Fatalf("view missing order %q", w.label)
		}
	}
	// Albums keep the library's order; playlists have their own.
	m.order[Library] = newest
	m, _ = sendKey(t, m, runeKey('l'))
	if fake.last().path != "/v1/me/library/albums?limit=100&sort=-dateAdded" {
		t.Fatalf("albums path %q", fake.last().path)
	}
	m.tab = Playlists
	m, _ = sendKey(t, m, runeKey('o'))
	if fake.last().path != "/v1/me/library/playlists?limit=100&sort=-name" {
		t.Fatalf("playlists path %q", fake.last().path)
	}
	// Inside a playlist the playlist's own order stands.
	m.detail = true
	calls := len(fake.calls)
	m, _ = sendKey(t, m, runeKey('o'))
	if len(fake.calls) != calls {
		t.Fatal("o inside a playlist requested a reload")
	}
}

func TestQueueMarksNowPlayingWithoutCurrentFlag(t *testing.T) {
	m := New(&fakeClient{})
	m.tab = Queue
	m.now = NowPlaying{ID: "i.2", Title: "Second", Playing: true}
	rows, _, err := parseQueue(json.RawMessage(`[{"id":"i.1","attributes":{"name":"First"}},{"id":"i.2","attributes":{"name":"Second"}}]`))
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(queueMsg{rows: rows})
	for _, line := range strings.Split(updated.(Model).View(), "\n") {
		if strings.Contains(line, "First") && strings.Contains(line, "▸") {
			t.Fatalf("wrong row marked: %q", line)
		}
		if strings.Contains(line, "Second") && !strings.Contains(line, "▸") {
			t.Fatalf("playing row not marked: %q", line)
		}
	}
}

func TestHelpAndPlayAll(t *testing.T) {
	fake := &fakeClient{}
	m := New(fake)
	m, _ = sendKey(t, m, runeKey('?'))
	if !strings.Contains(m.View(), "toggle shuffle") {
		t.Fatalf("help not shown:\n%s", m.View())
	}
	m, _ = sendKey(t, m, specialKey(tea.KeyEsc))
	if m.help {
		t.Fatal("esc did not close help")
	}
	// Inside a playlist, P queues its loaded tracks (Cider can't play a
	// playlist by ID); with shuffle on they are shuffled first.
	m.tab, m.detail, m.detailKind, m.detailTitle = Playlists, true, Playlists, "Heaps Indie"
	m.detailList = list{loaded: true}
	for i := range 20 {
		m.detailList.rows = append(m.detailList.rows, Item{ID: fmt.Sprint(i), Type: "library-songs"})
	}
	m.now.ShuffleMode = "on"
	m, msg := sendKey(t, m, runeKey('P'))
	if a, ok := msg.(actionMsg); !ok || a.notice != "Playing: Heaps Indie (shuffled) · 19 more queued" {
		t.Fatalf("P message = %#v", msg)
	}
	var order []string
	for _, c := range fake.calls {
		if strings.HasSuffix(c.path, "play-item") || strings.HasSuffix(c.path, "play-later") {
			order = append(order, c.body.(map[string]any)["id"].(string))
		}
	}
	if len(order) != 20 || !strings.HasSuffix(fake.calls[len(fake.calls)-20].path, "play-item") {
		t.Fatalf("queued %d tracks: %v", len(order), fake.calls)
	}
	if strings.Join(order, ",") == "0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19" {
		t.Fatal("shuffle on but tracks queued in order")
	}
	// On a song row, P does nothing.
	m.detail, m.tab = false, Library
	m.lists[Library] = list{loaded: true, rows: []Item{{ID: "i.1", Type: "library-songs"}}}
	calls := len(fake.calls)
	if m, _ = sendKey(t, m, runeKey('P')); len(fake.calls) != calls {
		t.Fatal("P on a song played it")
	}
}

// Enter on a song plays it and queues the rest of the list, because Cider's
// play-item replaces the queue with that single song.
func TestEnterPlaysFromHereAndQueuesRest(t *testing.T) {
	fake := &fakeClient{}
	m := New(fake)
	m.tab = Library
	m.lists[Library] = list{loaded: true, cursor: 1, rows: []Item{
		{ID: "a", Type: "library-songs"}, {ID: "b", Type: "library-songs", Title: "B"}, {ID: "c", Type: "library-songs"}, {ID: "d", Type: "library-songs"},
	}}
	_, msg := sendKey(t, m, specialKey(tea.KeyEnter))
	var got []string
	for _, c := range fake.calls {
		got = append(got, c.path[len("/api/v1/playback/"):]+":"+c.body.(map[string]any)["id"].(string))
	}
	if strings.Join(got, " ") != "play-item:b play-later:c play-later:d" {
		t.Fatalf("calls = %v", got)
	}
	if a := msg.(actionMsg); a.notice != "Playing: B · 2 more queued" {
		t.Fatalf("notice = %q", a.notice)
	}
}

func TestShiftTabGoesLeft(t *testing.T) {
	m := New(&fakeClient{})
	m.tab = Search
	m, _ = sendKey(t, m, specialKey(tea.KeyShiftTab))
	if m.tab != Queue {
		t.Fatalf("shift+tab from Search = %v, want Queue (wraps left)", m.tab)
	}
	m, _ = sendKey(t, m, specialKey(tea.KeyShiftTab))
	if m.tab != Playlists {
		t.Fatalf("second shift+tab = %v, want Playlists", m.tab)
	}
}

// queueFake serves a queue of n songs and records queue route calls.
func queueFake(n int) *fakeClient {
	var entries []string
	for i := range n {
		entries = append(entries, fmt.Sprintf(`{"id":"i.%d","type":"song","attributes":{"name":"S%d"}}`, i, i))
	}
	return &fakeClient{getBodies: map[string]string{"/api/v1/playback/queue": "[" + strings.Join(entries, ",") + "]"}}
}

func TestQueueTabKeysUseQueueRoutes(t *testing.T) {
	fake := queueFake(5)
	m := New(fake)
	m.tab = Queue
	m.now.ID = "i.1" // playing the second entry
	var raw json.RawMessage
	_ = fake.Get("/api/v1/playback/queue", &raw)
	rows, _, _ := parseQueue(raw)
	m.lists[Queue] = list{loaded: true, rows: rows, cursor: 4}
	cases := []struct {
		key  tea.KeyMsg
		path string
		body map[string]any
	}{
		{specialKey(tea.KeyEnter), "change-to-index", map[string]any{"index": 4}},
		{runeKey('x'), "remove-by-index", map[string]any{"index": 4}},
	}
	for _, c := range cases {
		_, _ = sendKey(t, m, c.key)
		got := fake.last()
		if got.path != "/api/v1/playback/queue/"+c.path || fmt.Sprint(got.body) != fmt.Sprint(c.body) {
			t.Errorf("%s: %s %v, want %s %v", c.key, got.path, got.body, c.path, c.body)
		}
	}
	// The playing song can't be removed; a filtered list has no safe index.
	m.lists[Queue].cursor = 1
	calls := len(fake.calls)
	if _, _ = sendKey(t, m, runeKey('x')); len(fake.calls) != calls {
		t.Error("x removed the playing song")
	}
	m.lists[Queue].cursor, m.lists[Queue].filter = 4, "S"
	if _, _ = sendKey(t, m, runeKey('x')); len(fake.calls) != calls {
		t.Error("x acted on a filtered queue")
	}
}

func TestShuffleTogglesCiderMode(t *testing.T) {
	fake := queueFake(30)
	m := New(fake)
	m.now.ID = "i.3"
	_, msg := sendKey(t, m, runeKey('s'))
	if a := msg.(actionMsg); a.err != nil || !a.skipped {
		t.Fatalf("shuffle = %#v", a)
	}
	for _, c := range fake.calls {
		if c.method != "POST" {
			continue
		}
		if c.path != "/api/v1/playback/toggle-shuffle" {
			t.Fatalf("shuffle called %s", c.path)
		}
	}
}

func TestQueueHasNoEnqueueOrMoveBinding(t *testing.T) {
	fake := &fakeClient{}
	m := New(fake)
	m.tab = Queue
	m.now.ID = "current"
	m.lists[Queue] = list{loaded: true, cursor: 1, rows: []Item{{ID: "current"}, {ID: "next"}}}
	for _, key := range []rune{'a', 'A'} {
		if _, msg := sendKey(t, m, runeKey(key)); msg != nil {
			t.Fatalf("%c acted on Queue: %#v", key, msg)
		}
	}
	if len(fake.calls) != 0 {
		t.Fatalf("unexpected queue mutations: %+v", fake.calls)
	}
	footer := strings.Join(m.footerLines(80), " ")
	for _, hint := range []string{"jump", "remove", "clear", "keys"} {
		if !strings.Contains(footer, hint) {
			t.Errorf("Queue footer missing %q", hint)
		}
	}
}

func TestFooterShowsEssentialKeysWrapped(t *testing.T) {
	m := New(&fakeClient{})
	m.tab = Search
	m.width = 80
	lines := m.footerLines(80)
	all := strings.Join(lines, " ")
	for _, k := range []string{"search", "enqueue", "play/pause", "skip", "keys", "quit"} {
		if !strings.Contains(all, k) {
			t.Errorf("footer missing %q: %q", k, all)
		}
	}
	for _, k := range []string{"shuffle", "repeat", "seek", "vol", "viz", "play all"} {
		if strings.Contains(all, k) {
			t.Errorf("secondary control %q still in footer", k)
		}
	}
	for _, l := range lines {
		if lipgloss.Width(l) > 80 {
			t.Errorf("footer line too wide: %q", l)
		}
	}
}

// Cider's queue includes played songs and can list one twice; with the
// playing song duplicated, queue edits must refuse rather than guess.
func TestQueueEditsRefuseAmbiguousPosition(t *testing.T) {
	fake := &fakeClient{getBodies: map[string]string{"/api/v1/playback/queue": `[{"id":"i.a"},{"id":"i.b"},{"id":"i.a"},{"id":"i.c"}]`}}
	m := New(fake)
	m.now.ID = "i.a"
	m.tab = Queue
	m.lists[Queue] = list{loaded: true, cursor: 3, rows: []Item{{ID: "i.a"}, {ID: "i.b"}, {ID: "i.a"}, {ID: "i.c"}}}
	_, msg := sendKey(t, m, runeKey('x'))
	if a, ok := msg.(actionMsg); !ok || a.err != errQueuePosition {
		t.Fatalf("remove with duplicate current = %#v", msg)
	}
	for _, c := range fake.calls {
		if c.method == "POST" {
			t.Fatalf("edited the queue anyway: %+v", c)
		}
	}
	// Played entries (before the current one) are never moved or removed.
	m.now.ID = "i.c"
	m.lists[Queue].cursor = 1
	if _, msg = sendKey(t, m, runeKey('x')); msg != nil {
		t.Fatalf("x on a played entry = %#v", msg)
	}
}

const sampleTTML = `<tt xmlns="http://www.w3.org/ns/ttml" itunes:timing="Word"><body>
<div><p begin="20.675" end="21.970">I've been everywhere</p>
<p begin="1:02.5" end="1:04"><span begin="1:02.5">Through</span> <span begin="1:03.1">every</span> <span begin="1:03.6">door</span></p></div>
</body></tt>`

func TestParseTTMLAndCurrentLine(t *testing.T) {
	lines, err := parseTTML(sampleTTML)
	if err != nil || len(lines) != 2 {
		t.Fatalf("lines = %+v err %v", lines, err)
	}
	if lines[0].start != 20.675 || lines[0].text != "I've been everywhere" || lines[1].start != 62.5 || len(lines[1].words) != 3 {
		t.Fatalf("parsed = %+v", lines)
	}
	for pos, want := range map[float64]int{10: -1, 21: 0, 62.4: 0, 63: 1} {
		if got := currentLine(lines, pos); got != want {
			t.Errorf("currentLine(%v) = %d, want %d", pos, got, want)
		}
	}
	if ttmlTime("1:02:03.5") != 3723.5 || ttmlTime("4.2s") != 4.2 || ttmlTime("x") != 0 {
		t.Error("ttmlTime formats")
	}
}

func TestLyricsFetchAutomaticallyAndFollowSong(t *testing.T) {
	fake := &fakeClient{amapi: func(path string, out any) error {
		switch {
		case path == "/v1/me/storefront":
			return json.Unmarshal([]byte(`{"data":[{"id":"us"}]}`), out)
		case path == "/v1/catalog/us/songs/123/lyrics":
			b, _ := json.Marshal(map[string]any{"data": []any{map[string]any{"attributes": map[string]any{"ttml": sampleTTML}}}})
			return json.Unmarshal(b, out)
		}
		t.Errorf("unexpected AMAPI %q", path)
		return nil
	}}
	m := New(fake)
	m.now = NowPlaying{ID: "i.1", CatalogID: "123", HasLyrics: true, Position: 21, Duration: 200}
	m.polledAt = time.Now()
	cmd := m.ensureLyrics()
	if cmd == nil || !strings.Contains(m.View(), "Loading lyrics") {
		t.Fatalf("not loading:\n%s", m.View())
	}
	updated, _ := m.Update(cmd())
	m = updated.(Model)
	view := m.View()
	if !strings.Contains(view, "▸ I've been everywhere") || !strings.Contains(view, "Through every door") {
		t.Fatalf("lyrics view:\n%s", view)
	}
	// A song without lyrics says so instead of fetching.
	calls := len(fake.calls)
	updated, _ = m.Update(pollMsg{now: NowPlaying{ID: "i.2", Title: "x"}, volOK: true})
	if m = updated.(Model); !strings.Contains(m.View(), "No lyrics") || len(fake.calls) != calls {
		t.Fatalf("no-lyrics song:\n%s", m.View())
	}
}

func TestPlaybackModesReflectCiderPoll(t *testing.T) {
	m := New(&fakeClient{})
	for _, modes := range []struct{ shuffle, repeat string }{
		{"off", "off"}, {"on", "all"}, {"on", "one"}, {"off", "off"},
	} {
		updated, _ := m.Update(pollMsg{now: NowPlaying{ShuffleMode: modes.shuffle, RepeatMode: modes.repeat}})
		m = updated.(Model)
		shuffle, repeat := faintStyle.Render("Shuffle"), faintStyle.Render("Repeat")
		if modes.shuffle == "on" {
			shuffle = accentStyle.Render("Shuffle")
		}
		switch modes.repeat {
		case "one":
			repeat = accentStyle.Render("Repeat 1")
		case "all":
			repeat = accentStyle.Render("Repeat")
		}
		line := m.seekLine(80)
		if !strings.HasSuffix(line, shuffle+" "+repeat+" ") {
			t.Fatalf("playback modes not shown beside time: %s", line)
		}
		if strings.Contains(m.headerLine(80), "Shuffle") || lipgloss.Width(line) != 80 {
			t.Fatalf("incorrect label placement or seek width: %s", line)
		}
	}
}
