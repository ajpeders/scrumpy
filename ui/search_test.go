package ui

import (
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSearchCyclesKindsAndKeepsQuery(t *testing.T) {
	fake := &fakeClient{amapi: func(path string, out any) error {
		if path == "/v1/me/storefront" {
			return json.Unmarshal([]byte(`{"data":[{"id":"us"}]}`), out)
		}
		return json.Unmarshal([]byte(`{"results":{"songs":{"data":[{"id":"song","attributes":{"name":"Song"}}]},"albums":{"data":[{"id":"album","attributes":{"name":"Album"}}]},"artists":{"data":[{"id":"artist","attributes":{"name":"Artist"}}]}}}`), out)
	}}
	m := New(fake)
	m.tab, m.input, m.query = Search, true, "test & term"
	m, msg := sendKey(t, m, specialKey(tea.KeyEnter))
	for i, kind := range []string{"songs", "albums", "artists", "songs"} {
		if i > 0 {
			m, msg = sendKey(t, m, runeKey('l'))
		}
		if !strings.Contains(fake.last().path, "types="+kind+"&") || !strings.Contains(fake.last().path, "term=test+%26+term") {
			t.Fatalf("wrong search path: %s", fake.last().path)
		}
		updated, _ := m.Update(msg)
		m = updated.(Model)
		rows := m.lists[Search].rows
		if len(rows) != 1 || rows[0].Type != kind || m.searchTerm != "test & term" {
			t.Fatalf("%s search rows: %+v", kind, rows)
		}
		if !strings.Contains(m.tabLine(), libraryKindNames[m.searchKind]) {
			t.Fatal("search mode is not visible")
		}
	}
}

func TestSearchKindCanChangeBeforeQueryAndLStillTypes(t *testing.T) {
	fake := &fakeClient{}
	m := New(fake)
	m.tab = Search
	m, msg := sendKey(t, m, runeKey('l'))
	if m.searchKind != libAlbums || msg != nil || len(fake.calls) != 0 {
		t.Fatal("switching an empty search should not fetch results")
	}
	m, _ = sendKey(t, m, runeKey('/'))
	m, _ = sendKey(t, m, runeKey('l'))
	if m.query != "l" || m.searchKind != libAlbums {
		t.Fatal("l switched modes instead of typing into search")
	}
}

func TestSearchKindSwitchRejectsOldResults(t *testing.T) {
	m := New(&fakeClient{})
	m.tab = Search
	m.lists[Search] = m.fresh()
	oldGen := m.lists[Search].gen
	m, _ = sendKey(t, m, runeKey('l'))
	updated, _ := m.Update(loadedMsg{tab: Search, gen: oldGen, page: page{Data: []appleItem{{ID: "stale", Type: "songs"}}}})
	m = updated.(Model)
	if len(m.lists[Search].rows) != 0 {
		t.Fatal("previous mode's results replaced the selected mode")
	}
}
