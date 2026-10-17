// Package ui implements the terminal remote control. It deliberately depends on
// the small HTTP surface, not on cider's concrete client or its response types.
package ui

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ajpeders/cider-tui/viz"
	tea "github.com/charmbracelet/bubbletea"
)

// Client is implemented by cider.Client's Get, Post and AMAPI methods.
type Client interface {
	Get(path string, out any) error
	Post(path string, body any, out any) error
	AMAPI(path string, out any) error
}

type Tab int

const (
	Search Tab = iota
	Library
	Playlists
	Queue
)

var tabNames = [...]string{"Search", "Library", "Playlists", "Queue"}

type Item struct {
	ID, Type, Title, Subtitle, Album string
}

type page struct {
	Data []appleItem `json:"data"`
	Next string      `json:"next"`
}

type appleItem struct {
	ID, Type   string
	Attributes struct {
		Name, ArtistName, AlbumName string
	} `json:"attributes"`
}

func items(p page) []Item {
	out := make([]Item, 0, len(p.Data))
	for _, v := range p.Data {
		title := v.Attributes.Name
		if title == "" {
			title = v.ID
		}
		subtitle := v.Attributes.ArtistName
		if subtitle == "" {
			subtitle = v.Attributes.AlbumName
		}
		out = append(out, Item{ID: v.ID, Type: v.Type, Title: title, Subtitle: subtitle, Album: v.Attributes.AlbumName})
	}
	return out
}

type list struct {
	rows            []Item
	cursor          int
	next            string
	loaded, loading bool
	gen             int    // identifies this list instance to its in-flight loads
	filter          string // client-side filter; the cursor indexes shown()
}

// shown returns the rows matching the filter (title, subtitle/artist or album,
// any case).
func (l *list) shown() []Item {
	if l.filter == "" {
		return l.rows
	}
	f := strings.ToLower(l.filter)
	var out []Item
	for _, r := range l.rows {
		if strings.Contains(strings.ToLower(r.Title+" "+r.Subtitle+" "+r.Album), f) {
			out = append(out, r)
		}
	}
	return out
}

// order is a server-side sort Apple supports for library lists. Sorting by
// artist, plays or release date is rejected by the API.
type order int

const (
	byName order = iota // Apple's default
	byNameDesc
	newest
	oldest
)

var (
	orderParam = [...]string{"", "&sort=-name", "&sort=-dateAdded", "&sort=dateAdded"}
	orderLabel = [...]string{"A→Z", "Z→A", "newest", "oldest"}
)

// scope is what / searches on the current view.
type scope int

const (
	catalogScope scope = iota // Search tab: the Apple Music catalog
	libraryScope              // Library tab: Apple's library search
	filterScope               // playlists, queue, inside a playlist/album: filter what's loaded
)

func (m Model) scope() scope {
	switch {
	case m.detail || m.tab == Playlists || m.tab == Queue:
		return filterScope
	case m.tab == Library:
		return libraryScope
	}
	return catalogScope
}

type NowPlaying struct {
	ID, Title, ArtistName, AlbumName string
	Position, Duration               float64
	Volume                           float64
	ShuffleMode, RepeatMode          string
	Playing                          bool
	CatalogID                        string // Apple catalog ID; lyrics need it
	HasLyrics                        bool
}

type Model struct {
	client         Client
	tab            Tab
	lists          [4]list
	query          string
	input          bool
	searchTerm     string
	libraryTerm    string
	libraryFull    *list // the library list hidden while search results show
	order          [4]order
	libraryAlbums  bool
	detail         bool
	detailTitle    string
	detailID       string
	detailKind     Tab
	detailList     list
	now            NowPlaying
	queueCurrent   string
	status         string
	offline        bool
	viz            <-chan []float64
	vizFrames      <-chan viz.Frame
	vizClosed      bool // the analyzer stopped: give its rows to the list
	fullViz        bool // v: hide the list, visualizer fills the screen
	waveform       bool // b: switch between spectrum and waveform
	samples        []float64
	waveTarget     []float64 // latest audio hop; displayed at a slower waveform rate
	lastWave       time.Time
	help           bool // ?: the key list replaces the list
	expanded       bool // d: details beneath the selected row
	lyricsID       string
	lyrics         []lyricLine
	lyricsErr      error
	lyricsLoading  bool
	lyricsExpanded bool // y: lyrics fill the bottom panel instead of the menu
	settings       bool
	theme          string
	backgroundOn   bool // false leaves the terminal's background untouched
	themeCursor    int
	settingsPath   string
	polledAt       time.Time // when now.Position was read; for live position
	meter          meter
	volumeHeld     time.Time // until then, polls keep the locally set volume
	queueLoaded    time.Time // when the Queue tab last refreshed
	queueLoading   bool      // only one queue read may be in flight
	queueDirty     bool      // a mutation happened during that read
	queueRetryAt   time.Time // back off after a failed read
	refreshLoading bool      // coalesce one-off playback refreshes
	notice         string    // brief confirmation shown in the footer
	noticeUntil    time.Time
	target         []float64 // latest analyzer frame the meter eases towards
	lastFrame      time.Time
	width, height  int
	lastTrack      string
	gen            int
}

func New(client Client) Model {
	return Model{client: client, tab: Queue, width: 80, height: 24, theme: "Terminal", backgroundOn: true}
}

// WithSettings loads the saved theme for the interactive application. Tests
// and embedded clients can use New without reading the user's config.
func (m Model) WithSettings() Model {
	path, err := themeSettingsPath()
	if err == nil {
		m.settingsPath = path
		var config appearanceSettings
		config, err = loadAppearance(path)
		m.theme, m.backgroundOn, m.waveform = config.Theme, config.Background, config.Visualizer == "waveform"
	}
	if err == nil {
		err = applyTheme(m.theme)
	}
	if err != nil {
		m.theme = "Terminal"
		m.backgroundOn = true
		m.waveform = false
		_ = applyTheme("Terminal")
		m.status = "Could not load theme settings"
	}
	return m
}

// fresh returns an empty list with a new generation, so pages still in flight
// for the list it replaces are dropped on arrival.
func (m *Model) fresh() list {
	m.gen++
	return list{gen: m.gen}
}

// WithViz draws levels from a viz.Analyzer stream as the main visualizer. A
// nil or closed channel hides it and the list takes its rows.
func (m Model) WithViz(ch <-chan []float64) Model {
	m.viz = ch
	return m
}

// WithFrames enables both spectrum and waveform from one analyzer stream.
func (m Model) WithFrames(ch <-chan viz.Frame) Model {
	m.vizFrames = ch
	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{poll(m.client), func() tea.Msg { return queueRefreshMsg{} }, lyricsFrame()}
	if m.viz != nil {
		cmds = append(cmds, waitViz(m.viz), animate())
	} else if m.vizFrames != nil {
		cmds = append(cmds, waitFrames(m.vizFrames), animate())
	}
	return tea.Batch(cmds...)
}

type pollMsg struct {
	extra bool      // a one-off refresh: must not start a second tick chain
	at    time.Time // when now-playing was sampled, before the other reads
	now   NowPlaying
	volOK bool // false when the volume read failed; keep the last known value
	err   error
}
type tickMsg struct{}
type loadedMsg struct {
	tab    Tab
	detail bool
	gen    int
	page   page
	err    error
}
type queueMsg struct {
	rows    []Item
	current string
	err     error
}

type queueRefreshMsg struct{}
type queueRetryMsg struct{ at time.Time }

// actionMsg reports a playback command. notice is shown briefly on success;
// skipped asks for an immediate now-playing refresh; queued reloads the queue.
type actionMsg struct {
	err             error
	notice          string
	skipped, queued bool
}
type vizMsg []float64
type frameMsg viz.Frame
type vizClosedMsg struct{}
type animMsg time.Time

// animInterval is the visualizer's redraw rate (~60 fps), independent of how
// often analyzer frames arrive.
const animInterval = 16 * time.Millisecond

// Audio arrives about 60 times per second; displaying every hop makes the
// waveform too frantic. Keep the newest hop and show it about 20 times/second.
const waveformInterval = 48 * time.Millisecond

func animate() tea.Cmd {
	return tea.Tick(animInterval, func(t time.Time) tea.Msg { return animMsg(t) })
}

func tick(d time.Duration) tea.Cmd { return tea.Tick(d, func(time.Time) tea.Msg { return tickMsg{} }) }

// waitViz blocks until the analyzer publishes a frame; the UI re-issues it
// after every frame so the strip keeps updating.
func waitViz(ch <-chan []float64) tea.Cmd {
	return func() tea.Msg {
		bars, ok := <-ch
		if !ok {
			return vizClosedMsg{}
		}
		return vizMsg(bars)
	}
}

func waitFrames(ch <-chan viz.Frame) tea.Cmd {
	return func() tea.Msg {
		frame, ok := <-ch
		if !ok {
			return vizClosedMsg{}
		}
		return frameMsg(frame)
	}
}

func poll(c Client) tea.Cmd {
	return func() tea.Msg {
		startedAt := time.Now()
		// Cider nests the whole now-playing state under "info".
		var raw struct {
			Info struct {
				Name                string  `json:"name"`
				ArtistName          string  `json:"artistName"`
				AlbumName           string  `json:"albumName"`
				CurrentPlaybackTime float64 `json:"currentPlaybackTime"`
				DurationInMillis    int64   `json:"durationInMillis"`
				ShuffleMode         int     `json:"shuffleMode"`
				RepeatMode          int     `json:"repeatMode"`
				HasLyrics           bool    `json:"hasLyrics"`
				PlayParams          struct {
					ID        string `json:"id"`
					CatalogID string `json:"catalogId"`
				} `json:"playParams"`
			} `json:"info"`
		}
		err := c.Get("/api/v1/playback/now-playing", &raw)
		if err != nil {
			return pollMsg{err: err, at: startedAt}
		}
		sampledAt := time.Now()
		n := NowPlaying{
			ID:          raw.Info.PlayParams.ID,
			CatalogID:   raw.Info.PlayParams.CatalogID,
			HasLyrics:   raw.Info.HasLyrics,
			Title:       raw.Info.Name,
			ArtistName:  raw.Info.ArtistName,
			AlbumName:   raw.Info.AlbumName,
			Position:    raw.Info.CurrentPlaybackTime,
			Duration:    float64(raw.Info.DurationInMillis) / 1000,
			ShuffleMode: shuffleLabel(raw.Info.ShuffleMode),
			RepeatMode:  repeatLabel(raw.Info.RepeatMode),
		}
		// Catalog (non-library) songs come with a blank catalogId; their
		// all-digit ID is the catalog ID. Library IDs look like "i.…".
		if n.CatalogID == "" && isCatalogID(n.ID) {
			n.CatalogID = n.ID
		}
		var vol struct {
			Volume float64 `json:"volume"`
		}
		e := c.Get("/api/v1/playback/volume", &vol)
		n.Volume = vol.Volume
		// Unknown play state counts as playing, so the visualizer is never
		// frozen by a failed read.
		var state struct {
			IsPlaying *bool `json:"is_playing"`
		}
		n.Playing = c.Get("/api/v1/playback/is-playing", &state) != nil || state.IsPlaying == nil || *state.IsPlaying
		return pollMsg{now: n, volOK: e == nil, at: sampledAt}
	}
}

func shuffleLabel(v int) string {
	if v == 1 {
		return "on"
	}
	return "off"
}

func repeatLabel(v int) string {
	switch v {
	case 1:
		return "all"
	case 2:
		return "one"
	default:
		return "off"
	}
}

func (m *Model) active() *list {
	if m.detail {
		return &m.detailList
	}
	return &m.lists[m.tab]
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tickMsg:
		return m, poll(m.client)
	case queueRefreshMsg:
		cmd := m.reloadQueue(false)
		return m, cmd
	case queueRetryMsg:
		if !msg.at.IsZero() && msg.at.Equal(m.queueRetryAt) {
			cmd := m.reloadQueue(false)
			return m, cmd
		}
	case pollMsg:
		// The next tick is scheduled only once this poll has returned, so a
		// slow Cider cannot accumulate overlapping requests.
		next := tick(500 * time.Millisecond)
		if msg.extra {
			next = nil
			m.refreshLoading = false
		}
		if !msg.at.IsZero() && msg.at.Before(m.polledAt) {
			return m, next // stale failures must not overwrite newer successes
		}
		if msg.err != nil {
			m.offline = true
			m.status = errorStatus(msg.err)
			if msg.extra {
				return m, nil
			}
			return m, tick(2 * time.Second)
		}
		m.offline = false
		if !strings.HasPrefix(m.status, "Queue refresh failed: ") {
			m.status = ""
		}
		if !msg.volOK || time.Now().Before(m.volumeHeld) {
			msg.now.Volume = m.now.Volume
		}
		m.now = msg.now
		m.polledAt = msg.at
		if m.polledAt.IsZero() {
			m.polledAt = time.Now()
		}
		cmds := []tea.Cmd{next, m.ensureLyrics()}
		if msg.now.ID != m.lastTrack {
			m.lastTrack = msg.now.ID
			cmds = append(cmds, m.reloadQueue(false))
		} else if m.tab == Queue && time.Since(m.queueLoaded) > 2*time.Second {
			// Keep the Queue tab current while it is shown (changes made in Cider).
			cmds = append(cmds, m.reloadQueue(false))
		}
		return m, tea.Batch(cmds...)
	case queueMsg:
		m.queueLoading = false
		if msg.err != nil {
			m.queueDirty = false
			m.queueRetryAt = time.Now().Add(5 * time.Second)
			m.status = "Queue refresh failed: " + errorStatus(msg.err)
			at := m.queueRetryAt
			return m, tea.Tick(time.Until(at), func(time.Time) tea.Msg { return queueRetryMsg{at: at} })
		}
		if strings.HasPrefix(m.status, "Queue refresh failed: ") {
			m.status = ""
		}
		q := &m.lists[Queue]
		q.rows, q.loaded = msg.rows, true
		m.queueLoaded = time.Now()
		q.cursor = max(0, min(q.cursor, len(q.rows)-1))
		m.queueCurrent = msg.current
		m.queueRetryAt = time.Time{}
		if m.queueDirty {
			m.queueDirty = false
			cmd := m.reloadQueue(true)
			return m, cmd
		}
	case loadedMsg:
		var l *list
		if msg.detail {
			if !m.detail || m.detailKind != msg.tab {
				return m, nil
			}
			l = &m.detailList
		} else {
			l = &m.lists[msg.tab]
		}
		if l.gen != msg.gen {
			return m, nil
		}
		l.loading = false
		if msg.err != nil {
			m.status = errorStatus(msg.err)
			return m, nil
		}
		l.loaded = true
		l.rows = append(l.rows, items(msg.page)...)
		l.next = msg.page.Next
	case actionMsg:
		if msg.err != nil {
			m.status = errorStatus(msg.err)
			return m, nil
		}
		m.status = ""
		if msg.notice != "" {
			m.notice, m.noticeUntil = msg.notice, time.Now().Add(2*time.Second)
		}
		var cmds []tea.Cmd
		if msg.skipped && !m.refreshLoading {
			m.refreshLoading = true
			cmds = append(cmds, refresh(m.client))
		}
		if msg.queued {
			cmds = append(cmds, m.reloadQueue(true))
		}
		return m, tea.Batch(cmds...)
	case vizMsg:
		m.target = msg
		if m.viz != nil {
			return m, waitViz(m.viz)
		}
	case frameMsg:
		m.target, m.waveTarget = msg.Bands, msg.Samples
		return m, waitFrames(m.vizFrames)
	case lyricsMsg:
		if msg.trackID == m.lyricsID {
			m.lyrics, m.lyricsErr, m.lyricsLoading = msg.lines, msg.err, false
		}
	case lyricsFrameMsg:
		return m, lyricsFrame()
	case vizClosedMsg:
		m.vizClosed = true
	case animMsg:
		if m.vizClosed {
			return m, nil
		}
		now := time.Time(msg)
		dt := 1.0 / 60
		if !m.lastFrame.IsZero() {
			dt = min(0.1, max(0, now.Sub(m.lastFrame).Seconds()))
		}
		m.lastFrame = now
		if m.lastWave.IsZero() || now.Sub(m.lastWave) >= waveformInterval {
			m.samples = m.waveTarget
			m.lastWave = now
		}
		target := m.target
		if !m.now.Playing {
			target = nil // paused: settle to rest whatever the capture hears
		}
		m.meter.advance(target, dt)
		return m, animate()
	case tea.KeyMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m Model) key(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := k.String()
	// The offline screen is modal: only quitting or asking for an immediate
	// retry does anything; other keys would just fail against a dead Cider.
	if m.offline {
		switch s {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			return m, refresh(m.client)
		}
		return m, nil
	}
	if m.input {
		switch s {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.input = false
			m.active().filter = ""
		case "enter":
			m.input = false
			switch m.scope() {
			case catalogScope:
				m.searchTerm = m.query
				m.lists[Search] = m.fresh()
				m.lists[Search].loading = true
				return m, search(m.client, m.query, m.lists[Search].gen)
			case libraryScope:
				if strings.TrimSpace(m.query) == "" {
					m.restoreLibrary()
					return m, nil
				}
				if m.libraryFull == nil {
					full := m.lists[Library]
					full.filter = ""
					m.libraryFull = &full
				}
				m.libraryTerm = m.query
				m.lists[Library] = m.fresh()
				m.lists[Library].loading = true
				return m, librarySearch(m.client, m.query, m.libraryAlbums, m.lists[Library].gen)
			}
		case "backspace", "ctrl+h":
			if len(m.query) > 0 {
				_, size := utf8.DecodeLastRuneInString(m.query)
				m.query = m.query[:len(m.query)-size]
			}
		default:
			switch k.Type {
			case tea.KeyRunes:
				m.query += string(k.Runes)
			case tea.KeySpace: // Bubble Tea reports the space bar separately
				m.query += " "
			}
		}
		if m.input { // filter the loaded rows live as you type, on every tab
			l := m.active()
			l.filter, l.cursor = m.query, 0
		}
		return m, nil
	}
	if m.settings {
		switch s {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc", ",":
			m.settings = false
		case "j", "down":
			m.themeCursor = min(len(themeNames)+1, m.themeCursor+1)
		case "k", "up":
			m.themeCursor = max(0, m.themeCursor-1)
		case "enter":
			name, background, waveform := m.theme, m.backgroundOn, m.waveform
			if m.themeCursor == len(themeNames)+1 {
				waveform = !waveform
			} else if m.themeCursor == len(themeNames) {
				background = !background
			} else {
				name = themeNames[m.themeCursor]
			}
			path := m.settingsPath
			if path == "" {
				var err error
				path, err = themeSettingsPath()
				if err != nil {
					m.status = "Could not locate theme settings"
					return m, nil
				}
			}
			visualizer := "spectrum"
			if waveform {
				visualizer = "waveform"
			}
			if err := saveAppearance(path, appearanceSettings{Theme: name, Background: background, Visualizer: visualizer}); err != nil {
				m.status = "Could not save theme settings"
				return m, nil
			}
			_ = applyTheme(name) // selected from the validated preset list
			m.theme, m.settingsPath, m.status = name, path, ""
			m.backgroundOn = background
			m.waveform = waveform
		}
		return m, nil
	}
	if m.lyricsExpanded {
		switch s {
		case "enter", "a", "x", "c", "j", "k", "up", "down", "o", "l", "P":
			return m, nil // do not act on hidden menu rows
		case "/", "tab", "shift+tab", "1", "2", "3", "4", "d", ",", "?", "v":
			m.lyricsExpanded = false
		}
	}
	switch s {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "/":
		m.input = true
		switch m.scope() {
		case catalogScope:
			m.query = m.searchTerm
		case libraryScope:
			m.query = m.libraryTerm
			// Keep the unfiltered library to restore on esc; the live filter
			// below would otherwise leave it with a filter and a reset cursor.
			if m.libraryFull == nil {
				full := m.lists[Library]
				full.filter = ""
				m.libraryFull = &full
			}
		case filterScope:
			m.query = m.active().filter
		}
		return m, nil
	case "tab", "shift+tab", "1", "2", "3", "4":
		switch s {
		case "tab":
			m.tab = (m.tab + 1) % 4
		case "shift+tab":
			m.tab = (m.tab + 3) % 4
		default:
			m.tab = Tab(s[0] - '1')
		}
		m.detail = false
		cmd := m.ensureLoaded()
		return m, cmd
	case "?":
		m.help = !m.help
	case "y":
		m.lyricsExpanded = !m.lyricsExpanded
		m.help, m.fullViz = false, false
	case "d":
		if m.active().cursor < len(m.active().shown()) {
			m.expanded = !m.expanded
			m.help, m.fullViz = false, false
		}
	case ",":
		m.settings, m.help = true, false
		m.fullViz = false
		for i, name := range themeNames {
			if name == m.theme {
				m.themeCursor = i
			}
		}
	case "esc", "backspace":
		switch {
		case m.help:
			m.help = false
		case m.lyricsExpanded:
			m.lyricsExpanded = false
		case m.expanded:
			m.expanded = false
		case m.active().filter != "":
			l := m.active()
			l.filter, l.cursor = "", 0
		case m.detail:
			m.detail = false
		case m.tab == Library:
			m.restoreLibrary()
		}
	case "j", "down":
		l := m.active()
		if l.cursor+1 < len(l.shown()) {
			l.cursor++
		}
		if l.next != "" && !l.loading && len(l.shown())-l.cursor <= 5 {
			l.loading = true
			return m, load(m.client, m.tab, m.detail, l.gen, l.next)
		}
	case "k", "up":
		l := m.active()
		if l.cursor > 0 {
			l.cursor--
		}
	case "enter", "a", "x":
		l := m.active()
		rows := l.shown()
		if l.cursor >= len(rows) {
			return m, nil
		}
		if m.tab == Queue && !m.detail {
			return m, m.queueKey(s, rows)
		}
		if s == "x" {
			return m, nil
		}
		item := rows[l.cursor]
		if s == "enter" && !m.detail && (m.tab == Playlists || m.tab == Library && m.libraryAlbums) {
			m.detail, m.detailKind, m.detailID, m.detailTitle = true, m.tab, item.ID, item.Title
			m.detailList = m.fresh()
			m.detailList.loading = true
			path := "/v1/me/library/albums/" + url.PathEscape(item.ID) + "/tracks?limit=100"
			if m.tab == Playlists {
				path = "/v1/me/library/playlists/" + url.PathEscape(item.ID) + "/tracks?limit=100"
			}
			return m, load(m.client, m.tab, true, m.detailList.gen, path)
		}
		if s == "enter" && m.tab != Search && isSong(item) {
			// Keep playing through the list after the chosen song.
			return m, playFrom(m.client, rows[l.cursor:], item.Title)
		}
		return m, play(m.client, s, item)
	case "o":
		if !m.detail && (m.tab == Library || m.tab == Playlists) {
			m.restoreLibrary()
			m.order[m.tab] = (m.order[m.tab] + 1) % order(len(orderParam))
			m.lists[m.tab] = m.fresh()
			cmd := m.ensureLoaded()
			return m, cmd
		}
	case "l":
		if m.tab == Library && !m.detail {
			m.libraryAlbums = !m.libraryAlbums
			m.libraryFull, m.libraryTerm = nil, ""
			m.lists[Library] = m.fresh()
			cmd := m.ensureLoaded()
			return m, cmd
		}
	case " ", "space":
		return m, post(m.client, "playpause", nil)
	case "n":
		return m, skip(m.client, "next")
	case "p":
		return m, skip(m.client, "previous")
	case "left":
		return m, post(m.client, "seek", map[string]any{"position": max(0, m.now.Position-5)})
	case "right":
		return m, post(m.client, "seek", map[string]any{"position": m.now.Position + 5})
	case "+", "=", "-":
		step := 0.05
		if s == "-" {
			step = -step
		}
		// Apply locally first so quick presses accumulate and the meter moves
		// now; polls are ignored briefly so an in-flight one can't undo it.
		m.now.Volume = min(1, max(0, math.Round((m.now.Volume+step)*100)/100))
		m.volumeHeld = time.Now().Add(1500 * time.Millisecond)
		return m, post(m.client, "volume", map[string]any{"volume": m.now.Volume})
	case "P":
		return m, m.playAll()
	case "v":
		m.fullViz = !m.fullViz
	case "b":
		m.waveform = !m.waveform
	case "s":
		return m, skip(m.client, "toggle-shuffle")
	case "r":
		return m, skip(m.client, "toggle-repeat")
	case "c":
		if m.tab == Queue && !m.detail {
			return m, clearQueue(m.client)
		}
	}
	return m, nil
}

// restoreLibrary puts back the full library list hidden by a search.
func (m *Model) restoreLibrary() {
	if m.libraryFull != nil {
		m.lists[Library] = *m.libraryFull
		m.libraryFull, m.libraryTerm = nil, ""
	}
}

func (m *Model) ensureLoaded() tea.Cmd {
	if m.tab == Queue {
		return m.reloadQueue(false)
	}
	l := &m.lists[m.tab]
	if l.loaded || l.loading || m.tab == Search {
		return nil
	}
	l.loading = true
	path := "/v1/me/library/songs?limit=100"
	if m.tab == Playlists {
		path = "/v1/me/library/playlists?limit=100"
	}
	if m.tab == Library && m.libraryAlbums {
		path = "/v1/me/library/albums?limit=100"
	}
	return load(m.client, m.tab, false, l.gen, path+orderParam[m.order[m.tab]])
}

// reloadQueue coalesces refreshes on the event loop. A completed mutation
// marks an in-flight read dirty so one fresh snapshot follows it, not another
// concurrent request. Background polls respect the failure backoff.
func (m *Model) reloadQueue(afterMutation bool) tea.Cmd {
	if m.queueLoading {
		m.queueDirty = m.queueDirty || afterMutation
		return nil
	}
	if !afterMutation && time.Now().Before(m.queueRetryAt) {
		return nil
	}
	m.queueLoading = true
	m.queueRetryAt = time.Time{} // invalidate a timer if another refresh starts first
	return loadQueue(m.client)
}

func searchPath(storefront, term string) string {
	return "/v1/catalog/" + url.PathEscape(storefront) + "/search?types=songs,albums,playlists&limit=25&term=" + url.QueryEscape(term)
}

func load(c Client, tab Tab, detail bool, gen int, path string) tea.Cmd {
	return func() tea.Msg {
		// Library search pages nest the page under results.<type>.
		var p struct {
			page
			Results map[string]page `json:"results"`
		}
		err := c.AMAPI(path, &p)
		for _, r := range p.Results {
			p.page = r
		}
		return loadedMsg{tab: tab, detail: detail, gen: gen, page: p.page, err: err}
	}
}

func post(c Client, route string, body any) tea.Cmd {
	return func() tea.Msg {
		return actionMsg{err: c.Post("/api/v1/playback/"+route, body, nil)}
	}
}

func play(c Client, key string, item Item) tea.Cmd {
	route, notice := "play-item", ""
	switch key {
	case "a":
		route, notice = "play-later", "Queued: "+item.Title
	}
	return func() tea.Msg {
		err := c.Post("/api/v1/playback/"+route, map[string]any{"type": item.Type, "id": item.ID}, nil)
		return actionMsg{err: err, notice: notice, queued: notice != ""}
	}
}

// ensureLyrics fetches lyrics for the playing track when they are shown and
// not already loaded or loading for it.
func (m *Model) ensureLyrics() tea.Cmd {
	metadataArrived := m.lyricsErr == errNoLyrics && m.now.HasLyrics && m.now.CatalogID != ""
	if m.now.ID == "" || (m.now.ID == m.lyricsID && !metadataArrived) {
		return nil
	}
	m.lyricsID, m.lyrics, m.lyricsErr = m.now.ID, nil, nil
	m.lyricsLoading = false
	if !m.now.HasLyrics || m.now.CatalogID == "" {
		m.lyricsErr = errNoLyrics
		return nil
	}
	m.lyricsLoading = true
	return fetchLyrics(m.client, m.now.ID, m.now.CatalogID)
}

var errNoLyrics = errors.New("no lyrics for this song")

// livePos estimates the playback position now: the last polled position plus
// the time since, so lyrics stay in step between twice-a-second polls.
func (m Model) livePos() float64 {
	pos := m.now.Position
	if m.now.Playing && !m.polledAt.IsZero() {
		pos += time.Since(m.polledAt).Seconds()
	}
	if m.now.Duration > 0 {
		pos = min(pos, m.now.Duration)
	}
	return pos
}

// errQueuePosition stops queue edits when the playing song's position is
// uncertain. Cider's queue list includes songs already played and can hold
// the same song more than once; moving entries around the wrong "current"
// left Cider unable to play.
var errQueuePosition = errors.New("can't tell where the playing song is in Cider's queue (it is missing or listed twice); start a fresh queue with ↵ on a song")

// playingIndex is the playing entry's position in the queue, or ok=false
// unless its ID appears exactly once.
func playingIndex(rows []Item, nowID string) (int, bool) {
	at, n := -1, 0
	for i, r := range rows {
		if nowID != "" && r.ID == nowID {
			at, n = i, n+1
		}
	}
	return at, n == 1
}

// queueKey acts on the Queue tab row under the cursor: ↵ jumps to it and x
// removes it. The filter is not applied here so
// indexes match Cider's queue.
func (m Model) queueKey(key string, _ []Item) tea.Cmd {
	if key != "enter" && key != "x" {
		return nil
	}
	l := &m.lists[Queue]
	rows := l.rows
	if l.filter != "" || l.cursor >= len(rows) {
		return nil // indexes would not match Cider's queue
	}
	cur, ok := playingIndex(rows, m.now.ID)
	if !ok {
		return func() tea.Msg { return actionMsg{err: errQueuePosition} }
	}
	i, title := l.cursor, rows[l.cursor].Title
	if i <= cur && key != "enter" {
		return nil // never rearrange the played part or the playing song
	}
	switch key {
	case "enter":
		return queueAction(m.client, "change-to-index", map[string]any{"index": i}, "Jumped to: "+title)
	case "x":
		if i != cur {
			return queueAction(m.client, "remove-by-index", map[string]any{"index": i}, "Removed: "+title)
		}
	}
	return nil
}

func queueAction(c Client, route string, body map[string]any, notice string) tea.Cmd {
	return func() tea.Msg {
		err := c.Post("/api/v1/playback/queue/"+route, body, nil)
		return actionMsg{err: err, notice: notice, queued: true, skipped: route == "change-to-index"}
	}
}

// clearQueue empties Cider's queue; the current song keeps playing. It needs
// an empty JSON body (a bare POST is a no-op).
func clearQueue(c Client) tea.Cmd {
	return func() tea.Msg {
		err := c.Post("/api/v1/playback/queue/clear-queue", map[string]any{}, nil)
		return actionMsg{err: err, notice: "Queue cleared", queued: true}
	}
}

// queueAhead caps how many following songs are queued when playing from a
// list: one Apple page, a few seconds of play-later requests.
const queueAhead = 100

func isSong(item Item) bool { return strings.Contains(item.Type, "song") }

// playFrom plays the first song of rows and queues the songs after it.
// Cider's play-item replaces the whole queue with that one song, and with
// autoplay off (the default) playback would stop after it.
func playFrom(c Client, rows []Item, title string) tea.Cmd {
	var songs []Item
	for _, r := range rows {
		if isSong(r) && len(songs) <= queueAhead {
			songs = append(songs, r)
		}
	}
	if len(songs) == 0 {
		return nil
	}
	return func() tea.Msg {
		route := "play-item"
		for _, s := range songs {
			if err := c.Post("/api/v1/playback/"+route, map[string]any{"type": s.Type, "id": s.ID}, nil); err != nil {
				return actionMsg{err: err}
			}
			route = "play-later"
		}
		notice := "Playing: " + title
		if n := len(songs) - 1; n > 0 {
			notice += fmt.Sprintf(" · %d more queued", n)
		}
		return actionMsg{notice: notice, skipped: true, queued: true}
	}
}

// playAll plays the open playlist or album from the top. Cider cannot play a
// playlist by ID (play-item accepts it but queues nothing), so the loaded
// tracks are queued instead, shuffled here when shuffle is on. An album row
// plays through Cider's own album playback, which does work.
func (m Model) playAll() tea.Cmd {
	if m.detail {
		rows := append([]Item(nil), m.detailList.rows...)
		title := m.detailTitle
		if m.now.ShuffleMode == "on" {
			rand.Shuffle(len(rows), func(i, j int) { rows[i], rows[j] = rows[j], rows[i] })
			title += " (shuffled)"
		}
		return playFrom(m.client, rows, title)
	}
	rows := m.active().shown()
	if m.active().cursor >= len(rows) || rows[m.active().cursor].Type != "library-albums" {
		return nil
	}
	item := rows[m.active().cursor]
	c := m.client
	return func() tea.Msg {
		err := c.Post("/api/v1/playback/play-item", map[string]any{"type": item.Type, "id": item.ID}, nil)
		return actionMsg{err: err, notice: "Playing: " + item.Title, skipped: true, queued: true}
	}
}

func skip(c Client, route string) tea.Cmd {
	return func() tea.Msg {
		return actionMsg{err: c.Post("/api/v1/playback/"+route, nil, nil), skipped: true}
	}
}

// refresh polls now-playing once, outside the regular tick chain. Playback
// actions use it to update sooner, and the offline screen's r key uses it to
// retry immediately instead of waiting for the 2 s backoff.
func refresh(c Client) tea.Cmd {
	return func() tea.Msg {
		msg := poll(c)().(pollMsg)
		msg.extra = true
		return msg
	}
}

func clock(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	n := int(sec)
	return fmt.Sprintf("%d:%02d", n/60, n%60)
}
