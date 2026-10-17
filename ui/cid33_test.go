package ui

// Tests for CID-33: playlist next-track / Queue desynchronization.
//
// Bubble Tea returns a tea.Cmd from Update; that cmd may be a BatchMsg
// containing several sub-cmds. To observe model state after the cmd
// runs, we have to drive each sub-cmd individually. The drive helper
// here does that, and re-runs Update for each sub-cmd's Msg so the
// model's state at the end reflects everything that the cmd scheduled.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// step runs cmd and feeds every message it produces back through Update.
// step also processes any subsequent cmds those messages schedule (one
// level of recursion is enough for our poll/queue handlers). The final
// model is returned.
func step(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	raw := cmd()
	if batch, ok := raw.(tea.BatchMsg); ok {
		var model tea.Model = m
		for _, c := range batch {
			if c == nil {
				continue
			}
			model, _ = model.Update(c())
		}
		return model.(Model)
	}
	updated, _ := m.Update(raw)
	return updated.(Model)
}

// pollWithTrack fires one poll cycle that reports `nowID` as playing
// and processes every cmd the poll schedules.
func pollWithTrack(t *testing.T, m Model, nowID string) Model {
	t.Helper()
	updated, cmd := m.Update(pollMsg{
		now:   NowPlaying{ID: nowID, Title: "Song", Playing: true},
		volOK: true,
	})
	return step(t, updated.(Model), cmd)
}

// TestQueueReloadsOnTrackChange: a poll that reports a different
// now-playing ID must schedule a queue reload. If the model skips the
// reload, the Queue marker won't follow the song.
func TestQueueReloadsOnTrackChange(t *testing.T) {
	c := &queueReadClient{
		fakeClient: &fakeClient{},
		responses: []string{
			`[{"id":"i.1","isCurrent":true,"attributes":{"name":"First"}}]`,
			`[{"id":"i.2","isCurrent":true,"attributes":{"name":"Second"}}]`,
		},
	}
	m := New(c)
	m = pollWithTrack(t, m, "i.1")
	if c.reads != 1 {
		t.Fatalf("first poll did not reload queue: reads=%d", c.reads)
	}
	m = pollWithTrack(t, m, "i.2")
	if c.reads != 2 {
		t.Fatalf("track change did not reload queue: reads=%d", c.reads)
	}
	if m.queueCurrent != "i.2" {
		t.Fatalf("queue did not follow track change: current=%q", m.queueCurrent)
	}
}

// TestQueueMarkerFollowsPlaylistNext: when a poll advances the
// now-playing track and the queue refresh reports the new current, the
// Queue tab's play-mark must move to the new row.
func TestQueueMarkerFollowsPlaylistNext(t *testing.T) {
	c := &queueReadClient{
		fakeClient: &fakeClient{},
		responses: []string{
			`[{"id":"i.1","isCurrent":true},{"id":"i.2"},{"id":"i.3"}]`,
		},
	}
	m := New(c)
	m.tab = Queue
	m.width, m.height = 80, 24
	m = pollWithTrack(t, m, "i.1")
	if m.queueCurrent != "i.1" {
		t.Fatalf("queue did not mark i.1 current: %q", m.queueCurrent)
	}
	// User hits "next" in Cider. The model must update now-playing and
	// reload the queue, and the queue refresh must mark i.2 as current.
	c.responses = []string{
		`[{"id":"i.1"},{"id":"i.2","isCurrent":true},{"id":"i.3"}]`,
	}
	m = pollWithTrack(t, m, "i.2")
	if m.queueCurrent != "i.2" {
		t.Fatalf("queue did not follow playlist next: current=%q", m.queueCurrent)
	}
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "▸") {
		t.Fatalf("no play-mark in view after refresh:\n%s", view)
	}
}

// TestPlayingIndexRefusesDuplicateCurrentID: when the queue response
// reports the same ID twice with no way to tell which one Cider marks
// as currently playing, the model must refuse to issue a positional
// edit (remove/jump) rather than guess. This is the existing safety
// guard; pinning it so a future relaxation can't silently re-introduce
// the risk of moving the wrong row.
func TestPlayingIndexRefusesDuplicateCurrentID(t *testing.T) {
	c := &queueReadClient{
		fakeClient: &fakeClient{},
		responses: []string{
			`[{"id":"i.dup","isCurrent":true},{"id":"i.dup"}]`,
		},
	}
	m := New(c)
	m.tab = Queue
	m.width, m.height = 80, 24
	updated, cmd := m.Update(pollMsg{
		now:   NowPlaying{ID: "i.dup", Playing: true},
		volOK: true,
	})
	m = step(t, updated.(Model), cmd)
	m.lists[Queue].cursor = 1
	updated, cmd = m.Update(runeKey('x'))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("x on a duplicate-current Queue row returned no cmd")
	}
	msg := cmd().(actionMsg)
	if msg.err == nil {
		t.Fatal("x on a duplicate-current Queue row issued a real remove; must refuse")
	}
}
