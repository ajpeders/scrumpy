package ui

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestStalePollFailureCannotDisconnectNewerPlayback(t *testing.T) {
	m := New(&fakeClient{})
	m.polledAt = time.Now()
	m.now = NowPlaying{ID: "playing", Playing: true}
	updated, next := m.Update(pollMsg{at: m.polledAt.Add(-time.Second), err: errors.New("old timeout")})
	m = updated.(Model)
	if m.offline || m.status != "" || m.now.ID != "playing" || next == nil {
		t.Fatal("stale failure replaced current playback or stopped regular polling")
	}
	updated, next = m.Update(pollMsg{extra: true, at: m.polledAt.Add(-time.Second), err: errors.New("old timeout")})
	if updated.(Model).offline || next != nil {
		t.Fatal("stale one-off failure disconnected playback or started another poll chain")
	}
}

func TestPlaybackRefreshesAreCoalesced(t *testing.T) {
	fake := &fakeClient{}
	m := New(fake)
	updated, first := m.Update(actionMsg{skipped: true})
	m = updated.(Model)
	if first == nil || !m.refreshLoading {
		t.Fatal("playback refresh was not tracked")
	}
	updated, duplicate := m.Update(actionMsg{skipped: true})
	m = updated.(Model)
	if duplicate != nil {
		t.Fatal("repeated playback actions started overlapping refreshes")
	}
	updated, _ = m.Update(first())
	m = updated.(Model)
	if m.refreshLoading || len(fake.calls) != 3 {
		t.Fatal("refresh did not finish with one set of playback reads")
	}
	_, next := m.Update(actionMsg{skipped: true})
	if next == nil {
		t.Fatal("completed playback refresh blocked future refreshes")
	}
}

func TestQueueBooleanIsNotMisreportedAsEmptyOrInvalidJSON(t *testing.T) {
	for _, raw := range []string{"false", "true", "null"} {
		rows, _, err := parseQueue([]byte(raw))
		if err == nil || len(rows) != 0 || !strings.Contains(err.Error(), "queue API returned no queue list") {
			t.Fatalf("%s did not identify Cider's unavailable queue response: %v", raw, err)
		}
	}
}

func TestFailedQueueRefreshRetriesEvenAwayFromQueueTab(t *testing.T) {
	c := &queueReadClient{fakeClient: &fakeClient{}, responses: []string{`[{"id":"added"}]`}}
	m := New(c)
	m.tab = Search
	m.queueLoading, m.queueDirty = true, true
	updated, timer := m.Update(queueMsg{err: errors.New("timeout")})
	m = updated.(Model)
	if timer == nil || m.queueRetryAt.IsZero() {
		t.Fatal("failed mutation refresh lost its retry away from Queue")
	}
	// Deliver the timer after its deadline without waiting five seconds.
	m.queueRetryAt = time.Now().Add(-time.Second)
	at := m.queueRetryAt
	updated, retry := m.Update(queueRetryMsg{at: at})
	m = updated.(Model)
	if retry == nil || !m.queueLoading {
		t.Fatal("deferred queue retry did not start")
	}
	updated, duplicate := m.Update(queueRetryMsg{at: at})
	m = updated.(Model)
	if duplicate != nil {
		t.Fatal("obsolete retry timer started an overlapping read")
	}
	updated, _ = m.Update(retry())
	m = updated.(Model)
	if m.status != "" || m.queueLoading || len(m.lists[Queue].rows) != 1 {
		t.Fatal("deferred queue retry did not recover the queue")
	}
}
