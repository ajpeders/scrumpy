package cider

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTokenDiscovery(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("CIDER_TOKEN", "")
	path := filepath.Join(home, ".config/sh.cider.genten/spa-config.yml")
	if _, _, err := DiscoverToken(); !errors.Is(err, ErrNoCider) || !errors.Is(err, ErrNoToken) {
		t.Fatalf("missing file: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	write := func(s string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(s), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("connectivity:\n  apiTokensRequired: true\n  apiTokens:\n    - token: first-secret\n    - token: second-secret\n")
	if token, source, err := DiscoverToken(); err != nil || token != "first-secret" || source != "Cider config" {
		t.Fatalf("config: %q %q %v", token, source, err)
	}
	t.Setenv("CIDER_TOKEN", "override-secret")
	if token, source, err := DiscoverToken(); err != nil || token != "override-secret" || source != "CIDER_TOKEN" {
		t.Fatalf("override: %q %q %v", token, source, err)
	}
	t.Setenv("CIDER_TOKEN", "")
	write("connectivity: [invalid")
	if _, _, err := DiscoverToken(); !errors.Is(err, ErrNoToken) || strings.Contains(fmt.Sprint(err), "secret") {
		t.Fatalf("invalid config: %v", err)
	}
	write("connectivity:\n  apiTokensRequired: true\n  apiTokens: []\n")
	if _, _, err := DiscoverToken(); !errors.Is(err, ErrNoToken) {
		t.Fatalf("empty tokens: %v", err)
	}
	write("connectivity:\n  apiTokensRequired: false\n  apiTokens: []\n")
	if token, _, err := DiscoverToken(); err != nil || token != "" {
		t.Fatalf("tokens disabled: %q %v", token, err)
	}
}

func TestTokenDiscoveryHonoursXDGConfigHome(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("CIDER_TOKEN", "")
	path := filepath.Join(config, "sh.cider.genten", "spa-config.yml")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("connectivity:\n  apiTokens:\n    - token: xdg-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if token, _, err := DiscoverToken(); err != nil || token != "xdg-secret" {
		t.Fatalf("XDG config: %q %v", token, err)
	}
}

// TestTokenDiscoveryHonoursPlatformConfigDir pins the macOS / Windows
// code paths the way the XDG test pins Linux. The test runs on Linux
// by stubbing userConfigDir to return a fake "Library/Application
// Support" path; on macOS and Windows the same code resolves to the
// real platform directory, so the wiring (and the "sh.cider.genten/
// spa-config.yml" filename we look for) is locked in here.
func TestTokenDiscoveryHonoursPlatformConfigDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("CIDER_TOKEN", "")
	dir := t.TempDir()
	old := userConfigDir
	userConfigDir = func() (string, error) { return dir, nil }
	defer func() { userConfigDir = old }()
	path := filepath.Join(dir, "sh.cider.genten", "spa-config.yml")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("connectivity:\n  apiTokens:\n    - token: platform-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if token, _, err := DiscoverToken(); err != nil || token != "platform-secret" {
		t.Fatalf("platform config: %q %v", token, err)
	}
	// And when the resolver fails, we surface ErrNoToken rather than
	// crashing. The macOS "no home directory" and the Windows
	// SHGetKnownFolderPath failure both map to this path.
	userConfigDir = func() (string, error) { return "", errors.New("no config dir") }
	if _, _, err := DiscoverToken(); !errors.Is(err, ErrNoToken) {
		t.Fatalf("resolver error: %v", err)
	}
}

func TestAuthAndHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("apptoken") != "secret" {
			http.Error(w, `{"error":"UNAUTHORIZED_APP_TOKEN"}`, 403)
			return
		}
		if r.URL.Path == "/failure" {
			http.Error(w, "private response", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok","volume":0.82402}`)
	}))
	defer server.Close()
	good, _ := New(server.URL, "secret")
	vol, err := good.Volume()
	if err != nil || vol != 0.82402 {
		t.Fatalf("volume: %v %v", vol, err)
	}
	bad, _ := New(server.URL, "bad")
	if _, err := bad.Volume(); !errors.Is(err, ErrBadToken) || strings.Contains(fmt.Sprint(err), "bad") {
		t.Fatalf("403: %v", err)
	}
	if err := good.Get("/failure", nil); err == nil || strings.Contains(err.Error(), "private") {
		t.Fatalf("HTTP error: %v", err)
	}
	if err := good.Get("http://evil.example/", nil); err == nil {
		t.Fatal("absolute path accepted")
	}
}

func TestNotRunning(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	client, _ := New("http://"+address, "token")
	if _, err := client.Volume(); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("connection refused: %v", err)
	}
}

func TestPaging(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/amapi/run-v3" || r.Method != http.MethodPost {
			t.Errorf("route: %s %s", r.Method, r.URL.Path)
		}
		var body struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		paths = append(paths, body.Path)
		switch body.Path {
		case "/v1/me/library/songs?limit=1":
			fmt.Fprint(w, `{"data":{"data":[{"id":"a"}],"meta":{"total":3},"next":"/v1/me/library/songs?offset=1"}}`)
		case "/v1/me/library/songs?offset=1":
			fmt.Fprint(w, `{"data":{"data":[{"id":"b"}],"meta":{"total":3},"next":"/v1/me/library/songs?offset=2"}}`)
		case "/v1/me/library/songs?offset=2":
			fmt.Fprint(w, `{"data":{"data":[{"id":"c"}],"meta":{"total":3}}}`)
		default:
			t.Errorf("unexpected path %s", body.Path)
		}
	}))
	defer server.Close()
	client, _ := New(server.URL, "token")
	it := client.Pages("/v1/me/library/songs?limit=1")
	var ids []string
	for it.Next() {
		var data []struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(it.Page().Data, &data); err != nil {
			t.Fatal(err)
		}
		if it.Page().Meta.Total != 3 {
			t.Fatal("missing count")
		}
		ids = append(ids, data[0].ID)
	}
	if it.Err() != nil || !reflect.DeepEqual(ids, []string{"a", "b", "c"}) || len(paths) != 3 {
		t.Fatalf("pages: %v %v %v", ids, paths, it.Err())
	}
}

func TestNowPlayingNestedInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("apptoken") != "secret" {
			http.Error(w, `{"error":"UNAUTHORIZED_APP_TOKEN"}`, 403)
			return
		}
		fmt.Fprint(w, `{"status":"ok","info":{"name":"A-Punk","artistName":"Vampire Weekend","albumName":"A-Punk - Single of the Week","currentPlaybackTime":30.5,"remainingTime":107.2,"durationInMillis":137760,"shuffleMode":1,"repeatMode":0,"playParams":{"id":"i.06Qzam2fXKB2KK"}}}`)
	}))
	defer server.Close()
	client, _ := New(server.URL, "secret")
	now, err := client.NowPlaying()
	if err != nil {
		t.Fatal(err)
	}
	if now.Title() != "A-Punk" || now.Info.ArtistName != "Vampire Weekend" ||
		now.Info.CurrentPlaybackTime != 30.5 || now.Info.DurationInMillis != 137760 ||
		now.Info.ShuffleMode != 1 || now.Info.PlayParams.ID != "i.06Qzam2fXKB2KK" {
		t.Fatalf("now playing = %+v", now)
	}
}

func TestLive(t *testing.T) {
	if os.Getenv("CIDER_LIVE") != "1" {
		t.Skip("set CIDER_LIVE=1 for read-only live smoke test")
	}
	client, _, err := NewFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Volume(); err != nil {
		t.Fatal(err)
	}
	if _, err := client.NowPlaying(); err != nil {
		t.Fatal(err)
	}
	if _, err := client.LibraryCount(); err != nil {
		t.Fatal(err)
	}
}

func TestRedirectDoesNotForwardToken(t *testing.T) {
	leaked := false
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked = r.Header.Get("apptoken") != ""
		w.Write([]byte(`{}`))
	}))
	defer other.Close()
	cider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/steal", http.StatusFound)
	}))
	defer cider.Close()
	client, err := New(cider.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	var status *HTTPError
	if err := client.Get("/api/v1/playback/volume", &struct{}{}); !errors.As(err, &status) || status.StatusCode != http.StatusFound {
		t.Fatalf("redirect error = %v, want HTTP 302", err)
	}
	if leaked {
		t.Fatal("apptoken forwarded to redirect target")
	}
}
