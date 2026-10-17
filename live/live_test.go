package live

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// These are synthetic protocol fixtures, NOT recorded Cider event names.
func TestPollingSession(t *testing.T) {
	var mu sync.Mutex
	var requests []string
	reads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path != "/socket.io/" || r.URL.Query().Get("EIO") != "4" || r.URL.Query().Get("transport") != "polling" || r.Header.Get("apptoken") != "secret" {
			t.Errorf("bad request: %s %s", r.Method, r.URL)
			w.WriteHeader(400)
			return
		}
		body := ""
		if r.Method == http.MethodPost {
			buf := make([]byte, 10)
			n, _ := r.Body.Read(buf)
			body = string(buf[:n])
		}
		requests = append(requests, r.Method+":"+r.URL.Query().Get("sid")+":"+body)
		if r.URL.Query().Get("sid") == "" {
			w.Write([]byte(`0{"sid":"session-1","upgrades":[],"pingInterval":25000,"pingTimeout":20000}`))
			return
		}
		if r.Method == http.MethodPost {
			w.Write([]byte("ok"))
			return
		}
		reads++
		if reads == 1 {
			w.Write([]byte("40\x1e2\x1e42[\"synthetic-change\",{\"position\":7}]\x1e42/admin,[\"ignored\"]"))
		} else {
			w.Write([]byte("41"))
		}
	}))
	defer server.Close()
	c, err := New(server.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	signals := make(chan Signal, 8)
	err = c.session(ctx, signals)
	if err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("session error: %v", err)
	}
	connected, event := <-signals, <-signals
	if connected.Kind != Connected || !connected.PollRequired {
		t.Fatalf("connected = %+v", connected)
	}
	if event.Kind != Event || event.Name != "synthetic-change" || string(event.Payload) != `["synthetic-change",{"position":7}]` || !event.PollRequired {
		t.Fatalf("event = %+v", event)
	}
	select {
	case extra := <-signals:
		t.Fatalf("unexpected signal: %+v", extra)
	default:
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{"GET::", "POST:session-1:40", "GET:session-1:", "POST:session-1:3", "GET:session-1:"}
	if strings.Join(requests, "|") != strings.Join(want, "|") {
		t.Fatalf("requests = %v, want %v", requests, want)
	}
}

func TestForbiddenSignalsFallbackAndStops(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	c, _ := New(server.URL, "invalid")
	signals := make(chan Signal, 2)
	c.Run(context.Background(), signals)
	s, ok := <-signals
	if !ok || s.Kind != Disconnected || !s.PollRequired || requests != 1 {
		t.Fatalf("signal = %+v, requests = %d", s, requests)
	}
	var status *StatusError
	if !errors.As(s.Err, &status) || status.Code != 403 {
		t.Fatalf("error = %v", s.Err)
	}
	if _, ok := <-signals; ok {
		t.Fatal("channel should close")
	}
}

func TestParseEventRejectsUnknownPacketTypes(t *testing.T) {
	for _, packet := range []string{`42/admin,["change"]`, `42["change"`, `42[]`, `42[5]`, `451-["change"]`, `42["",1]`} {
		if _, ok := parseEvent(packet); ok {
			t.Errorf("accepted %q", packet)
		}
	}
}

func TestInvalidHandshakeAndURL(t *testing.T) {
	for _, s := range []string{`40`, `0{}`, `0{"sid":"x","pingInterval":0,"pingTimeout":2000}`, `0not-json`} {
		if _, err := parseHandshake(s); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
	for _, s := range []string{"http://user:pass@localhost:10767", "http://localhost/?token=x", "file:///tmp/cider"} {
		if _, err := New(s, "secret"); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
}

func TestCancelledRunClosesChannel(t *testing.T) {
	c, _ := New("http://127.0.0.1:1", "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	signals := make(chan Signal)
	c.Run(ctx, signals)
	if _, ok := <-signals; ok {
		t.Fatal("expected closed channel")
	}
}
