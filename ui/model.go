// Package ui implements the terminal remote control. It deliberately depends on
// the small HTTP surface, not on cider's concrete client or its response types.
package ui

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ajpeders/sidecue/viz"
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
	Genre, ReleaseDate               string
	Duration                         float64
	TrackNumber, DiscNumber          int
}

type page struct {
	Data []appleItem `json:"data"`
	Next string      `json:"next"`
}

type appleItem struct {
	ID, Type   string
	Attributes itemAttributes `json:"attributes"`
}

type itemAttributes struct {
	Name, ArtistName, AlbumName string
	GenreNames                  []string `json:"genreNames"`
	DurationInMillis            int64    `json:"durationInMillis"`
	ReleaseDate                 string   `json:"releaseDate"`
	TrackNumber, DiscNumber     int
}

func addItemDetails(item *Item, a itemAttributes) {
	if item.Album == "" {
		item.Album = a.AlbumName
	}
	if item.Genre == "" {
		item.Genre = strings.Join(a.GenreNames, ", ")
	}
	if item.Duration == 0 && a.DurationInMillis > 0 {
		item.Duration = float64(a.DurationInMillis) / 1000
	}
	if item.ReleaseDate == "" {
		item.ReleaseDate = a.ReleaseDate
	}
	if item.TrackNumber == 0 {
		item.TrackNumber = a.TrackNumber
	}
	if item.DiscNumber == 0 {
		item.DiscNumber = a.DiscNumber
	}
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
		if subtitle == "" {
			// An artist row has no artist/album; its genres identify it.
			subtitle = strings.Join(v.Attributes.GenreNames, ", ")
		}
		item := Item{ID: v.ID, Type: v.Type, Title: title, Subtitle: subtitle}
		addItemDetails(&item, v.Attributes)
		out = append(out, item)
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

// libraryKind identifies the resource group for Library and catalog searches.
type libraryKind int

const (
	libSongs libraryKind = iota
	libAlbums
	libArtists
)

var libraryKindNames = [...]string{"Songs", "Albums", "Artists"}

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
	client           Client
	tab              Tab
	lists            [4]list
	query            string
	input            bool
	searchTerm       string
	searchKind       libraryKind
	libraryTerm      string
	libraryFull      *list // the library list hidden while search results show
	order            [4]order
	libraryKind      libraryKind
	detail           bool
	detailTitle      string
	detailID         string
	detailKind       Tab
	detailList       list
	artistType       string // the opened artist's type while viewing its detail
	artistSongs      bool   // artist detail shows top songs (true) or albums (false)
	now              NowPlaying
	queueCurrent     string
	status           string
	offline          bool
	pollFailures     int // tolerate brief API hiccups after a successful connection
	viz              <-chan []float64
	vizFrames        <-chan viz.Frame
	vizClosed        bool // the analyzer stopped: give its rows to the list
	fullViz          bool // v: hide the list, visualizer fills the screen
	waveform         bool // b: switch between spectrum and waveform
	samples          []float64
	waveTarget       []float64 // latest audio hop; displayed at a slower waveform rate
	lastWave         time.Time
	help             bool // ?: the key list replaces the list
	expanded         bool // d: details beneath the selected row
	lyricsID         string
	lyrics           []lyricLine
	lyricsErr        error
	lyricsLoading    bool
	lyricsHidden     bool // y: hide lyrics for this session (band + sidebar both off)
	lyricsOn         bool // show synced lyrics at all
	settings         bool
	theme            string
	backgroundOn     bool     // false leaves the terminal's background untouched
	artwork          bool     // show the current track's cover beside now-playing
	coverLines       []string // pre-rendered cover rows for the active protocol
	cover            *artwork
	coverCols        int
	coverRows        int
	artworkID        string
	artworkLoading   bool
	artworkRetryAt   time.Time
	detailsCover     []string // pre-rendered cover for the d-expanded item
	detailsCoverID   string
	detailsCoverType string
	detailsCoverLoad bool
	detailsKitty     *kittyCache // kitty image; a pointer so View can cache its encoding
	themeCursor      int
	settingsPath     string
	polledAt         time.Time // when now.Position was read; for live position
	meter            meter
	volumeHeld       time.Time // until then, polls keep the locally set volume
	queueLoaded      time.Time // when the Queue tab last refreshed
	queueLoading     bool      // only one queue read may be in flight
	queueDirty       bool      // a mutation happened during that read
	queueRetryAt     time.Time // back off after a failed read
	refreshLoading   bool      // coalesce one-off playback refreshes
	notice           string    // brief confirmation shown in the footer
	noticeUntil      time.Time
	target           []float64 // latest analyzer frame the meter eases towards
	lastFrame        time.Time
	animating        bool // an animMsg chain is in flight; never start a second
	lyricsTicking    bool // a lyricsFrameMsg chain is in flight
	width, height    int
	lastTrack        string
	gen              int
}

func New(client Client) Model {
	return Model{client: client, tab: Queue, width: 80, height: 24, theme: "Terminal", backgroundOn: true, lyricsOn: true, lyricsTicking: true}
}

// WithSettings loads the saved theme for the interactive application. Tests
// and embedded clients can use New without reading the user's config.
func (m Model) WithSettings() Model {
	path, err := themeSettingsPath()
	if err == nil {
		m.settingsPath = path
		var config appearanceSettings
		config, err = loadAppearance(path)
		m.theme, m.backgroundOn = config.Theme, config.Background
		m.waveform, m.artwork = config.Visualizer == "waveform", config.Artwork
		m.lyricsOn = config.Lyrics
	}
	if err == nil {
		err = applyTheme(m.theme)
	}
	if err != nil {
		m.theme = "Terminal"
		m.backgroundOn = true
		m.waveform = false
		m.artwork = false
		m.lyricsOn = true
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
	m.animating = true // Init starts the redraw chain
	return m
}

// WithFrames enables both spectrum and waveform from one analyzer stream.
func (m Model) WithFrames(ch <-chan viz.Frame) Model {
	m.vizFrames = ch
	m.animating = true // Init starts the redraw chain
	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{poll(m.client), func() tea.Msg { return queueRefreshMsg{} }, lyricsFrame()}
	if m.viz != nil {
		cmds = append(cmds, waitViz(m.viz), animate())
	} else if m.vizFrames != nil {
		cmds = append(cmds, animate()) // the anim tick reads the frames
	}
	return tea.Batch(cmds...)
}

type pollMsg struct {
	extra            bool      // a one-off refresh: must not start a second tick chain
	at               time.Time // when now-playing was sampled, before the other reads
	now              NowPlaying
	playingConfirmed bool // is-playing explicitly reported true (not a failed/unknown read)
	volOK            bool // false when the volume read failed; keep the last known value
	err              error
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

// wake restarts the redraw chains that rested while nothing moved. The flags
// make it idempotent, so a chain can never be started twice.
func (m *Model) wake() []tea.Cmd {
	var cmds []tea.Cmd
	if !m.animating && !m.vizClosed && (m.viz != nil || m.vizFrames != nil) && m.now.Playing {
		m.animating = true
		cmds = append(cmds, animate())
	}
	if !m.lyricsTicking && m.now.Playing && len(m.lyrics) > 0 {
		m.lyricsTicking = true
		cmds = append(cmds, lyricsFrame())
	}
	return cmds
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
		stateErr := c.Get("/api/v1/playback/is-playing", &state)
		confirmed := stateErr == nil && state.IsPlaying != nil && *state.IsPlaying
		n.Playing = stateErr != nil || state.IsPlaying == nil || *state.IsPlaying
		return pollMsg{now: n, playingConfirmed: confirmed, volOK: e == nil, at: sampledAt}
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
		if artworkMode == artKitty {
			m.coverCols, m.coverRows = terminalArtworkSize()
			if m.cover != nil {
				m.coverLines = renderCoverSize(m.cover, artworkMode, m.coverCols, m.coverRows)
			}
		}
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
			m.pollFailures++
			m.status = errorStatus(msg.err)
			// Preserve the current layout and controls for isolated failures.
			// Startup failures still show the connection screen immediately.
			if !m.offline && !m.polledAt.IsZero() && m.pollFailures < 3 {
				m.status = "Connection interrupted; retrying: " + m.status
				return m, next
			}
			m.offline = true
			if msg.extra {
				return m, nil
			}
			return m, tick(2 * time.Second)
		}
		m.offline = false
		m.pollFailures = 0
		if !strings.HasPrefix(m.status, "Queue refresh failed: ") {
			m.status = ""
		}
		if !msg.volOK || time.Now().Before(m.volumeHeld) {
			msg.now.Volume = m.now.Volume
		}
		// Cider can briefly return an idle-shaped now-playing response during
		// a track/mode transition even though is-playing still says true. Do
		// not turn that inconsistent pair into "Nothing playing", or invalidate
		// lyrics/artwork and the queue's current track. An explicit false (or
		// an unknown play state) still accepts the empty snapshot normally.
		if msg.playingConfirmed && msg.now.ID == "" && msg.now.Title == "" && m.now.ID != "" {
			previous := m.now
			previous.Position = m.livePos()
			previous.Playing = true
			previous.Volume = msg.now.Volume
			previous.ShuffleMode = msg.now.ShuffleMode
			previous.RepeatMode = msg.now.RepeatMode
			msg.now = previous
		}
		m.now = msg.now
		m.polledAt = msg.at
		if m.polledAt.IsZero() {
			m.polledAt = time.Now()
		}
		cmds := append([]tea.Cmd{next, m.ensureLyrics(), m.ensureArtwork()}, m.wake()...)
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
			cmds := append(m.wake(), waitViz(m.viz))
			return m, tea.Batch(cmds...)
		}
	case lyricsMsg:
		if msg.trackID == m.lyricsID {
			m.lyrics, m.lyricsErr, m.lyricsLoading = msg.lines, msg.err, false
			cmds := m.wake()
			return m, tea.Batch(cmds...)
		}
	case artworkMsg:
		if msg.catalogID == m.artworkID {
			m.artworkLoading = false
			m.coverLines = nil
			m.cover = nil
			m.artworkRetryAt = time.Time{}
			if msg.err == nil && msg.cover != nil {
				m.cover = msg.cover
				cols, rows := m.artworkSize()
				m.coverLines = renderCoverSize(msg.cover, artworkMode, cols, rows)
			} else {
				m.artworkRetryAt = time.Now().Add(30 * time.Second)
			}
		}
	case copiedMsg:
		m.status = msg.status
	case detailsCoverMsg:
		if msg.id == m.detailsCoverID {
			m.detailsCoverLoad = false
			if msg.err == nil && msg.cover != nil {
				m.detailsCover = renderCover(msg.cover, artworkMode)
				m.detailsKitty = &kittyCache{png: msg.cover.png} // kitty placement is sized per panel
			}
		}
	case lyricsFrameMsg:
		// Rest when there is nothing moving; wake restarts the chain.
		if !m.now.Playing || len(m.lyrics) == 0 {
			m.lyricsTicking = false
			return m, nil
		}
		return m, lyricsFrame()
	case vizClosedMsg:
		m.vizClosed = true
	case animMsg:
		// Take the newest analyzer frame on the redraw clock. Delivering frames
		// as messages too would double the redraws: Bubble Tea renders after
		// every message, and both arrive about 60 times per second.
		if m.vizFrames != nil {
			select {
			case frame, ok := <-m.vizFrames:
				if ok {
					m.target, m.waveTarget = frame.Bands, frame.Samples
				} else {
					m.vizClosed = true
				}
			default:
			}
		}
		if m.vizClosed {
			m.animating = false
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
		if !m.now.Playing && m.meter.atRest() {
			m.animating = false // wake restarts it when playback resumes
			return m, nil
		}
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
			if m.refreshLoading {
				return m, nil
			}
			m.refreshLoading = true
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
				return m, search(m.client, m.query, m.searchKind, m.lists[Search].gen)
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
				return m, librarySearch(m.client, m.query, m.libraryKind, m.lists[Library].gen)
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
	// Settings consumes navigation keys, but transport controls remain global
	// (except while typing into a search field, handled above).
	playbackKey := false
	switch s {
	case "n", "p", " ", "space", "left", "right", "+", "=", "-", "s", "r":
		playbackKey = true
	}
	// Help is a modal surface. While it's open we only let the user dismiss
	// it (?, esc), quit, or run a transport key. Block every key that would
	// otherwise act on a hidden list (j/k, d, x, enter, a, l, P, Y, c, o,
	// tab, 1-4, v, y, b, /) so a stray press can't quietly mutate state
	// behind the overlay.
	if m.help && !playbackKey {
		switch s {
		case "?", "esc", "backspace", "q", "ctrl+c":
			// fall through to the switch below
		default:
			return m, nil
		}
	}
	if m.settings && !playbackKey {
		switch s {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc", ",":
			m.settings = false
		case "j", "down":
			m.themeCursor = min(len(themeNames)+3, m.themeCursor+1)
		case "k", "up":
			m.themeCursor = max(0, m.themeCursor-1)
		case "enter":
			name, background, waveform, artwork, lyrics := m.theme, m.backgroundOn, m.waveform, m.artwork, m.lyricsOn
			switch {
			case m.themeCursor == len(themeNames)+3:
				lyrics = !lyrics
			case m.themeCursor == len(themeNames)+2:
				artwork = !artwork
			case m.themeCursor == len(themeNames)+1:
				waveform = !waveform
			case m.themeCursor == len(themeNames):
				background = !background
			default:
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
			if err := saveAppearance(path, appearanceSettings{Theme: name, Background: background, Visualizer: visualizer, Artwork: artwork, Lyrics: lyrics}); err != nil {
				m.status = "Could not save theme settings"
				return m, nil
			}
			_ = applyTheme(name) // selected from the validated preset list
			m.theme, m.settingsPath, m.status = name, path, ""
			m.backgroundOn = background
			m.waveform = waveform
			m.artwork = artwork
			m.lyricsOn = lyrics
			if !lyrics {
				m.lyricsHidden = false
			}
			artCmd := m.ensureArtwork()
			if !artwork {
				m.clearDetailsCover()
			}
			lyricsCmd := m.ensureLyrics()
			return m, tea.Batch(artCmd, lyricsCmd)
		}
		return m, nil
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
		m.artistType, m.artistSongs = "", false
		cmd := m.ensureLoaded()
		return m, cmd
	case "?":
		m.help = !m.help
	case "y":
		if m.lyricsOn {
			m.lyricsHidden = !m.lyricsHidden
			if m.lyricsHidden {
				m.notice, m.noticeUntil = "Lyrics hidden", time.Now().Add(2*time.Second)
			} else {
				m.notice, m.noticeUntil = "Lyrics on", time.Now().Add(2*time.Second)
			}
		}
		m.help, m.fullViz = false, false
	case "d":
		if m.active().cursor < len(m.active().shown()) {
			m.expanded = !m.expanded
			m.help, m.fullViz = false, false
			if !m.expanded {
				m.clearDetailsCover()
			} else if cmd := m.maybeDetailsCover(); cmd != nil {
				return m, cmd
			}
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
		case m.expanded:
			m.expanded = false
		case m.fullViz:
			// Leave full-screen visualizer before doing anything else: this
			// is the easiest way out when the only visible surface is the
			// bars and the user wants their queue back.
			m.fullViz = false
		case m.active().filter != "":
			l := m.active()
			l.filter, l.cursor = "", 0
		case m.detail:
			m.detail, m.artistType, m.artistSongs = false, "", false
		case m.tab == Library:
			m.restoreLibrary()
		}
	case "j", "down":
		l := m.active()
		if l.cursor+1 < len(l.shown()) {
			l.cursor++
		}
		cover := m.maybeDetailsCover()
		if l.next != "" && !l.loading && len(l.shown())-l.cursor <= 5 {
			l.loading = true
			return m, tea.Batch(cover, load(m.client, m.tab, m.detail, l.gen, l.next))
		}
		if cover != nil {
			return m, cover
		}
	case "k", "up":
		l := m.active()
		if l.cursor > 0 {
			l.cursor--
		}
		if cmd := m.maybeDetailsCover(); cmd != nil {
			return m, cmd
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
		if isArtist(item) {
			if s == "enter" && !m.detail {
				// An artist has no playable ID: open its songs/albums instead.
				// Catalog artists default to top songs; library artists to albums.
				m.detail, m.detailKind, m.detailID, m.detailTitle = true, m.tab, item.ID, item.Title
				m.artistType = item.Type
				m.artistSongs = item.Type != "library-artists"
				m.detailList = m.fresh()
				m.detailList.loading = true
				return m, artistContent(m.client, m.tab, item, m.artistSongs, m.detailList.gen)
			}
			return m, nil
		}
		if s == "enter" && !m.detail && (m.tab == Playlists || m.tab == Library && m.libraryKind == libAlbums) {
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
		if m.tab == Search && !m.detail {
			m.searchKind = (m.searchKind + 1) % 3
			m.lists[Search] = m.fresh()
			if strings.TrimSpace(m.searchTerm) != "" {
				m.lists[Search].loading = true
				return m, search(m.client, m.searchTerm, m.searchKind, m.lists[Search].gen)
			}
			return m, nil
		}
		if m.detail && m.artistType != "" && m.artistType != "library-artists" {
			// Flip a catalog artist's detail between top songs and albums.
			m.artistSongs = !m.artistSongs
			m.detailList = m.fresh()
			m.detailList.loading = true
			item := Item{ID: m.detailID, Type: m.artistType, Title: m.detailTitle}
			return m, artistContent(m.client, m.tab, item, m.artistSongs, m.detailList.gen)
		}
		if m.tab == Library && !m.detail {
			m.libraryKind = (m.libraryKind + 1) % 3
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
	case "left", "right":
		// Scrub from the live position and apply locally first, so rapid
		// presses accumulate instead of all seeking to the same stale snapshot.
		delta := -5.0
		if s == "right" {
			delta = 5
		}
		target := max(0, m.livePos()+delta)
		if m.now.Duration > 0 {
			target = min(target, m.now.Duration)
		}
		m.now.Position = target
		m.polledAt = time.Now()
		return m, post(m.client, "seek", map[string]any{"position": target})
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
	case "Y":
		if rows := m.active().shown(); m.active().cursor < len(rows) {
			return m, copyCmd(rows[m.active().cursor])
		}
	case "c":
		if m.tab == Queue && !m.detail && m.active().filter == "" {
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
	if m.tab == Library {
		switch m.libraryKind {
		case libAlbums:
			path = "/v1/me/library/albums?limit=100"
		case libArtists:
			path = "/v1/me/library/artists?limit=100"
		}
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

func searchPath(storefront, term string, kind libraryKind) string {
	types := [...]string{"songs", "albums", "artists"}[kind]
	return "/v1/catalog/" + url.PathEscape(storefront) + "/search?types=" + types + "&limit=25&term=" + url.QueryEscape(term)
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
	if !m.lyricsOn {
		return nil
	}
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

// ensureArtwork fetches the cover for the playing track when artwork is shown.
// It mirrors ensureLyrics: only one fetch per track, keyed by the catalog ID,
// or the library ID for library-only tracks. It is a no-op when the setting
// is off or the track has no ID.
func (m *Model) ensureArtwork() tea.Cmd {
	id := cmp.Or(m.now.CatalogID, m.now.ID)
	if !m.artwork || id == "" {
		m.coverLines, m.artworkID = nil, ""
		m.cover = nil
		m.artworkLoading = false
		m.artworkRetryAt = time.Time{}
		return nil
	}
	if id == m.artworkID && (m.artworkRetryAt.IsZero() || time.Now().Before(m.artworkRetryAt)) {
		return nil
	}
	m.artworkID, m.coverLines, m.artworkLoading = id, nil, true
	m.cover = nil
	m.artworkRetryAt = time.Time{}
	return fetchArtwork(m.client, id)
}

// maybeDetailsCover fetches a cover for the currently expanded item when one
// is missing. It is called from j/k/d so selection movement pre-loads the art.
func (m *Model) maybeDetailsCover() tea.Cmd {
	if !m.expanded {
		return nil
	}
	visible := m.active().shown()
	if m.active().cursor >= len(visible) {
		return nil
	}
	return m.ensureDetailsCover(visible[m.active().cursor])
}

// clearDetailsCover drops any in-flight or rendered cover for the d panel.
func (m *Model) clearDetailsCover() {
	m.detailsCover, m.detailsCoverID, m.detailsCoverType = nil, "", ""
	m.detailsCoverLoad = false
}

// ensureDetailsCover fetches a small cover for the item the details panel is
// about to show. It mirrors ensureArtwork but is keyed by catalogID+Type and is
// a no-op when the artwork setting is off or the item has no art (artists).
func (m *Model) ensureDetailsCover(item Item) tea.Cmd {
	if !m.artwork || !m.expanded || item.ID == "" {
		m.detailsCover, m.detailsCoverID, m.detailsCoverType = nil, "", ""
		m.detailsCoverLoad = false
		return nil
	}
	resource := artworkResource(item.Type)
	if resource == "" {
		m.detailsCover, m.detailsCoverID, m.detailsCoverType = nil, "", ""
		m.detailsCoverLoad = false
		return nil
	}
	key := resource + ":" + item.ID
	if key == m.detailsCoverID {
		return nil
	}
	m.detailsCover, m.detailsCoverID, m.detailsCoverType = nil, key, item.Type
	m.detailsCoverLoad = true
	return fetchDetailsCover(m.client, item.ID, resource)
}

// fetchDetailsCover loads the cover used in the details panel. Reuses the
// existing artwork cache; falls through to the same artwork pipeline so a
// repeated selection shows instantly after the first view.
func fetchDetailsCover(c Client, catalogID, resource string) tea.Cmd {
	return func() tea.Msg {
		cov, err := loadArtworkFor(c, catalogID, resource)
		return detailsCoverMsg{id: resource + ":" + catalogID, cover: cov, err: err}
	}
}

// detailsCoverMsg carries the cover decoded for the d-expanded item. A non-nil
// err means the cover was not available; the panel shows text only.
type detailsCoverMsg struct {
	id    string
	cover *artwork
	err   error
}

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

// playingIndex returns the position of the playing entry in the queue.
//
// Cider's queue endpoints do not preserve a "current" flag on each row
// in the response — instead the row's `IsCurrent`/`Current` boolean is
// surfaced as the `current` field on the parsed queueMsg. The handler
// that updates the queue records that as `m.queueCurrent`.
//
// Resolution rules, in order:
//  1. If `current` is set and matches exactly one row, return its index.
//     Cider told us which row is playing; trust it even when other rows
//     share the same ID (the user can `a`-enqueue a song that's already
//     queued, or shuffle can yield the same ID twice in a row).
//  2. Otherwise the queue is ambiguous and we refuse to guess: returning
//     ok=false makes the handler surface a clear "can't tell where the
//     playing song is" error rather than risk moving the wrong entry.
func playingIndex(rows []Item, current, nowID string) (int, bool) {
	if current != "" {
		at, n := -1, 0
		for i, r := range rows {
			if r.ID == current {
				at, n = i, n+1
			}
		}
		if n == 1 {
			return at, true
		}
		return -1, false
	}
	if nowID == "" {
		return -1, false
	}
	at, n := -1, 0
	for i, r := range rows {
		if r.ID == nowID {
			at, n = i, n+1
		}
	}
	if n == 1 {
		return at, true
	}
	return -1, false
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
	cur, ok := playingIndex(rows, m.queueCurrent, m.now.ID)
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

// isArtist matches both catalog ("artists") and library ("library-artists") rows.
func isArtist(item Item) bool { return strings.Contains(item.Type, "artist") }

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
