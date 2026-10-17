package viz

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Cider removes its stream when paused; the analyzer must stop capturing and
// keep running, not crash.
func TestStreamDisappearingStopsCapture(t *testing.T) {
	dir := t.TempDir()
	state := filepath.Join(dir, "dump.json")
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(name, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "pw-dump"), "#!/bin/sh\ncat \""+state+"\"\n")
	write(filepath.Join(dir, "pw-record"), "#!/bin/sh\nexec sleep 30\n")
	write(state, `[{"type":"PipeWire:Interface:Node","info":{"props":{"application.process.binary":"Cider","media.class":"Stream/Output/Audio","object.serial":7}}}]`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH")) // fakes shadow PipeWire
	defer func(d time.Duration) { retryInterval = d }(retryInterval)
	retryInterval = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	frames := NewAnalyzer(4).Run(ctx)
	time.Sleep(150 * time.Millisecond) // capturing the stream
	write(state, `[]`)                 // Cider paused: stream gone
	time.Sleep(300 * time.Millisecond)
	cancel()
	for range frames { // closes once the analyzer has stopped cleanly
	}
}

// A pw-record that never gets linked produces no data; the analyzer must give
// up on it and start a fresh one rather than wait forever.
func TestStalledCaptureIsRestarted(t *testing.T) {
	dir := t.TempDir()
	starts := filepath.Join(dir, "starts")
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(name, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "pw-dump"), `#!/bin/sh
echo '[{"type":"PipeWire:Interface:Node","info":{"props":{"application.process.binary":"Cider","media.class":"Stream/Output/Audio","object.serial":7}}}]'
`)
	write(filepath.Join(dir, "pw-record"), "#!/bin/sh\necho x >> \""+starts+"\"\nexec sleep 30\n")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	defer func(r, s time.Duration) { retryInterval, stallTimeout = r, s }(retryInterval, stallTimeout)
	retryInterval, stallTimeout = 50*time.Millisecond, 100*time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	frames := NewAnalyzer(4).Run(ctx)
	time.Sleep(600 * time.Millisecond)
	cancel()
	for range frames {
	}
	data, _ := os.ReadFile(starts)
	if n := len(data) / 2; n < 2 {
		t.Fatalf("pw-record started %d times, want a restart after stalling", n)
	}
}
