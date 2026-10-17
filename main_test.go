package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("apptoken") != "test-token" {
			t.Error("missing token")
		}
		switch r.URL.Path {
		case "/api/v1/playback/volume":
			fmt.Fprint(w, `{"status":"ok","volume":0.5}`)
		case "/api/v1/playback/now-playing":
			fmt.Fprint(w, `{"status":"ok"}`)
		case "/api/v1/amapi/run-v3":
			fmt.Fprint(w, `{"data":{"meta":{"total":13274}}}`)
		default:
			t.Errorf("unexpected route %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("CIDER_URL", server.URL)
	t.Setenv("CIDER_TOKEN", "test-token")
	t.Setenv("PATH", t.TempDir()) // no PipeWire tools
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = old; reader.Close() }()
	if code := run([]string{"--check"}); code != 0 {
		t.Fatalf("exit code %d", code)
	}
	writer.Close()
	buf := make([]byte, 1024)
	n, err := reader.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	output := string(buf[:n])
	for _, want := range []string{"Cider: reachable", "Volume: 50%", "Now playing: idle", "Library songs: 13274", "Token source: CIDER_TOKEN", "Visualizer: unavailable (pw-record not found"} {
		if !strings.Contains(output, want) {
			t.Errorf("missing %q from output", want)
		}
	}
	if strings.Contains(output, "test-token") {
		t.Fatal("token leaked")
	}
}

func TestHelpDocumentsFlagsAndEnvironment(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = writer
	defer func() { os.Stderr = old; reader.Close() }()
	code := run([]string{"-h"})
	writer.Close()
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	buf := make([]byte, 4096)
	n, _ := reader.Read(buf)
	for _, want := range []string{"--check", "--version", "CIDER_TOKEN", "CIDER_URL"} {
		if !strings.Contains(string(buf[:n]), want) {
			t.Errorf("help missing %q", want)
		}
	}
}

func TestVersionFlag(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = old; reader.Close() }()
	version = "v9.9.9"
	defer func() { version = "" }()
	code := run([]string{"--version"})
	writer.Close()
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	buf := make([]byte, 256)
	n, _ := reader.Read(buf)
	if got := string(buf[:n]); got != "sidecue v9.9.9\n" {
		t.Fatalf("got %q", got)
	}
}
