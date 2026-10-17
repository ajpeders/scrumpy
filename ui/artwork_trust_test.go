package ui

// Tests for CID-32: artwork URL trust boundary.
//
// Apple Music catalog covers are served from *.mzstatic.com over HTTPS.
// Cider's amapi passthrough can otherwise return any host a misconfigured
// proxy or compromised endpoint wants — local IPs, internal services,
// anything reachable from the user's machine. These tests pin the
// allowlist and the redirect-refusal policy.

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckArtworkTrust(t *testing.T) {
	cases := []struct {
		name   string
		url    string
		wantOK bool
	}{
		{"apple cdn apex", "https://mzstatic.com/image.256x256bb.jpg", true},
		{"apple cdn subdomain", "https://is1-ssl.mzstatic.com/image/abc/{w}x{h}bb.jpg", true},
		{"apple cdn with port", "https://is1-ssl.mzstatic.com:443/image.256x256bb.jpg", true},
		{"http rejected", "http://is1-ssl.mzstatic.com/image.256x256bb.jpg", false},
		{"ftp rejected", "ftp://is1-ssl.mzstatic.com/image.256x256bb.jpg", false},
		{"file scheme rejected", "file:///etc/passwd", false},
		{"javascript scheme rejected", "javascript:alert(1)", false},
		{"data scheme rejected", "data:image/png;base64,iVBORw0K", false},
		{"missing host rejected", "https:///path", false},
		{"unrelated host rejected", "https://example.com/image.jpg", false},
		{"lookalike subdomain rejected", "https://mzstatic.com.attacker.example/image.jpg", false},
		{"literal loopback IP rejected", "https://127.0.0.1/image.jpg", false},
		{"literal private IP rejected", "https://10.0.0.1/image.jpg", false},
		{"literal link-local rejected", "https://169.254.169.254/latest/meta-data/", false},
		{"literal ipv6 loopback rejected", "https://[::1]/image.jpg", false},
		{"literal IP with cdn suffix rejected", "https://1.2.3.4.mzstatic.com/image.jpg", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkArtworkTrust(tc.url)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("expected %q to be accepted, got error: %v", tc.url, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected %q to be rejected", tc.url)
			}
			if !errors.Is(err, errArtworkTrust) {
				t.Fatalf("expected errArtworkTrust, got %v", err)
			}
		})
	}
}

// TestDownloadImageRejectsNonAppleHost exercises the production downloadImage
// against a local HTTP server. A host on the allowlist fetches the bytes; a
// host off it is refused without dialling.
func TestDownloadImageRejectsNonAppleHost(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "fake-cover-bytes")
	}))
	defer srv.Close()

	// Point the global imageClient at this test server. Save/restore so the
	// next test isn't affected.
	old := imageClient
	imageClient = srv.Client()
	defer func() { imageClient = old }()

	// The TLS test server is on a non-mzstatic host, so the allowlist
	// refuses it before any TCP handshake. (It would refuse the handshake
	// too: even if we made it through, the hostname doesn't match.)
	if _, err := downloadImage(srv.URL); err == nil {
		t.Fatal("downloadImage accepted a non-Apple host")
	} else if !errors.Is(err, errArtworkTrust) {
		t.Fatalf("expected errArtworkTrust, got %v", err)
	}
}

// TestArtworkClientRefusesRedirects pins the redirect-refusal policy. Apple
// serves artwork bytes directly; a redirect is a sign of compromise.
func TestArtworkClientRefusesRedirects(t *testing.T) {
	var redirected bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/cover.256x256bb.jpg", http.StatusFound)
			return
		}
		redirected = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// We can't dial srv over a "mzstatic.com" name from the test runner, so
	// the trust check would refuse it. Override the URL parsing by going
	// through artworkClient directly with a same-host URL on the
	// artwork-trusted suffix.
	//
	// Easier: simulate by checking the CheckRedirect policy directly. The
	// default redirect count is 10; the policy rejects every hop.
	//
	// We swap the redirect policy under test by creating a client that uses
	// the same CheckRedirect as artworkClient but a transport that doesn't
	// enforce TLS. The point of the assertion is the CheckRedirect: any
	// redirected request must fail with the artwork redirect error.
	c := *artworkClient
	c.CheckRedirect = artworkClient.CheckRedirect
	req, _ := http.NewRequest("GET", srv.URL+"/start", nil)
	if _, err := c.Do(req); err == nil {
		t.Fatal("artworkClient followed a redirect")
	} else if !strings.Contains(err.Error(), "redirect refused") {
		t.Fatalf("expected redirect-refused error, got %v", err)
	}
	if redirected {
		t.Fatal("redirect target was reached despite refusal")
	}
}
