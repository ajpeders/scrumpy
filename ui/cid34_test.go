package ui

// Tests for CID-34: bottom-panel rows show the album next to the
// artist (single-line, "Artist · Album") where the album is meaningful
// for the row type and panel context.

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// findRowContaining picks the first rendered row that contains `needle`.
// Returns false if nothing matched.
func findRowContaining(rows []string, needle string) (string, bool) {
	for _, r := range rows {
		if strings.Contains(ansi.Strip(r), needle) {
			return r, true
		}
	}
	return "", false
}

// viewRows splits m.View() into rows and returns them.
func viewRows(m Model) []string {
	return strings.Split(m.View(), "\n")
}

// TestQueueRowShowsAlbum: a song row on the Queue tab renders
// "Artist · Album" with album present. Wide layout (≥80 columns).
func TestQueueRowShowsAlbum(t *testing.T) {
	m := New(&fakeClient{})
	m.width, m.height = 120, 30
	m.tab = Queue
	m.lists[Queue] = list{loaded: true, rows: []Item{
		{ID: "1", Title: "Title", Subtitle: "Artist", Album: "Album"},
	}}
	rows := viewRows(m)
	if _, ok := findRowContaining(rows, "Artist · Album"); !ok {
		t.Fatalf("Queue row missing 'Artist · Album':\n%s", strings.Join(rows, "\n"))
	}
}

// TestLibrarySongRowShowsAlbum: a Library song row (Library tab, kind
// = songs) shows the album.
func TestLibrarySongRowShowsAlbum(t *testing.T) {
	m := New(&fakeClient{})
	m.width, m.height = 120, 30
	m.tab = Library
	m.libraryKind = libSongs
	m.lists[Library] = list{loaded: true, rows: []Item{
		{ID: "1", Title: "Title", Subtitle: "Artist", Album: "Album", Type: "library-songs"},
	}}
	rows := viewRows(m)
	if _, ok := findRowContaining(rows, "Artist · Album"); !ok {
		t.Fatalf("Library song row missing 'Artist · Album':\n%s", strings.Join(rows, "\n"))
	}
}

// TestLibraryAlbumRowSuppressesAlbum: rows that ARE albums (Library
// tab, kind = albums) must not duplicate the album name next to the
// artist.
func TestLibraryAlbumRowSuppressesAlbum(t *testing.T) {
	m := New(&fakeClient{})
	m.width, m.height = 120, 30
	m.tab = Library
	m.libraryKind = libAlbums
	m.lists[Library] = list{loaded: true, rows: []Item{
		{ID: "1", Title: "Album Name", Subtitle: "Artist", Album: "Album Name", Type: "library-albums"},
	}}
	rows := viewRows(m)
	plain := ansi.Strip(strings.Join(rows, "\n"))
	if strings.Contains(plain, "Artist · Album Name") {
		t.Fatalf("Library album row duplicated its title in subtitle:\n%s", plain)
	}
	if !strings.Contains(plain, "Artist") {
		t.Fatalf("Library album row missing artist entirely:\n%s", plain)
	}
}

// TestPlaylistTrackDetailShowsAlbum: tracks inside a playlist detail
// show their own album (the playlist's name is not the album of its
// tracks).
func TestPlaylistTrackDetailShowsAlbum(t *testing.T) {
	m := New(&fakeClient{})
	m.width, m.height = 120, 30
	m.tab = Playlists
	m.detail = true
	m.detailKind = Playlists
	m.detailTitle = "My Playlist"
	m.detailList = list{loaded: true, rows: []Item{
		{ID: "1", Title: "Track", Subtitle: "Artist", Album: "Album", Type: "songs"},
	}}
	rows := viewRows(m)
	if _, ok := findRowContaining(rows, "Artist · Album"); !ok {
		t.Fatalf("Playlist track missing 'Artist · Album':\n%s", strings.Join(rows, "\n"))
	}
}

// TestAlbumTrackDetailSuppressesAlbum: tracks inside an album detail
// view suppress the album (it's already in the detail header).
func TestAlbumTrackDetailSuppressesAlbum(t *testing.T) {
	m := New(&fakeClient{})
	m.width, m.height = 120, 30
	m.tab = Library
	m.detail = true
	m.detailKind = Library
	m.detailTitle = "Album Name"
	// Under detail, the model reads from detailList.
	m.detailList = list{loaded: true, rows: []Item{
		{ID: "1", Title: "Track", Subtitle: "Artist", Album: "Album Name", Type: "library-songs"},
	}}
	rows := viewRows(m)
	plain := ansi.Strip(strings.Join(rows, "\n"))
	if strings.Contains(plain, "Artist · Album Name") {
		t.Fatalf("Album-track row duplicated parent album name:\n%s", plain)
	}
	if !strings.Contains(plain, "Artist") {
		t.Fatalf("Album-track row missing artist:\n%s", plain)
	}
}

// TestRowAlbumAbsentLeavesArtistOnly: a song row with no album field
// renders the artist alone, with no trailing "· ".
func TestRowAlbumAbsentLeavesArtistOnly(t *testing.T) {
	m := New(&fakeClient{})
	m.width, m.height = 120, 30
	m.tab = Queue
	m.lists[Queue] = list{loaded: true, rows: []Item{
		{ID: "1", Title: "Title", Subtitle: "Artist", Album: ""},
	}}
	plain := ansi.Strip(m.View())
	if !strings.Contains(plain, "Artist") {
		t.Fatalf("Queue row missing artist: %s", plain)
	}
	if strings.Contains(plain, " · ") {
		t.Fatalf("Queue row rendered separator despite empty album:\n%s", plain)
	}
}

// TestRowArtistAbsentLeavesAlbumOnly: a song row with no artist but
// an album falls back to the album.
func TestRowArtistAbsentLeavesAlbumOnly(t *testing.T) {
	m := New(&fakeClient{})
	m.width, m.height = 120, 30
	m.tab = Queue
	m.lists[Queue] = list{loaded: true, rows: []Item{
		{ID: "1", Title: "Title", Subtitle: "", Album: "Album"},
	}}
	plain := ansi.Strip(m.View())
	if !strings.Contains(plain, "Album") {
		t.Fatalf("Queue row missing album fallback: %s", plain)
	}
	if strings.Contains(plain, " · ") {
		t.Fatalf("Queue row rendered separator despite empty artist:\n%s", plain)
	}
}

// TestRowBothAbsentLeavesEmpty: with no artist and no album the
// subtitle column is empty; we never invent metadata.
func TestRowBothAbsentLeavesEmpty(t *testing.T) {
	m := New(&fakeClient{})
	m.width, m.height = 120, 30
	m.tab = Queue
	m.lists[Queue] = list{loaded: true, rows: []Item{
		{ID: "1", Title: "Title", Subtitle: "", Album: ""},
	}}
	for _, line := range strings.Split(m.View(), "\n") {
		plain := ansi.Strip(line)
		if strings.Contains(plain, "Title") && strings.Contains(plain, " · ") {
			t.Fatalf("row rendered separator with no metadata:\n%s", plain)
		}
	}
}

// TestRowLongAlbumTruncatesAtWidth: a long album name truncates with
// an ellipsis instead of pushing past the column boundary.
func TestRowLongAlbumTruncatesAtWidth(t *testing.T) {
	m := New(&fakeClient{})
	m.width, m.height = 120, 30
	m.tab = Queue
	long := strings.Repeat("very-long-album-name-", 6)
	m.lists[Queue] = list{loaded: true, rows: []Item{
		{ID: "1", Title: "Title", Subtitle: "Artist", Album: long},
	}}
	for _, line := range strings.Split(m.View(), "\n") {
		if ansi.StringWidth(line) > m.width {
			t.Fatalf("row overflowed width %d: %q (cells=%d)", m.width, line, ansi.StringWidth(line))
		}
	}
	rows := viewRows(m)
	if _, ok := findRowContaining(rows, "…"); !ok {
		t.Fatalf("long album did not truncate with …:\n%s", strings.Join(rows, "\n"))
	}
}

// TestRowUnicodeMetadataRendersCleanly: Unicode in title/artist/album
// must not desync the row width.
func TestRowUnicodeMetadataRendersCleanly(t *testing.T) {
	m := New(&fakeClient{})
	m.width, m.height = 120, 30
	m.tab = Queue
	m.lists[Queue] = list{loaded: true, rows: []Item{
		{ID: "1", Title: "日本語 タイトル", Subtitle: "Österreichischer Künstler", Album: "Die schöne Müllerin"},
		{ID: "2", Title: "タイトル", Subtitle: "", Album: ""},
	}}
	for _, line := range strings.Split(m.View(), "\n") {
		if ansi.StringWidth(line) > m.width {
			t.Fatalf("Unicode row overflowed width %d: %q (cells=%d)", m.width, line, ansi.StringWidth(line))
		}
	}
	rows := viewRows(m)
	if _, ok := findRowContaining(rows, "Künstler · Die"); !ok {
		t.Fatalf("Unicode artist · album missing:\n%s", strings.Join(rows, "\n"))
	}
}

// TestSelectedRowPreservesAlbumFormat: album is shown even on the
// selected row; selection only changes the styling, not the content.
func TestSelectedRowPreservesAlbumFormat(t *testing.T) {
	m := New(&fakeClient{})
	m.width, m.height = 120, 30
	m.tab = Queue
	m.lists[Queue] = list{loaded: true, rows: []Item{
		{ID: "1", Title: "Title", Subtitle: "Artist", Album: "Album"},
	}}
	m.lists[Queue].cursor = 0
	rows := viewRows(m)
	if _, ok := findRowContaining(rows, "Artist · Album"); !ok {
		t.Fatalf("selected row missing album:\n%s", strings.Join(rows, "\n"))
	}
}

// TestCurrentTrackPreservesAlbumFormat: the row marked as the
// currently-playing track also shows the album.
func TestCurrentTrackPreservesAlbumFormat(t *testing.T) {
	m := New(&fakeClient{})
	m.width, m.height = 120, 30
	m.tab = Queue
	m.now.ID = "playing"
	m.lists[Queue] = list{loaded: true, rows: []Item{
		{ID: "playing", Title: "Title", Subtitle: "Artist", Album: "Album"},
		{ID: "next", Title: "Next", Subtitle: "Artist", Album: "Album"},
	}}
	rows := viewRows(m)
	if _, ok := findRowContaining(rows, "Artist · Album"); !ok {
		t.Fatalf("current-track row missing album:\n%s", strings.Join(rows, "\n"))
	}
}

// TestRowPreservesHeightAcrossWidths: row height does not change at
// any tested width. We assert the frame count fits the same budget
// regardless of how the metadata folds.
func TestRowPreservesHeightAcrossWidths(t *testing.T) {
	for _, w := range []int{120, 80, 50, 40, 24} {
		m := New(&fakeClient{})
		m.width, m.height = w, 30
		m.tab = Queue
		m.lists[Queue] = list{loaded: true, rows: []Item{
			{ID: "1", Title: "Title", Subtitle: "Artist", Album: "Album"},
		}}
		rows := strings.Split(m.View(), "\n")
		if len(rows) > m.height {
			t.Fatalf("width %d: frame exceeded height (%d > %d)", w, len(rows), m.height)
		}
	}
}

// TestNarrowWidthDropsAlbumButKeepsArtist: at narrow widths the
// title eats the available column; the subtitle should still carry
// at least the artist when there's room, and drop album cleanly when
// there isn't.
func TestNarrowWidthDropsAlbumButKeepsArtist(t *testing.T) {
	m := New(&fakeClient{})
	m.width, m.height = 50, 30
	m.tab = Queue
	m.lists[Queue] = list{loaded: true, rows: []Item{
		{ID: "1", Title: "Title", Subtitle: "Artist", Album: "Album"},
	}}
	plain := ansi.Strip(m.View())
	for _, line := range strings.Split(m.View(), "\n") {
		if ansi.StringWidth(line) > m.width {
			t.Fatalf("narrow row overflowed: %q (cells=%d, width=%d)", line, ansi.StringWidth(line), m.width)
		}
	}
	if !strings.Contains(plain, "Title") || !strings.Contains(plain, "Artist") {
		t.Fatalf("narrow row dropped title or artist:\n%s", plain)
	}
}

// TestLibraryArtistsSuppressAlbum: rows in the Library at the
// artists kind suppress the album field.
func TestLibraryArtistsSuppressAlbum(t *testing.T) {
	m := New(&fakeClient{})
	m.width, m.height = 120, 30
	m.tab = Library
	m.libraryKind = libArtists
	m.lists[Library] = list{loaded: true, rows: []Item{
		{ID: "1", Title: "Artist Name", Subtitle: "Artist Name", Album: "Album", Type: "library-artists"},
	}}
	plain := ansi.Strip(m.View())
	if strings.Contains(plain, "Artist Name ·") {
		t.Fatalf("Library artists row rendered separator: %s", plain)
	}
}

// TestPlaylistsTabRowsSuppressAlbum: the Playlists tab itself lists
// playlists, not songs; the subtitle column should not duplicate the
// playlist name as a " · album" suffix.
func TestPlaylistsTabRowsSuppressAlbum(t *testing.T) {
	m := New(&fakeClient{})
	m.width, m.height = 120, 30
	m.tab = Playlists
	m.lists[Playlists] = list{loaded: true, rows: []Item{
		{ID: "1", Title: "Playlist Name", Subtitle: "Curator", Album: "Album", Type: "library-playlists"},
	}}
	plain := ansi.Strip(m.View())
	if strings.Contains(plain, "·") {
		t.Fatalf("Playlists tab row rendered separator: %s", plain)
	}
}

// TestSearchRowShowsAlbumForSongs: Search tab rows of type "songs"
// show the album; non-song types (albums, artists) suppress it.
func TestSearchRowShowsAlbumForSongs(t *testing.T) {
	m := New(&fakeClient{})
	m.width, m.height = 120, 30
	m.tab = Search
	m.lists[Search] = list{loaded: true, rows: []Item{
		{ID: "1", Title: "Track", Subtitle: "Artist", Album: "Album", Type: "songs"},
		{ID: "2", Title: "Album Name", Subtitle: "Artist", Album: "Album Name", Type: "albums"},
		{ID: "3", Title: "Artist Name", Subtitle: "Genre", Album: "", Type: "artists"},
	}}
	plain := ansi.Strip(m.View())
	if !strings.Contains(plain, "Artist · Album") {
		t.Fatalf("Search song row missing 'Artist · Album':\n%s", plain)
	}
	if strings.Contains(plain, "Artist · Album Name") {
		t.Fatalf("Search album row duplicated title as subtitle:\n%s", plain)
	}
}
