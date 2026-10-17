package cider

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRequestsReuseConnectionsAfterAcknowledgementsAndJSON(t *testing.T) {
	var connections atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/failure" {
			w.WriteHeader(http.StatusInternalServerError)
		}
		fmt.Fprint(w, `{"status":"ok"}`+strings.Repeat(" ", 4096))
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	server.Start()
	defer server.Close()
	c, err := New(server.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := c.Post("/command", map[string]any{}, nil); err != nil {
			t.Fatal(err)
		}
		var result struct{ Status string }
		if err := c.Get("/state", &result); err != nil || result.Status != "ok" {
			t.Fatalf("state: %+v %v", result, err)
		}
	}
	if err := c.Get("/failure", nil); err == nil {
		t.Fatal("expected HTTP failure")
	}
	if err := c.Get("/state", nil); err != nil {
		t.Fatal(err)
	}
	if got := connections.Load(); got != 1 {
		t.Fatalf("serial requests used %d TCP connections, want one", got)
	}
}
