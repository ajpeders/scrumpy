package ui

import (
	"errors"
	"testing"
)

func stubClipboard(t *testing.T, have map[string]bool) *[]string {
	t.Helper()
	oldLook, oldWrite := lookPath, clipboardWrite
	t.Cleanup(func() { lookPath, clipboardWrite = oldLook, oldWrite })
	lookPath = func(n string) (string, error) {
		if have[n] {
			return n, nil
		}
		return "", errors.New("missing")
	}
	var got []string
	clipboardWrite = func(c clipboardTool, text string) error {
		got = append(got, c.name, text)
		return nil
	}
	return &got
}

func TestClipboardText(t *testing.T) {
	cases := []struct {
		it   Item
		want string
	}{
		{Item{Title: "T", Subtitle: "A", Album: "B"}, "T — A · B"},
		{Item{Title: "T", Subtitle: "A"}, "T — A"},
		{Item{Title: "T"}, "T"},
	}
	for _, c := range cases {
		if got := clipboardText(c.it); got != c.want {
			t.Errorf("got %q want %q", got, c.want)
		}
	}
}

func TestClipboardToolPreference(t *testing.T) {
	got := stubClipboard(t, map[string]bool{"xclip": true, "wl-copy": true, "xsel": true})
	m := New(&fakeClient{})
	m.lists[Queue].rows = []Item{{Title: "T", Subtitle: "A", Album: "B"}}
	m, msg := sendKey(t, m, runeKey('Y'))
	updated, _ := m.Update(msg)
	m = updated.(Model)
	if len(*got) != 2 || (*got)[0] != "wl-copy" || (*got)[1] != "T — A · B" {
		t.Errorf("got %v", *got)
	}
	if m.status != "Copied: T — A · B" {
		t.Errorf("status %q", m.status)
	}
}

func TestClipboardNoTool(t *testing.T) {
	got := stubClipboard(t, nil)
	m := New(&fakeClient{})
	m.lists[Queue].rows = []Item{{Title: "T"}}
	m, msg := sendKey(t, m, runeKey('Y'))
	updated, _ := m.Update(msg)
	m = updated.(Model)
	if len(*got) != 0 || m.status != "no clipboard tool found" {
		t.Errorf("got %v status %q", *got, m.status)
	}
}

func TestClipboardUsesCursorRow(t *testing.T) {
	got := stubClipboard(t, map[string]bool{"pbcopy": true})
	m := New(&fakeClient{})
	m.lists[Queue].rows = []Item{{Title: "one"}, {Title: "two"}}
	m.lists[Queue].cursor = 1
	_, _ = sendKey(t, m, runeKey('Y'))
	if len(*got) != 2 || (*got)[0] != "pbcopy" || (*got)[1] != "two" {
		t.Errorf("got %v", *got)
	}
}

// A stalled clipboard tool must not freeze the UI: Update only schedules the
// copy, and the status arrives as a message once it finishes.
func TestClipboardCopyRunsOffEventLoop(t *testing.T) {
	got := stubClipboard(t, map[string]bool{"wl-copy": true})
	m := New(&fakeClient{})
	m.lists[Queue].rows = []Item{{Title: "T"}}
	updated, cmd := m.Update(runeKey('Y'))
	if len(*got) != 0 {
		t.Fatal("clipboard tool ran inside Update")
	}
	if cmd == nil {
		t.Fatal("Y returned no command")
	}
	updated, _ = updated.(Model).Update(cmd())
	if s := updated.(Model).status; s != "Copied: T" {
		t.Fatalf("status = %q, want %q", s, "Copied: T")
	}
}
