package main

// Tests for the release-publish helper used by .forgejo/workflows/release.yml.
// We exercise publish-release.sh end to end against a mock Forgejo API. The
// test asserts which routes the script hit (lookup vs create vs upload) and
// which asset names it uploaded, so a regression to the lookup-or-create
// decision or to the existing-asset skip would fail.

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestGitHubCIDoesNotPersistCheckoutCredentials(t *testing.T) {
	workflow, err := os.ReadFile(".github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(workflow), "- uses: actions/checkout@v4\n        with:\n          persist-credentials: false") {
		t.Fatal("GitHub CI must disable checkout credential persistence before running PR code")
	}
}

// mockReleaseAPI records which routes the script hit, so tests can assert
// on the lookup-vs-create-vs-upload sequence rather than trusting exit
// codes alone.
type mockReleaseAPI struct {
	mu             sync.Mutex
	lookupHits     int
	createHits     int
	uploadHits     []string // basenames of uploaded assets, in order
	existingAssets []string // basenames the release already has; /assets returns these
	lookupBody     string   // JSON returned by /releases/tags/<tag> when lookupStatus==200
	lookupStatus   int      // HTTP status for the lookup; 0 = use 404
}

func (m *mockReleaseAPI) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()
		switch {
		case strings.Contains(r.URL.Path, "/releases/tags/"):
			m.lookupHits++
			status := m.lookupStatus
			if status == 0 {
				status = http.StatusNotFound
			}
			w.WriteHeader(status)
			fmt.Fprint(w, m.lookupBody)
		case strings.HasSuffix(r.URL.Path, "/assets") && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			if len(m.existingAssets) == 0 {
				fmt.Fprint(w, `[]`)
				return
			}
			fmt.Fprint(w, `[`)
			for i, name := range m.existingAssets {
				if i > 0 {
					fmt.Fprint(w, ",")
				}
				fmt.Fprintf(w, `{"name":%q}`, name)
			}
			fmt.Fprint(w, `]`)
		case strings.HasSuffix(r.URL.Path, "/assets") && r.Method == http.MethodPost:
			name := r.URL.Query().Get("name")
			m.uploadHits = append(m.uploadHits, name)
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{}`)
		case r.URL.Path == "/api/v1/repos/alex/scrumpy/releases" && r.Method == http.MethodPost:
			m.createHits++
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, m.lookupBody)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	})
}

func TestPublishReleaseCreatesWhenMissing(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantID    string
		wantError bool
	}{
		{
			name: "simple release body",
			body: `{
				"id": 42,
				"tag_name": "v1.0.0",
				"author": {"id": 99, "login": "alice"}
			}`,
			wantID: "42",
		},
		{
			// The exact case the greedy sed regex got wrong: the inner
			// author.id is the *last* numeric id in the body, so a greedy
			// `.*"id":\([0-9]*\).*` would have matched 99 instead of 17.
			name: "nested author id",
			body: `{
				"id": 17,
				"author": {"id": 999, "login": "alice"},
				"assets": []
			}`,
			wantID: "17",
		},
		{
			name:      "missing id field",
			body:      `{"tag_name": "v1.0.0"}`,
			wantError: true,
		},
		{
			name:      "id is null",
			body:      `{"id": null, "tag_name": "v1.0.0"}`,
			wantError: true,
		},
		{
			name:      "id is not an integer",
			body:      `{"id": "not-an-int", "tag_name": "v1.0.0"}`,
			wantError: true,
		},
	}

	script, err := filepath.Abs(filepath.Join("scripts", "publish-release.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("publish-release.sh missing (%v): the script is part of the release pipeline and must be committed", err)
	}
	jq, err := exec.LookPath("jq")
	if err != nil {
		t.Fatalf("jq not installed (%v): the release script depends on jq; CI installs it explicitly", err)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockReleaseAPI{lookupBody: tc.body}
			server := httptest.NewServer(mock.handler(t))
			defer server.Close()

			dir := t.TempDir()
			amd := filepath.Join(dir, "scrumpy-linux-amd64")
			arm := filepath.Join(dir, "scrumpy-linux-arm64")
			for _, p := range []string{amd, arm} {
				if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			cmd := exec.Command("bash", script,
				server.URL+"/api/v1/repos/alex/scrumpy/releases",
				"test-token",
				"v1.0.0",
				amd, arm,
			)
			cmd.Env = append(os.Environ(), "JQ_BIN="+jq)
			out, err := cmd.CombinedOutput()

			mock.mu.Lock()
			defer mock.mu.Unlock()

			if tc.wantError {
				if err == nil {
					t.Fatalf("expected error, got output: %s", out)
				}
				return
			}
			if err != nil {
				t.Fatalf("script failed: %v\noutput: %s", err, out)
			}
			if mock.lookupHits != 1 {
				// 404 path: one probe, then create. (200 path probes
				// again to fetch the body — see TestPublishReleaseReusesExisting.)
				t.Fatalf("expected 1 lookup hit on the 404 path, got %d (output: %s)", mock.lookupHits, out)
			}
			if mock.createHits != 1 {
				t.Fatalf("expected 1 create, got %d (output: %s)", mock.createHits, out)
			}
			want := []string{"scrumpy-linux-amd64", "scrumpy-linux-arm64"}
			if len(mock.uploadHits) != len(want) {
				t.Fatalf("uploaded %v, want %v (output: %s)", mock.uploadHits, want, out)
			}
			for i, name := range want {
				if mock.uploadHits[i] != name {
					t.Fatalf("upload %d = %q, want %q (output: %s)", i, mock.uploadHits[i], name, out)
				}
			}
		})
	}
}

// TestPublishReleaseReusesExisting: when the release already exists and
// some assets are uploaded, a retry must NOT recreate the release and
// must skip the existing assets. The script compares by basename, since
// the workspace passes full paths but Forgejo stores just the filename.
func TestPublishReleaseReusesExisting(t *testing.T) {
	script, _ := filepath.Abs(filepath.Join("scripts", "publish-release.sh"))
	jq, _ := exec.LookPath("jq")

	mock := &mockReleaseAPI{
		lookupStatus:   http.StatusOK,
		lookupBody:     `{"id": 7, "tag_name": "v1.0.0", "assets": []}`,
		existingAssets: []string{"scrumpy-linux-amd64"},
	}
	server := httptest.NewServer(mock.handler(t))
	defer server.Close()

	dir := t.TempDir()
	amd := filepath.Join(dir, "scrumpy-linux-amd64")
	arm := filepath.Join(dir, "scrumpy-linux-arm64")
	sums := filepath.Join(dir, "SHA256SUMS")
	for _, p := range []string{amd, arm, sums} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command("bash", script,
		server.URL+"/api/v1/repos/alex/scrumpy/releases",
		"test-token",
		"v1.0.0",
		amd, arm, sums,
	)
	cmd.Env = append(os.Environ(), "JQ_BIN="+jq)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("script failed: %v\noutput: %s", err, out)
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if mock.createHits != 0 {
		t.Fatalf("retry created a duplicate release (got %d POSTs to /releases)", mock.createHits)
	}
	if mock.lookupHits < 2 {
		t.Fatalf("expected >=2 lookup hits, got %d", mock.lookupHits)
	}
	want := []string{"scrumpy-linux-arm64", "SHA256SUMS"}
	if len(mock.uploadHits) != len(want) {
		t.Fatalf("uploaded %v, want %v (output: %s)", mock.uploadHits, want, out)
	}
	for i, name := range want {
		if mock.uploadHits[i] != name {
			t.Fatalf("upload %d = %q, want %q (output: %s)", i, mock.uploadHits[i], name, out)
		}
	}
}

// TestPublishReleasePropagatesAuthError: a 401 on the lookup must fail
// loudly (not silently create a duplicate release). Cider's token may
// have been revoked; we want the job to error visibly.
func TestPublishReleasePropagatesAuthError(t *testing.T) {
	script, _ := filepath.Abs(filepath.Join("scripts", "publish-release.sh"))
	jq, _ := exec.LookPath("jq")

	mock := &mockReleaseAPI{lookupStatus: http.StatusUnauthorized}
	server := httptest.NewServer(mock.handler(t))
	defer server.Close()

	cmd := exec.Command("bash", script,
		server.URL+"/api/v1/repos/alex/scrumpy/releases",
		"test-token",
		"v1.0.0",
		"/tmp/nonexistent",
	)
	cmd.Env = append(os.Environ(), "JQ_BIN="+jq)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected error on 401, got output: %s", out)
	}
	if mock.createHits != 0 {
		t.Fatalf("script created a release on auth failure (got %d POSTs)", mock.createHits)
	}
	if !strings.Contains(string(out), "401") {
		t.Fatalf("error did not mention HTTP 401: %s", out)
	}
}
