package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

type queueReadClient struct {
	*fakeClient
	responses []string
	readErr   error
	reads     int
}

func (c *queueReadClient) Get(path string, out any) error {
	if path != "/api/v1/playback/queue" {
		return c.fakeClient.Get(path, out)
	}
	c.reads++
	if c.readErr != nil {
		return c.readErr
	}
	index := min(c.reads-1, len(c.responses)-1)
	return json.Unmarshal([]byte(c.responses[index]), out)
}

func TestEnqueueRetriesOnlyIncompleteQueueRead(t *testing.T) {
	c := &queueReadClient{fakeClient: &fakeClient{}, responses: []string{"[", `[{"id":"added","attributes":{"name":"Song"}}]`}}
	m := New(c)
	m.tab = Library
	m.lists[Library].rows = []Item{{ID: "added", Type: "library-songs", Title: "Song"}}
	m.status = "old JSON error"
	m, msg := sendKey(t, m, runeKey('a'))
	updated, reload := m.Update(msg)
	m = updated.(Model)
	if m.status != "" || m.notice != "Queued: Song" {
		t.Fatal("successful enqueue kept a stale error")
	}
	updated, _ = m.Update(reload())
	m = updated.(Model)
	if c.reads != 2 || len(c.calls) != 1 || c.calls[0].path != "/api/v1/playback/play-later" {
		t.Fatalf("enqueue should run once and queue read twice: reads=%d calls=%+v", c.reads, c.calls)
	}
	if m.status != "" || len(m.lists[Queue].rows) != 1 {
		t.Fatalf("queue did not recover: %q %+v", m.status, m.lists[Queue])
	}
}

func TestQueueRefreshFailurePreservesLastQueueAndIsLabeled(t *testing.T) {
	c := &queueReadClient{fakeClient: &fakeClient{}, readErr: fmt.Errorf("decode Cider response: %w", io.EOF)}
	m := New(c)
	m.lists[Queue] = list{loaded: true, rows: []Item{{ID: "keep"}}}
	updated, _ := m.Update(loadQueue(c)())
	m = updated.(Model)
	if c.reads != 2 || !strings.HasPrefix(m.status, "Queue refresh failed: ") {
		t.Fatalf("unbounded retry or ambiguous error: reads=%d status=%q", c.reads, m.status)
	}
	if len(m.lists[Queue].rows) != 1 || m.lists[Queue].rows[0].ID != "keep" {
		t.Fatal("refresh failure erased the last known queue")
	}
	updated, _ = m.Update(queueMsg{rows: []Item{{ID: "new"}}})
	if updated.(Model).status != "" {
		t.Fatal("recovered queue kept the refresh error")
	}
}

func TestQueueRefreshDoesNotRetryNonDecodeFailures(t *testing.T) {
	c := &queueReadClient{fakeClient: &fakeClient{}, readErr: errors.New("request failed")}
	if msg := loadQueue(c)().(queueMsg); msg.err == nil || c.reads != 1 {
		t.Fatalf("non-decode failure retried: reads=%d msg=%+v", c.reads, msg)
	}
	c = &queueReadClient{fakeClient: &fakeClient{}, responses: []string{`{"status":"ok"}`}}
	if msg := loadQueue(c)().(queueMsg); msg.err == nil || c.reads != 1 {
		t.Fatal("unexpected queue schema was silently accepted or retried")
	}
}

func TestQueueRefreshCoalescesInFlightReadsAndMutations(t *testing.T) {
	c := &queueReadClient{fakeClient: &fakeClient{}, responses: []string{`[{"id":"old"}]`, `[{"id":"new"}]`}}
	m := New(c)
	updated, first := m.Update(queueRefreshMsg{})
	m = updated.(Model)
	if first == nil || !m.queueLoading {
		t.Fatal("initial queue request was not tracked")
	}
	updated, duplicate := m.Update(queueRefreshMsg{})
	m = updated.(Model)
	if duplicate != nil {
		t.Fatal("overlapping queue request started")
	}
	updated, duplicate = m.Update(actionMsg{queued: true})
	m = updated.(Model)
	if duplicate != nil || !m.queueDirty {
		t.Fatal("enqueue did not defer refresh until in-flight read completes")
	}
	updated, followup := m.Update(first())
	m = updated.(Model)
	if followup == nil || !m.queueLoading || m.queueDirty {
		t.Fatal("dirty snapshot did not schedule exactly one follow-up read")
	}
	updated, followup = m.Update(followup())
	m = updated.(Model)
	if followup != nil || m.queueLoading || c.reads != 2 || m.lists[Queue].rows[0].ID != "new" {
		t.Fatal("queue refreshes did not settle on the new snapshot")
	}
}

func TestQueueRefreshBacksOffAfterFailure(t *testing.T) {
	m := New(&fakeClient{})
	updated, _ := m.Update(queueMsg{err: errors.New("request timed out")})
	m = updated.(Model)
	updated, cmd := m.Update(queueRefreshMsg{})
	m = updated.(Model)
	if cmd != nil || m.queueLoading || !time.Now().Before(m.queueRetryAt) {
		t.Fatal("failed queue read immediately retried")
	}
	updated, _ = m.Update(pollMsg{volOK: true})
	m = updated.(Model)
	if !strings.HasPrefix(m.status, "Queue refresh failed: ") || !strings.Contains(m.View(), "Queue unavailable") {
		t.Fatal("playback poll hid queue failure or presented it as an empty queue")
	}
	m.queueRetryAt = time.Now().Add(-time.Second)
	updated, cmd = m.Update(queueRefreshMsg{})
	if cmd == nil || !updated.(Model).queueLoading {
		t.Fatal("queue did not retry after backoff")
	}
}
