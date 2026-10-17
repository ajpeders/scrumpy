package ui

// Renders fixed-width before/after examples of the bottom-panel row
// format. The test always passes; its only purpose is to keep the
// example output in sync with the renderer so the doc can quote
// actual strings.

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderCID34Examples(t *testing.T) {
	mkExample := func(label string, m Model) string {
		var b strings.Builder
		b.WriteString("\n--- ")
		b.WriteString(label)
		b.WriteString(" ---\n")
		for _, line := range strings.Split(m.View(), "\n") {
			b.WriteString(ansi.Strip(line))
			b.WriteString("\n")
		}
		return b.String()
	}

	// Wide (120 cols), Queue, mixed albums.
	qWide := New(&fakeClient{})
	qWide.width, qWide.height = 120, 16
	qWide.tab = Queue
	qWide.lists[Queue] = list{loaded: true, rows: []Item{
		{ID: "1", Title: "A-Punk", Subtitle: "Vampire Weekend", Album: "Vampire Weekend"},
		{ID: "2", Title: "Sunflower", Subtitle: "Post Malone", Album: "Into the Spider-Verse"},
		{ID: "3", Title: "Hello", Subtitle: "Adele", Album: "25"},
		{ID: "4", Title: "Untitled", Subtitle: "Unknown artist", Album: ""},
		{ID: "5", Title: "Solo (Remastered 2011)", Subtitle: "An Extremely Long-Winded Artist", Album: "Without a Net"},
	}}

	// Standard (80 cols), Queue.
	qStd := New(&fakeClient{})
	qStd.width, qStd.height = 80, 16
	qStd.tab = Queue
	qStd.lists[Queue] = list{loaded: true, rows: []Item{
		{ID: "1", Title: "A-Punk", Subtitle: "Vampire Weekend", Album: "Vampire Weekend"},
		{ID: "2", Title: "Sunflower", Subtitle: "Post Malone", Album: "Into the Spider-Verse"},
		{ID: "3", Title: "Hello", Subtitle: "Adele", Album: "25"},
	}}

	// Narrow (40 cols), Queue.
	qNar := New(&fakeClient{})
	qNar.width, qNar.height = 40, 16
	qNar.tab = Queue
	qNar.lists[Queue] = list{loaded: true, rows: []Item{
		{ID: "1", Title: "A-Punk", Subtitle: "Vampire Weekend", Album: "Vampire Weekend"},
		{ID: "2", Title: "Hello", Subtitle: "Adele", Album: "25"},
	}}

	// Library songs wide.
	libSongsView := New(&fakeClient{})
	libSongsView.width, libSongsView.height = 120, 16
	libSongsView.tab = Library
	libSongsView.libraryKind = libSongs
	libSongsView.lists[Library] = list{loaded: true, rows: []Item{
		{ID: "1", Title: "A-Punk", Subtitle: "Vampire Weekend", Album: "Vampire Weekend", Type: "library-songs"},
	}}

	// Library albums: album should NOT be duplicated next to the artist.
	libAlbumsView := New(&fakeClient{})
	libAlbumsView.width, libAlbumsView.height = 120, 16
	libAlbumsView.tab = Library
	libAlbumsView.libraryKind = libAlbums
	libAlbumsView.lists[Library] = list{loaded: true, rows: []Item{
		{ID: "1", Title: "Vampire Weekend", Subtitle: "Vampire Weekend", Album: "Vampire Weekend", Type: "library-albums"},
	}}

	// Album tracks drill-down: parent header says "Album Name"; rows
	// must NOT show the album again next to the artist.
	albumDrill := New(&fakeClient{})
	albumDrill.width, albumDrill.height = 120, 16
	albumDrill.tab = Library
	albumDrill.detail = true
	albumDrill.detailKind = Library
	albumDrill.detailTitle = "Vampire Weekend"
	albumDrill.detailList = list{loaded: true, rows: []Item{
		{ID: "1", Title: "A-Punk", Subtitle: "Vampire Weekend", Album: "Vampire Weekend", Type: "library-songs"},
		{ID: "2", Title: "Cape Cod Kwassa Kwassa", Subtitle: "Vampire Weekend", Album: "Vampire Weekend", Type: "library-songs"},
	}}

	// Playlist tracks drill-down: playlist name ≠ track album, so the
	// album IS shown.
	plDrill := New(&fakeClient{})
	plDrill.width, plDrill.height = 120, 16
	plDrill.tab = Playlists
	plDrill.detail = true
	plDrill.detailKind = Playlists
	plDrill.detailTitle = "My Roadtrip Mix"
	plDrill.detailList = list{loaded: true, rows: []Item{
		{ID: "1", Title: "A-Punk", Subtitle: "Vampire Weekend", Album: "Vampire Weekend", Type: "songs"},
		{ID: "2", Title: "Hello", Subtitle: "Adele", Album: "25", Type: "songs"},
	}}

	// Search mixed types.
	search := New(&fakeClient{})
	search.width, search.height = 120, 16
	search.tab = Search
	search.lists[Search] = list{loaded: true, rows: []Item{
		{ID: "1", Title: "A-Punk", Subtitle: "Vampire Weekend", Album: "Vampire Weekend", Type: "songs"},
		{ID: "2", Title: "Vampire Weekend", Subtitle: "Vampire Weekend", Album: "Vampire Weekend", Type: "albums"},
		{ID: "3", Title: "Vampire Weekend", Subtitle: "Indie rock", Album: "", Type: "artists"},
	}}

	// Unicode.
	unicodeRows := New(&fakeClient{})
	unicodeRows.width, unicodeRows.height = 120, 12
	unicodeRows.tab = Queue
	unicodeRows.lists[Queue] = list{loaded: true, rows: []Item{
		{ID: "1", Title: "日本語 タイトル", Subtitle: "Österreichischer Künstler", Album: "Die schöne Müllerin"},
		{ID: "2", Title: "歌曲标题", Subtitle: "艺术家", Album: ""},
	}}

	t.Log(mkExample("Wide 120 cols / Queue (mixed)", qWide))
	t.Log(mkExample("Standard 80 cols / Queue", qStd))
	t.Log(mkExample("Narrow 40 cols / Queue", qNar))
	t.Log(mkExample("Wide 120 cols / Library songs", libSongsView))
	t.Log(mkExample("Wide 120 cols / Library albums", libAlbumsView))
	t.Log(mkExample("Wide 120 cols / Album-tracks drill-down", albumDrill))
	t.Log(mkExample("Wide 120 cols / Playlist-tracks drill-down", plDrill))
	t.Log(mkExample("Wide 120 cols / Search (mixed types)", search))
	t.Log(mkExample("Wide 120 cols / Unicode", unicodeRows))
}
