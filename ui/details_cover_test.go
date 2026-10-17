package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// coverItem returns an item whose details cover is expected to fetch. The
// settings toggle and type dictate whether a fetch actually happens.
func coverItem() Item {
	return Item{ID: "123", Type: "songs", Title: "Song", Subtitle: "Artist", Album: "Album"}
}

func TestDetailsCoverFetchesOnlyWhenArtworkOn(t *testing.T) {
	for _, on := range []bool{false, true} {
		m := New(&fakeClient{})
		m.artwork = on
		m.expanded = true
		m.lists[Queue] = list{loaded: true, rows: []Item{coverItem()}}
		m.lists[Queue].cursor = 0
		cmd := m.maybeDetailsCover()
		if on && cmd == nil {
			t.Fatal("artwork on: expected a fetch command")
		}
		if !on && cmd != nil {
			t.Fatalf("artwork off: unexpected fetch %v", cmd)
		}
	}
}

func TestDetailsCoverKeysByResourceAndID(t *testing.T) {
	for _, typ := range []string{"songs", "albums", "playlists", "library-albums", "library-playlists", "music-videos"} {
		m := New(&fakeClient{})
		m.artwork = true
		m.expanded = true
		m.lists[Queue] = list{loaded: true, rows: []Item{{ID: "1", Type: typ, Title: "X"}}}
		m.lists[Queue].cursor = 0
		if cmd := m.maybeDetailsCover(); cmd == nil {
			t.Fatalf("%s: expected fetch", typ)
		}
		want := artworkResource(typ) + ":1"
		if m.detailsCoverID != want {
			t.Fatalf("%s: key %q want %q", typ, m.detailsCoverID, want)
		}
	}
}

func TestDetailsCoverSkipsArtistsAndEmptyID(t *testing.T) {
	for _, item := range []Item{
		{ID: "1", Type: "artists", Title: "A"},
		{ID: "1", Type: "library-artists", Title: "A"},
		{ID: "", Type: "songs", Title: "X"},
	} {
		m := New(&fakeClient{})
		m.artwork = true
		m.expanded = true
		m.lists[Queue] = list{loaded: true, rows: []Item{item}}
		m.lists[Queue].cursor = 0
		if cmd := m.maybeDetailsCover(); cmd != nil {
			t.Fatalf("%+v: unexpected fetch", item)
		}
	}
}

func TestDetailsCoverClearsOnCollapseAndSettingsOff(t *testing.T) {
	old := artworkMode
	artworkMode = artBlock
	t.Cleanup(func() { artworkMode = old })

	m := New(&fakeClient{})
	m.artwork = true
	m.expanded = true
	m.lists[Queue] = list{loaded: true, rows: []Item{coverItem()}}
	m.lists[Queue].cursor = 0
	m.maybeDetailsCover()
	updated, _ := m.Update(detailsCoverMsg{id: m.detailsCoverID, cover: blankArt()})
	m = updated.(Model)
	if m.detailsCover == nil {
		t.Fatal("cover did not render on success")
	}
	m.expanded = false
	m.clearDetailsCover()
	if m.detailsCover != nil || m.detailsCoverID != "" || m.detailsCoverLoad {
		t.Fatal("clear did not reset cover state")
	}
}

func TestDetailsCoverMessageDropsStaleFetch(t *testing.T) {
	old := artworkMode
	artworkMode = artBlock
	t.Cleanup(func() { artworkMode = old })

	m := New(&fakeClient{})
	m.artwork = true
	m.expanded = true
	m.lists[Queue] = list{loaded: true, rows: []Item{coverItem()}}
	m.lists[Queue].cursor = 0
	m.maybeDetailsCover()
	// Simulate a stale fetch arriving after selection moved on.
	updated, _ := m.Update(detailsCoverMsg{id: "songs:OTHER", cover: blankArt()})
	m = updated.(Model)
	if m.detailsCover != nil {
		t.Fatal("stale cover was applied")
	}
	// Apply the right one.
	updated, _ = m.Update(detailsCoverMsg{id: m.detailsCoverID, cover: blankArt()})
	m = updated.(Model)
	if m.detailsCover == nil {
		t.Fatal("matching cover was not applied")
	}
}

func TestDetailsPanelLaysCoverBesideMetadata(t *testing.T) {
	old := artworkMode
	artworkMode = artBlock
	t.Cleanup(func() { artworkMode = old })

	m := New(&fakeClient{})
	m.artwork = true
	m.expanded = true
	m.lists[Queue] = list{loaded: true, rows: []Item{coverItem()}}
	m.lists[Queue].cursor = 0
	m.maybeDetailsCover()
	updated, _ := m.Update(detailsCoverMsg{id: m.detailsCoverID, cover: blankArt()})
	m = updated.(Model)
	if m.detailsCover == nil {
		t.Fatalf("cover did not render for w loop: %+v", m.detailsCoverID)
	}

	for _, w := range []int{60, 45} {
		rows := m.itemDetailsLines(m.lists[Queue].rows[0], w, 25)
		joined := ansi.Strip(strings.Join(rows, "\n"))
		if !strings.Contains(joined, "Song") || !strings.Contains(joined, "Album") {
			t.Fatalf("w=%d: metadata missing:\n%s", w, joined)
		}
		if w >= detailsPanelCoverMinW && !strings.Contains(joined, "█") {
			t.Fatalf("w=%d: cover column missing:\n%s", w, joined)
		}
		for _, r := range rows {
			if ansi.StringWidth(r) > w {
				t.Fatalf("w=%d: row %q exceeds width", w, r)
			}
		}
	}

	// Narrow panels drop the cover column entirely.
	rows := m.itemDetailsLines(m.lists[Queue].rows[0], 18, 10)
	for _, r := range rows {
		if ansi.StringWidth(r) > 18 {
			t.Fatalf("narrow: row %q overflows", r)
		}
	}
}

func TestDetailsPanelWithoutCoverUnchanged(t *testing.T) {
	m := New(&fakeClient{})
	m.artwork = true
	m.expanded = true
	m.lists[Queue] = list{loaded: true, rows: []Item{{ID: "1", Type: "songs", Title: "Song", Subtitle: "Artist", Album: "Album"}}}
	m.lists[Queue].cursor = 0
	rows := m.itemDetailsLines(m.lists[Queue].rows[0], 50, 20)
	joined := ansi.Strip(strings.Join(rows, "\n"))
	if !strings.HasPrefix(joined, "╭─ Details") {
		t.Fatal("details header missing")
	}
	if strings.Contains(joined, "█") {
		t.Fatal("unexpected cover glyphs without a loaded cover")
	}
}

func TestDetailsCoverFetchErrorLeavesPanelUsable(t *testing.T) {
	m := New(&fakeClient{})
	m.artwork = true
	m.expanded = true
	m.lists[Queue] = list{loaded: true, rows: []Item{coverItem()}}
	m.lists[Queue].cursor = 0
	m.maybeDetailsCover()
	updated, _ := m.Update(detailsCoverMsg{id: m.detailsCoverID, err: errors.New("boom")})
	m = updated.(Model)
	rows := m.itemDetailsLines(m.lists[Queue].rows[0], 50, 20)
	joined := ansi.Strip(strings.Join(rows, "\n"))
	if !strings.Contains(joined, "Song") || !strings.Contains(joined, "Album") {
		t.Fatalf("metadata vanished after failed cover fetch:\n%s", joined)
	}
}

func blankArt() *artwork {
	// A solid-colour RGBA so the quadrant render fills with a block glyph.
	img := image.NewRGBA(image.Rect(0, 0, artworkWide, artworkHigh))
	red := color.RGBA{255, 0, 0, 255}
	for y := 0; y < artworkHigh; y++ {
		for x := 0; x < artworkWide; x++ {
			img.SetRGBA(x, y, red)
		}
	}
	return &artwork{block: img}
}

// In kitty mode the cover is one zero-width escape row; the panel must still
// reserve the image's columns so metadata is not painted over.
func TestDetailsPanelReservesCoverColumnsInKittyMode(t *testing.T) {
	old := artworkMode
	artworkMode = artKitty
	t.Cleanup(func() { artworkMode = old })

	m := New(&fakeClient{})
	m.artwork = true
	m.expanded = true
	m.lists[Queue] = list{loaded: true, rows: []Item{coverItem()}}
	m.maybeDetailsCover()
	updated, _ := m.Update(detailsCoverMsg{id: m.detailsCoverID, cover: buildArtwork(image.NewRGBA(image.Rect(0, 0, 32, 32)))})
	m = updated.(Model)

	rows := m.itemDetailsLines(m.lists[Queue].rows[0], 60, 25)
	if len(rows) < 3 {
		t.Fatalf("panel too short: %d rows", len(rows))
	}
	// Row 1 is the first body row: image cells, gutter, then the Title label.
	body := ansi.Strip(rows[1])
	idx := strings.Index(body, "Title")
	if idx < 1+artworkCols+1 {
		t.Fatalf("metadata starts at col %d, want >= %d (cover columns not reserved):\n%q", idx, 1+artworkCols+1, body)
	}
}

// Opening d, or moving down with j, must start the cover fetch; only k did.
func TestDetailsCoverFetchStartsOnDAndJ(t *testing.T) {
	for _, keys := range []string{"d", "dj"} {
		m := New(&fakeClient{})
		m.artwork = true
		m.lists[Queue] = list{loaded: true, rows: []Item{coverItem(), {ID: "456", Type: "songs", Title: "Two"}}}
		var cmd tea.Cmd
		for _, k := range keys {
			var u tea.Model
			u, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{k}})
			m = u.(Model)
		}
		want := "songs:123"
		if keys == "dj" {
			want = "songs:456"
		}
		if cmd == nil || m.detailsCoverID != want {
			t.Fatalf("keys %q: cmd=%v detailsCoverID=%q, want fetch for %q", keys, cmd, m.detailsCoverID, want)
		}
	}
}

// Library items (queue entries, library tabs) carry "i."/"l."/"p." IDs that the
// catalog endpoint does not know; their artwork lives under /v1/me/library.
func TestArtworkURLUsesLibraryEndpointForLibraryIDs(t *testing.T) {
	cases := map[string]string{
		"i.xrXk1PPTOxplxx": "/v1/me/library/songs/i.xrXk1PPTOxplxx",
		"1472484927":       "/v1/catalog/us/songs/1472484927",
	}
	for id, want := range cases {
		f := &fakeClient{}
		f.amapi = func(path string, out any) error {
			if strings.Contains(path, "storefront") {
				return json.Unmarshal([]byte(`{"data":[{"id":"us"}]}`), out)
			}
			return json.Unmarshal([]byte(`{"data":[{"attributes":{"artwork":{"url":"https://x/{w}x{h}.jpg"}}}]}`), out)
		}
		if _, err := artworkURL(f, id, "songs"); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if got := f.last().path; got != want {
			t.Fatalf("%s: path %q, want %q", id, got, want)
		}
	}
}

// The details cover must not share the now-playing image id (a new transmit
// with the same id replaces the other placement), and must be deleted once
// the panel no longer draws it.
func TestDetailsCoverUsesOwnKittyImage(t *testing.T) {
	old := artworkMode
	artworkMode = artKitty
	t.Cleanup(func() { artworkMode = old })

	m := New(&fakeClient{})
	m.artwork = true
	m.width, m.height = 100, 40
	m.lists[Queue] = list{loaded: true, rows: []Item{coverItem()}}
	u, _ := m.Update(runeKey('d'))
	m = u.(Model)
	u, _ = m.Update(detailsCoverMsg{id: m.detailsCoverID, cover: buildArtwork(image.NewRGBA(image.Rect(0, 0, 32, 32)))})
	m = u.(Model)
	place := fmt.Sprintf(",i=%d,", detailsImageID)
	if detailsImageID == kittyImageID || !strings.Contains(m.View(), place) {
		t.Fatal("details cover not placed with its own id")
	}
	del := fmt.Sprintf("a=d,d=I,i=%d,", detailsImageID)
	if strings.Contains(m.View(), del) {
		t.Fatal("details image deleted while the panel shows it")
	}
	u, _ = m.Update(runeKey('d'))
	m = u.(Model)
	if !strings.Contains(m.View(), del) {
		t.Fatal("details image not deleted after collapsing")
	}
}

// Cider's queue reports singular types ("song"), unlike Apple's "songs".
func TestArtworkResourceAcceptsQueueSingularTypes(t *testing.T) {
	for typ, want := range map[string]string{"song": "songs", "album": "albums", "playlist": "playlists", "music-video": "music-videos", "artist": ""} {
		if got := artworkResource(typ); got != want {
			t.Fatalf("artworkResource(%q) = %q, want %q", typ, got, want)
		}
	}
}

// A kitty image paints its full c×r cells regardless of the text, so in a
// short panel it must shrink to the rows the box has rather than spill out.
func TestDetailsKittyCoverFitsPanelHeight(t *testing.T) {
	old := artworkMode
	artworkMode = artKitty
	t.Cleanup(func() { artworkMode = old })

	m := New(&fakeClient{})
	m.artwork = true
	m.lists[Queue] = list{loaded: true, rows: []Item{coverItem()}}
	u, _ := m.Update(runeKey('d'))
	m = u.(Model)
	u, _ = m.Update(detailsCoverMsg{id: m.detailsCoverID, cover: buildArtwork(image.NewRGBA(image.Rect(0, 0, 32, 32)))})
	m = u.(Model)

	rows := m.itemDetailsLines(m.lists[Queue].rows[0], 70, 7)
	if len(rows) > 7 {
		t.Fatalf("panel has %d rows, want <= 7", len(rows))
	}
	joined := strings.Join(rows, "\n")
	if !strings.Contains(joined, ",c=10,r=5,") {
		t.Fatalf("cover not sized to the 5 body rows: %q", joined[:min(120, len(joined))])
	}
	body := ansi.Strip(rows[1])
	if idx := strings.Index(body, "Title"); idx < 1+10+1 {
		t.Fatalf("metadata at col %d overlaps the shrunk cover: %q", idx, body)
	}
}
