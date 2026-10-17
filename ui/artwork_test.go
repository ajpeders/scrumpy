package ui

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderArtworkQuadrants(t *testing.T) {
	art := image.NewRGBA(image.Rect(0, 0, artworkWide, artworkHigh))
	for y := range artworkHigh {
		for x := range artworkWide {
			art.SetRGBA(x, y, color.RGBA{uint8(255 * x / artworkWide), uint8(255 * y / artworkHigh), 0, 255})
		}
	}
	lines := renderArtwork(art)
	if len(lines) != artworkRows {
		t.Fatalf("rendered %d rows, want %d", len(lines), artworkRows)
	}
	for _, line := range lines {
		if !strings.Contains(line, "\x1b[38;2;") || !strings.Contains(line, "\x1b[48;2;") {
			t.Fatalf("row missing RGB colours: %q", line)
		}
		if ansi.Strip(line) == "" {
			t.Fatal("empty row")
		}
	}
}

func TestQuadBlockSelectsGlyphAndColours(t *testing.T) {
	red := color.RGBA{255, 0, 0, 255}
	blue := color.RGBA{0, 0, 255, 255}
	// Top row red, bottom row blue → upper-half block, red lit blue background.
	got := quadBlock(red, red, blue, blue)
	if !strings.Contains(got, "▀") {
		t.Fatalf("top/bottom split = %q, want ▀", got)
	}
	if !strings.Contains(got, "\x1b[38;2;255;0;0m") || !strings.Contains(got, "\x1b[48;2;0;0;255m") {
		t.Fatalf("colours = %q", got)
	}
	// Solid colour → full block.
	if solid := quadBlock(red, red, red, red); !strings.Contains(solid, "█") {
		t.Fatalf("solid = %q, want █", solid)
	}
	// Left column red, right column blue → left-half block.
	if half := quadBlock(red, blue, red, blue); !strings.Contains(half, "▌") {
		t.Fatalf("left/right split = %q, want ▌", half)
	}
}

func TestDownsampleAveragesToArtworkSize(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			src.SetRGBA(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	out := downsampleTo(src, artworkWide, artworkHigh)
	if out == nil || out.Bounds().Dx() != artworkWide || out.Bounds().Dy() != artworkHigh {
		t.Fatalf("downsample bounds = %v", out.Bounds())
	}
	for y := range artworkHigh {
		for x := range artworkWide {
			if c := out.RGBAAt(x, y); c.R != 255 || c.G != 0 || c.B != 0 {
				t.Fatalf("pixel %d,%d = %v, want solid red", x, y, c)
			}
		}
	}
	if downsampleTo(image.NewRGBA(image.Rect(0, 0, 0, 0)), 4, 4) != nil {
		t.Fatal("empty source must downsample to nil")
	}
}

func TestArtworkURLRewritesSize(t *testing.T) {
	fake := &fakeClient{amapi: func(path string, out any) error {
		switch {
		case path == "/v1/me/storefront":
			return json.Unmarshal([]byte(`{"data":[{"id":"gb"}]}`), out)
		case path == "/v1/catalog/gb/songs/123":
			return json.Unmarshal([]byte(`{"data":[{"attributes":{"artwork":{"url":"https://cdn/{w}x{h}bb.jpg"}}}]}`), out)
		}
		t.Errorf("unexpected AMAPI %q", path)
		return nil
	}}
	u, err := artworkURL(fake, "123", "songs")
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://cdn/256x256bb.jpg"; u != want {
		t.Fatalf("artwork URL = %q, want %q", u, want)
	}
}

func TestArtworkURLRejectsSongWithoutArtwork(t *testing.T) {
	fake := &fakeClient{amapi: func(path string, out any) error {
		if path == "/v1/me/storefront" {
			return json.Unmarshal([]byte(`{"data":[{"id":"us"}]}`), out)
		}
		return json.Unmarshal([]byte(`{"data":[]}`), out)
	}}
	if _, err := artworkURL(fake, "9", "songs"); err == nil {
		t.Fatal("expected no-artwork error")
	}
}

func TestArtworkCacheRoundTrip(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 32, 32))); err != nil {
		t.Fatal(err)
	}
	if err := saveArtwork("123", buf.Bytes()); err != nil {
		t.Fatal(err)
	}
	got := cachedArtwork("123")
	if got == nil || got.block == nil || got.block.Bounds().Dx() != artworkWide {
		t.Fatalf("cached artwork = %v", got)
	}
	// A different track's cache path must stay within the artwork directory.
	path, err := artworkCachePath("../../etc")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(path, "artwork") || strings.Contains(path, "..") {
		t.Fatalf("unsafe cache path %q", path)
	}
}

func TestArtworkCacheMigratesPreviousArtwork(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 32, 32))); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(dir, "sidecue", "artwork", "123.img")
	if err := os.MkdirAll(filepath.Dir(legacy), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if cachedArtwork("123") == nil {
		t.Fatal("previous artwork was not loaded")
	}
	path, err := artworkCachePath("123")
	if err != nil {
		t.Fatal(err)
	}
	if migrated, err := os.ReadFile(path); err != nil || !bytes.Equal(migrated, buf.Bytes()) {
		t.Fatalf("migrated artwork = %d bytes, %v", len(migrated), err)
	}
}

func TestPruneArtworkCacheRemovesOldestFirst(t *testing.T) {
	dir := t.TempDir()
	for i, name := range []string{"a.img", "b.img", "c.img"} {
		data := bytes.Repeat([]byte{byte(i)}, 200)
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
		// Stagger mtimes so the oldest is deterministic.
		os.Chtimes(filepath.Join(dir, name), time.Unix(0, 0), time.Unix(0, int64(i)))
	}
	// Limit to two files: the oldest (a.img) must be removed.
	pruneArtworkCache(dir, 400)
	remaining, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 2 {
		t.Fatalf("pruned to %d files, want 2", len(remaining))
	}
	for _, e := range remaining {
		if e.Name() == "a.img" {
			t.Fatal("oldest file was not removed")
		}
	}
}

func TestArtworkFetchesAndRendersOnTrack(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	orig := downloadImage
	defer func() { downloadImage = orig }()
	downloadImage = func(u string) ([]byte, error) {
		if u != "https://cdn/256x256bb.jpg" {
			t.Fatalf("downloaded %q", u)
		}
		var buf bytes.Buffer
		img := image.NewRGBA(image.Rect(0, 0, 32, 32))
		for y := range 32 {
			for x := range 32 {
				img.SetRGBA(x, y, color.RGBA{200, 10, 30, 255})
			}
		}
		if err := png.Encode(&buf, img); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes(), nil
	}
	fake := &fakeClient{amapi: func(path string, out any) error {
		switch {
		case path == "/v1/me/storefront":
			return json.Unmarshal([]byte(`{"data":[{"id":"us"}]}`), out)
		case path == "/v1/catalog/us/songs/123":
			return json.Unmarshal([]byte(`{"data":[{"attributes":{"artwork":{"url":"https://cdn/{w}x{h}bb.jpg"}}}]}`), out)
		}
		t.Errorf("unexpected AMAPI %q", path)
		return nil
	}}
	m := New(fake)
	m.artwork = true
	m.width = 80
	m.height = 24
	m.now = NowPlaying{ID: "i.1", CatalogID: "123", Title: "Song", ArtistName: "Artist", AlbumName: "Album"}
	origMode := artworkMode
	artworkMode = artBlock
	defer func() { artworkMode = origMode }()

	cmd := m.ensureArtwork()
	if cmd == nil {
		t.Fatal("no artwork command issued")
	}
	updated, _ := m.Update(cmd())
	m = updated.(Model)
	if m.coverLines == nil {
		t.Fatal("artwork did not load")
	}
	if !strings.Contains(m.View(), "█") {
		t.Fatalf("artwork not rendered:\n%s", m.View())
	}
	// A second ensure for the same track does not re-download.
	downloadImage = func(string) ([]byte, error) { t.Fatal("re-downloaded"); return nil, nil }
	if m.ensureArtwork() != nil {
		t.Fatal("ensureArtwork refetched an already-loaded cover")
	}
}

func TestKittImageRenderUsesGraphicsProtocol(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 64, 64))); err != nil {
		t.Fatal(err)
	}
	cov := buildArtwork(image.NewRGBA(image.Rect(0, 0, 64, 64)))
	if cov.png == nil {
		t.Fatal("no PNG produced")
	}
	lines := renderCover(cov, artKitty)
	if len(lines) != artworkRows {
		t.Fatalf("kitty render gave %d rows, want %d", len(lines), artworkRows)
	}
	top := lines[0]
	if !strings.Contains(top, "\x1b_G") || !strings.Contains(top, "\x1b\\") {
		t.Fatalf("kitty APC missing: %q", top)
	}
	if !strings.Contains(top, "a=T") || !strings.Contains(top, "f=100") ||
		!strings.Contains(top, "C=1") || !strings.Contains(top, "q=2") {
		t.Fatalf("kitty control data missing: %q", top)
	}
	for i := 1; i < len(lines); i++ {
		if lines[i] != "" {
			t.Fatalf("non-top kitty row %d = %q, want empty", i, lines[i])
		}
	}
}

func TestEncodeKittyChunksLargePayload(t *testing.T) {
	data := bytes.Repeat([]byte{0xab}, 12000) // base64 > 4096 chars → multiple chunks
	out := encodeKitty("a=T,f=100", data)
	if got := strings.Count(out, "\x1b_G"); got < 2 {
		t.Fatalf("expected multiple chunks, got %d", got)
	}
	if !strings.Contains(out, "\x1b_Ga=T,f=100,m=1;") {
		t.Fatalf("first chunk control data wrong: %q", out[:80])
	}
	if !strings.Contains(out, "m=0;") {
		t.Fatalf("missing final chunk marker: %q", out[len(out)-64:])
	}
	// Round-trip: only valid base64 sits after the control/semicolon.
	for _, seg := range strings.Split(out, "\x1b\\") {
		seg = strings.TrimPrefix(seg, "\x1b_G")
		if seg == "" {
			continue
		}
		semi := strings.IndexByte(seg, ';')
		if semi < 0 {
			t.Fatalf("chunk without payload: %q", seg)
		}
		payload := seg[semi+1:]
		if _, err := base64.StdEncoding.DecodeString(payload); err != nil {
			t.Fatalf("chunk payload not base64: %v", err)
		}
	}
}

func TestKittyImageSurvivesLineTruncation(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 64, 64))); err != nil {
		t.Fatal(err)
	}
	cov := buildArtwork(image.NewRGBA(image.Rect(0, 0, 64, 64)))
	lines := renderCover(cov, artKitty)
	// Simulate the fit/ renderer truncation: a left-padded line holding the APC.
	line := strings.Repeat(" ", 60) + lines[0]
	if got := ansi.StringWidth(lines[0]); got != 0 {
		t.Fatalf("kitty APC reports width %d, want 0", got)
	}
	truncated := ansi.Truncate(line, 80, "…")
	if !strings.Contains(truncated, "\x1b_G") || !strings.Contains(truncated, "\x1b\\") {
		t.Fatalf("APC lost after truncation: %q", truncated[len(truncated)-40:])
	}
}

func TestArtBandPutsCoverLeftLyricsRight(t *testing.T) {
	orig := artworkMode
	artworkMode = artBlock
	defer func() { artworkMode = orig }()

	m := New(&fakeClient{})
	m.artwork = true
	m.width = 90
	m.height = 30
	m.now = NowPlaying{ID: "i.1", CatalogID: "123", Title: "Song", ArtistName: "Artist", AlbumName: "Album"}
	cov := &artwork{block: image.NewRGBA(image.Rect(0, 0, artworkWide, artworkHigh))}
	for y := range artworkHigh {
		for x := range artworkWide {
			cov.block.SetRGBA(x, y, color.RGBA{200, 10, 30, 255})
		}
	}
	m.coverLines = renderArtwork(cov.block)
	m.lyrics = []lyricLine{{start: 1, end: 2, text: "Hello world"}, {start: 2, end: 3, text: "Second line"}}

	lines := m.nowLines()
	if len(lines) != artworkBandRows {
		t.Fatalf("nowLines = %d rows, want %d", len(lines), artworkBandRows)
	}
	top := ansi.Strip(lines[0])
	if !strings.HasPrefix(top, "╭"+strings.Repeat("─", artworkCols)+"╮") ||
		!strings.HasPrefix(ansi.Strip(lines[len(lines)-1]), "╰"+strings.Repeat("─", artworkCols)+"╯") {
		t.Fatalf("cover not on the left:\n%q", top)
	}
	for i := 1; i <= artworkRows; i++ {
		if !strings.HasPrefix(ansi.Strip(lines[i]), "│"+strings.Repeat("█", artworkCols)+"│") {
			t.Fatalf("cover has extra padding on row %d", i)
		}
	}
	if !strings.Contains(top, "Song") {
		t.Fatalf("title missing from band:\n%q", top)
	}
	if !strings.Contains(ansi.Strip(lines[1]), "Artist") ||
		!strings.Contains(ansi.Strip(lines[2]), "╭─ Lyrics") ||
		!strings.Contains(ansi.Strip(lines[len(lines)-1]), "╯") {
		t.Fatal("lyrics must retain a border beneath the song metadata")
	}
	var full strings.Builder
	for _, l := range lines {
		full.WriteString(ansi.Strip(l))
		full.WriteByte('\n')
	}
	if !strings.Contains(full.String(), "Hello world") {
		t.Fatalf("lyrics missing from band:\n%s", full.String())
	}
}

func TestMenuDropsLyricsSidebarWhenArtShown(t *testing.T) {
	orig := artworkMode
	artworkMode = artBlock
	defer func() { artworkMode = orig }()

	m := New(&fakeClient{})
	m.artwork = true
	m.width = 90
	m.height = 30
	m.now = NowPlaying{ID: "i.1", CatalogID: "123", Title: "Song"}
	m.coverLines = renderArtwork(image.NewRGBA(image.Rect(0, 0, artworkWide, artworkHigh)))

	got := m.menuLines(90, 10)
	want := m.listLines(90, 10)
	if len(got) != len(want) {
		t.Fatalf("menuLines(%d) != listLines(%d) when art shown", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("menuLines should match listLines when art shown, row %d differs:\n%q\n%q", i, got[i], want[i])
		}
	}
}

func TestDetectArtworkProtocol(t *testing.T) {
	env := func(keys ...string) func(string) string {
		m := map[string]string{}
		for i := 0; i+1 < len(keys); i += 2 {
			m[keys[i]] = keys[i+1]
		}
		return func(k string) string { return m[k] }
	}
	if got := detectArtworkProtocol(env()); got != artBlock {
		t.Fatalf("empty env = %v, want block", got)
	}
	if got := detectArtworkProtocol(env("KITTY_WINDOW_ID", "1")); got != artKitty {
		t.Fatalf("KITTY_WINDOW_ID = %v, want kitty", got)
	}
	if got := detectArtworkProtocol(env("TERM_PROGRAM", "wezterm")); got != artKitty {
		t.Fatalf("wezterm = %v, want kitty", got)
	}
	if got := detectArtworkProtocol(env("TERM_PROGRAM", "iTerm.app")); got != artKitty {
		t.Fatalf("iTerm.app = %v, want kitty", got)
	}
	if got := detectArtworkProtocol(env("TERM", "xterm-256color")); got != artBlock {
		t.Fatalf("plain xterm = %v, want block", got)
	}
	for _, vars := range [][]string{
		{"TMUX", "/tmp/tmux", "KITTY_WINDOW_ID", "1"},
		{"STY", "screen-session", "TERM_PROGRAM", "wezterm"},
		{"TERM", "tmux-256color", "KITTY_WINDOW_ID", "1"},
	} {
		if detectArtworkProtocol(env(vars...)) != artBlock {
			t.Fatalf("multiplexer did not fall back: %v", vars)
		}
	}
}

func TestArtworkUpscalingHasNoHoles(t *testing.T) {
	src := image.NewRGBA(image.Rect(5, 7, 7, 9))
	want := color.RGBA{200, 50, 10, 255}
	for y := 7; y < 9; y++ {
		for x := 5; x < 7; x++ {
			src.SetRGBA(x, y, want)
		}
	}
	cov := buildArtwork(src)
	full, err := png.Decode(bytes.NewReader(cov.png))
	if err != nil {
		t.Fatal(err)
	}
	for y := range artworkPNGSize {
		for x := range artworkPNGSize {
			expected := want
			if x < artworkBorderPixels || y < artworkBorderPixels ||
				x >= artworkPNGSize-artworkBorderPixels || y >= artworkPNGSize-artworkBorderPixels {
				expected = color.RGBA{128, 128, 128, 255}
			}
			if got := color.RGBAModel.Convert(full.At(x, y)); got != expected {
				t.Fatalf("hole at %d,%d: %v", x, y, got)
			}
		}
	}
}

func TestArtworkRejectsOversizedDimensions(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4097, 1))); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeArtwork(buf.Bytes()); err == nil {
		t.Fatal("oversized image accepted")
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if err := saveArtwork("large", buf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if cachedArtwork("large") != nil {
		t.Fatal("oversized cached image accepted")
	}
}

// View lays the frame out around a stand-in for the kitty cover; the real
// payload must come back exactly once and leave the stored model untouched.
func TestKittyCoverRestoredAfterLayout(t *testing.T) {
	old := artworkMode
	artworkMode = artKitty
	t.Cleanup(func() { artworkMode = old })
	m := New(&fakeClient{})
	m.width, m.height = 120, 40
	m.artwork = true
	m.now = NowPlaying{Title: "Song", ArtistName: "Artist"}
	m.coverLines = make([]string, artworkRows)
	cover := kittyImage([]byte("image"), artworkCols, artworkRows)
	m.coverLines[0] = cover
	frame := m.View()
	if strings.Count(frame, cover) != 1 || strings.Contains(frame, coverStandIn) {
		t.Fatalf("cover not restored exactly once: %q", frame)
	}
	if m.coverLines[0] != cover {
		t.Fatal("View modified the stored cover lines")
	}
}

// The details panel renders ~60x/s; a cached placement must be reused for the
// same size rather than base64-encoding the PNG again.
func TestKittyCacheReusesEncodingForSameSize(t *testing.T) {
	c := &kittyCache{png: []byte("png"), cols: 4, rows: 4, s: "cached"}
	if got := c.get(detailsImageID, 4, 4); got != "cached" {
		t.Fatalf("same size re-encoded: got %q", got)
	}
	if got := c.get(detailsImageID, 5, 5); got == "cached" || !strings.HasPrefix(got, "\x1b_G") {
		t.Fatalf("new size should re-encode, got %q", got)
	}
}

// Library-only and uploaded tracks have no catalog ID, but their artwork is
// still served from /v1/me/library under the i. ID.
func TestEnsureArtworkFetchesLibraryOnlyTrack(t *testing.T) {
	m := New(&fakeClient{})
	m.artwork = true
	m.now = NowPlaying{ID: "i.9", Title: "Upload"}
	if m.ensureArtwork() == nil {
		t.Fatal("no artwork fetch for a library-only track")
	}
	if m.artworkID != "i.9" {
		t.Fatalf("artworkID = %q, want i.9", m.artworkID)
	}
}
