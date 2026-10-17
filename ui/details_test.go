package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestCatalogDetailsRetainMetadata(t *testing.T) {
	var p page
	if err := json.Unmarshal([]byte(`{"data":[{"id":"1","type":"songs","attributes":{"name":"Song","artistName":"Artist","albumName":"Album","durationInMillis":185500,"genreNames":["Alternative","Rock"],"releaseDate":"2026-10-16","trackNumber":3,"discNumber":2}}]}`), &p); err != nil {
		t.Fatal(err)
	}
	item := items(p)[0]
	if item.Album != "Album" || item.Duration != 185.5 || item.Genre != "Alternative, Rock" ||
		item.ReleaseDate != "2026-10-16" || item.TrackNumber != 3 || item.DiscNumber != 2 {
		t.Fatalf("catalog details lost: %+v", item)
	}
}

func TestQueueDetailsReadDirectAndNestedMetadata(t *testing.T) {
	for _, raw := range []string{
		`[{"id":"1","attributes":{"name":"Song","albumName":"Album","durationInMillis":185500,"genreNames":["Rock"],"releaseDate":"2026-10-16","trackNumber":3,"discNumber":2}}]`,
		`[{"id":"1","item":{"attributes":{"name":"Song","albumName":"Album","durationInMillis":185500,"genreNames":["Rock"],"releaseDate":"2026-10-16","trackNumber":3,"discNumber":2}}}]`,
	} {
		rows, _, err := parseQueue(json.RawMessage(raw))
		if err != nil {
			t.Fatal(err)
		}
		item := rows[0]
		if item.Album != "Album" || item.Duration != 185.5 || item.Genre != "Rock" ||
			item.ReleaseDate != "2026-10-16" || item.TrackNumber != 3 || item.DiscNumber != 2 {
			t.Fatalf("queue details lost: %+v", item)
		}
	}
}

func TestDetailsPanelShowsRichMetadataForNonPlayingSong(t *testing.T) {
	m := New(&fakeClient{})
	m.now = NowPlaying{ID: "different", AlbumName: "Wrong album", Duration: 20}
	item := Item{ID: "selected-id", Type: "library-songs", Title: "Selected song", Subtitle: "Artist", Album: "Selected album", Duration: 185.5, Genre: "Rock", ReleaseDate: "2026-10-16T12:00:00Z", TrackNumber: 3, DiscNumber: 2}
	rows := m.itemDetailsLines(item, 50, 20)
	view := ansi.Strip(strings.Join(rows, "\n"))
	for _, want := range []string{"Details", "Selected song", "Artist", "Selected album", "3:05", "Rock", "2026-10-16", "Track", "Disc", "song", "selected-id"} {
		if !strings.Contains(view, want) {
			t.Fatalf("details missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "T12:") || strings.Contains(view, "Wrong album") || strings.Contains(view, "library-songs") {
		t.Fatalf("details contain raw or incorrect metadata:\n%s", view)
	}
	if !strings.HasPrefix(view, "╭─ Details") || !strings.HasSuffix(view, "╯") {
		t.Fatal("details border missing")
	}
	t.Log("\n" + view)
}

func TestDetailsWrapAndKeepSelectedRowVisible(t *testing.T) {
	for _, size := range [][2]int{{80, 12}, {35, 8}, {18, 5}, {8, 3}} {
		m := New(&fakeClient{})
		m.expanded = true
		m.queueLoading = true
		m.lists[Queue] = list{loaded: true, rows: []Item{{ID: "1", Title: "Selected", Subtitle: strings.Repeat("日本語 long artist ", 4), Album: "Album"}}}
		rows := m.listLines(size[0], size[1])
		view := ansi.Strip(strings.Join(rows, "\n"))
		if len(rows) != size[1] || !strings.Contains(view, "Selected") || !strings.Contains(view, "Loading") {
			t.Fatalf("size %v: selected row or loading state hidden:\n%s", size, view)
		}
		for _, row := range m.itemDetailsLines(m.lists[Queue].rows[0], size[0], size[1]) {
			if ansi.StringWidth(row) > size[0] {
				t.Fatalf("size %v: details overflow width", size)
			}
		}
	}
}
