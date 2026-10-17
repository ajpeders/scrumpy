package ui

import (
	"strings"
	"testing"
	"time"
)

// The now-playing endpoint is sampled before is-playing. A transition may
// return idle-shaped metadata while the later play-state read confirms audio.
func TestCID37EmptyMetadataWhilePlayingKeepsTrack(t *testing.T) {
	for _, route := range []string{"toggle-shuffle", "toggle-repeat"} {
		t.Run(route, func(t *testing.T) {
			fake := &fakeClient{getBodies: map[string]string{
				"/api/v1/playback/now-playing": `{"status":"ok","info":{"shuffleMode":1,"repeatMode":1}}`,
				"/api/v1/playback/is-playing":  `{"status":"ok","is_playing":true}`,
				"/api/v1/playback/volume":      `{"status":"ok","volume":0.3}`,
			}}
			m := New(fake)
			m.now = NowPlaying{ID: "i.old", Title: "Hard to Explain", ArtistName: "The Strokes", Position: 42, Duration: 200, Volume: 0.8, Playing: true}
			m.lastTrack = m.now.ID
			m.polledAt = time.Now()
			m.lyricsID = m.now.ID
			m.lists[Queue].rows = []Item{{ID: "i.old"}, {ID: "i.next"}}
			m.queueLoaded = time.Now()

			if err := fake.Post("/api/v1/playback/"+route, nil, nil); err != nil {
				t.Fatal(err)
			}
			msg := poll(fake)().(pollMsg)
			if msg.err != nil || !msg.playingConfirmed || msg.now.Title != "" {
				t.Fatalf("expected empty metadata with confirmed playback: %+v", msg)
			}
			updated, cmd := m.Update(msg)
			m = updated.(Model)
			if m.now.ID != "i.old" || m.now.Title != "Hard to Explain" || !m.now.Playing || m.now.Volume != 0.3 || m.now.Position < 42 {
				t.Fatalf("lost known playback: %+v", m.now)
			}
			if m.now.ShuffleMode != "on" || m.now.RepeatMode != "all" {
				t.Fatalf("lost updated playback modes: %+v", m.now)
			}
			if m.lastTrack != "i.old" || m.lyricsID != "i.old" || m.queueLoading || len(m.lists[Queue].rows) != 2 {
				t.Fatal("empty snapshot invalidated track-dependent state or refreshed queue")
			}
			if cmd == nil || !strings.Contains(m.View(), "Hard to Explain") || strings.Contains(m.View(), "Nothing playing") {
				t.Fatal("transient empty response blanked playing UI or stopped poll chain")
			}

			fake.getBodies["/api/v1/playback/now-playing"] = `{"status":"ok","info":{"name":"I Wonder","playParams":{"id":"i.next"},"durationInMillis":180000}}`
			updated, _ = m.Update(poll(fake)())
			m = updated.(Model)
			if m.now.ID != "i.next" || m.now.Title != "I Wonder" || m.lastTrack != "i.next" || !m.queueLoading {
				t.Fatalf("new real track did not replace retained metadata: %+v", m.now)
			}
		})
	}
}

func TestCID37StoppedAndUncertainEmptyMetadataClearTrack(t *testing.T) {
	for _, state := range []struct {
		name, response string
	}{
		{"stopped", `{"status":"ok","is_playing":false}`},
		{"unknown", `{"status":"ok"}`},
	} {
		t.Run(state.name, func(t *testing.T) {
			fake := &fakeClient{getBodies: map[string]string{
				"/api/v1/playback/now-playing": `{"status":"ok","info":{"shuffleMode":0,"repeatMode":0}}`,
				"/api/v1/playback/is-playing":  state.response,
			}}
			m := New(fake)
			m.now = NowPlaying{ID: "old", Title: "Song", Playing: true}
			m.lastTrack = "old"
			updated, _ := m.Update(poll(fake)())
			m = updated.(Model)
			if m.now.ID != "" || m.now.Title != "" || m.lastTrack != "" || !strings.Contains(m.View(), "Nothing playing") {
				t.Fatalf("idle metadata did not clear when playback was not confirmed: %+v", m.now)
			}
		})
	}
}

func TestCID37StartupEmptyMetadataIsIdle(t *testing.T) {
	fake := &fakeClient{getBodies: map[string]string{
		"/api/v1/playback/now-playing": `{"status":"ok","info":{}}`,
		"/api/v1/playback/is-playing":  `{"status":"ok","is_playing":true}`,
	}}
	m := New(fake)
	updated, _ := m.Update(poll(fake)())
	m = updated.(Model)
	if m.now.Title != "" || !strings.Contains(m.View(), "Nothing playing") {
		t.Fatalf("startup without a known track should remain empty: %+v", m.now)
	}
}
