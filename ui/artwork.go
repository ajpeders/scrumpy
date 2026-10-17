package ui

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// artwork is a track's cover, held at two resolutions: a small quadrant-block
// canvas (for terminals without a graphics protocol) and a full-size PNG (for
// the kitty graphics protocol).
type artwork struct {
	block *image.RGBA
	png   []byte
}

// artworkMsg carries the decoded cover for a catalog ID. A non-nil err means the
// artwork could not be fetched; the UI simply shows text as before.
type artworkMsg struct {
	catalogID string
	cover     *artwork
	err       error
}

// The cover is artworkCols columns by artworkRows rows (cells are roughly twice
// as tall as wide, so that is square); the quadrant canvas holds four pixels per
// cell.
const (
	artworkCols     = 20
	artworkRows     = artworkCols / 2
	artworkWide     = artworkCols * 2
	artworkHigh     = artworkRows * 2 // == artworkCols
	artworkBandCols = artworkCols + 2 // cover and border, without extra padding
	artworkBandRows = artworkRows + 2

	artworkPNGSize      = 256 // resolution of the full image sent to kitty terminals
	artworkBorderPixels = 2
	kittyImageID        = 1 // fixed id so each new cover replaces the previous one
	detailsImageID      = 2 // the d panel's cover, kept apart from now-playing's
)

// artProtocol is how the cover is drawn, chosen once from the environment.
type artProtocol int

const (
	artBlock artProtocol = iota // Unicode quadrant blocks: works everywhere
	artKitty                    // kitty graphics protocol: true image
)

// artworkMode is the terminal's image capability. It is read once at startup;
// tests may overwrite it to exercise a specific protocol.
var artworkMode = detectArtworkProtocol(os.Getenv)

// detectArtworkProtocol reports kitty-graphics support from the terminal's
// environment. iTerm2, WezTerm and Ghostty implement the kitty protocol, so they
// share the artKitty path.
func detectArtworkProtocol(getenv func(string) string) artProtocol {
	term := getenv("TERM")
	if getenv("TMUX") != "" || getenv("STY") != "" || strings.HasPrefix(term, "screen") || strings.HasPrefix(term, "tmux") {
		return artBlock
	}
	switch {
	case getenv("KITTY_WINDOW_ID") != "":
		return artKitty
	case getenv("TERM") == "xterm-kitty", getenv("TERM") == "xterm-ghostty":
		return artKitty
	case getenv("LC_TERMINAL") == "iTerm2":
		return artKitty
	}
	switch getenv("TERM_PROGRAM") {
	case "kitty", "wezterm", "ghostty", "iTerm.app", "Konsole":
		return artKitty
	}
	return artBlock
}

// renderCover turns a decoded cover into display rows for the active protocol:
// a kitty image occupies the first row (the terminal draws it over the rows
// below), while quadrant blocks fill every row.
func renderCover(cov *artwork, mode artProtocol) []string {
	return renderCoverSize(cov, mode, artworkCols, artworkRows)
}

func renderCoverSize(cov *artwork, mode artProtocol, cols, rows int) []string {
	if cov == nil {
		return nil
	}
	if mode == artKitty && cov.png != nil {
		lines := make([]string, rows)
		lines[0] = kittyImage(cov.png, cols, rows)
		return lines
	}
	if cov.block == nil {
		return nil
	}
	return renderArtwork(cov.block)
}

// Fit the square cover to whole terminal cells when their pixel dimensions
// permit it, avoiding letterboxing between the image and the lyrics bottom.
func artworkSizeForCells(cellW, cellH int) (cols, rows int) {
	if cellW <= 0 || cellH <= 0 {
		return artworkCols, artworkRows
	}
	a, b := cellW, cellH
	for b != 0 {
		a, b = b, a%b
	}
	cols, rows = cellH/a, cellW/a
	// Unusual cell proportions must not turn the cover into an oversized panel.
	if cols > artworkCols+10 || rows > artworkRows+5 {
		return artworkCols, artworkRows
	}
	scale := max(1, (artworkCols+cols/2)/cols)
	scale = min(scale, (artworkCols+10)/cols, (artworkRows+5)/rows)
	return cols * scale, rows * scale
}

// kittyImage transmits a PNG and displays it scaled to cols×rows cells without
// moving the cursor (C=1), under the text (z=-1) and without an acknowledgement
// response (q=2), so the Bubble Tea line renderer stays aligned, blank cells
// over the cover stay transparent and the image persists across redraws.
func kittyImage(data []byte, cols, rows int) string {
	return kittyImageAs(kittyImageID, data, cols, rows)
}

// kittyImageAs is kittyImage under an explicit image id. Transmitting with an
// id replaces that image's other placements, so each cover needs its own id.
func kittyImageAs(id int, data []byte, cols, rows int) string {
	ctrl := fmt.Sprintf("a=T,f=100,c=%d,r=%d,C=1,q=2,z=-1,i=%d", cols, rows, id)
	return encodeKitty(ctrl, data)
}

// kittyCache memoises a kitty placement of one PNG so panels redrawn every
// frame don't base64-encode the image again until their size changes.
type kittyCache struct {
	png        []byte
	cols, rows int
	s          string
}

func (c *kittyCache) get(id, cols, rows int) string {
	if c.s == "" || c.cols != cols || c.rows != rows {
		c.s, c.cols, c.rows = kittyImageAs(id, c.png, cols, rows), cols, rows
	}
	return c.s
}

// encodeKitty emits the graphics APC sequence, chunking the base64 payload into
// ≤4096-byte pieces (all but the last a multiple of four) as the protocol
// requires. Only the first chunk carries the extra control data.
func encodeKitty(ctrl string, data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	var b strings.Builder
	prefix := ctrl + ",m=" // the first chunk carries the control data plus m
	m := "1"
	for len(encoded) > 0 {
		n := min(4096, len(encoded))
		if len(encoded) > 4096 {
			n -= n % 4
		}
		if n >= len(encoded) {
			m = "0" // final chunk
		}
		b.WriteString("\x1b_G")
		b.WriteString(prefix)
		b.WriteString(m)
		b.WriteString(";")
		b.WriteString(encoded[:n])
		b.WriteString("\x1b\\")
		encoded = encoded[n:]
		prefix = "m=" // later chunks carry only the m key
	}
	return b.String()
}

// renderArtwork draws the quadrant-block canvas with Unicode quadrant blocks:
// each cell renders four pixels as one glyph, lit quadrants in the foreground
// colour and the rest in the background colour.
func renderArtwork(art *image.RGBA) []string {
	lines := make([]string, artworkRows)
	for row := range artworkRows {
		var b strings.Builder
		for col := range artworkCols {
			x, y := col*2, row*2
			b.WriteString(quadBlock(
				art.RGBAAt(x, y), art.RGBAAt(x+1, y),
				art.RGBAAt(x, y+1), art.RGBAAt(x+1, y+1)))
		}
		lines[row] = b.String() + "\x1b[0m"
	}
	return lines
}

// quadrantGlyphs maps a 4-bit mask of lit quadrants (bit 0 top-left, bit 1
// top-right, bit 2 bottom-left, bit 3 bottom-right) to the block character that
// lights exactly those quadrants in the foreground colour.
var quadrantGlyphs = [16]string{
	" ", "▘", "▝", "▀", "▖", "▌", "▞", "▛",
	"▗", "▚", "▐", "▜", "▄", "▙", "▟", "█",
}

// quadBlock renders a 2×2 cell of pixels as one glyph, choosing the foreground
// and background colours that best represent its four pixels and the glyph whose
// lit quadrants match that split.
func quadBlock(tl, tr, bl, br color.RGBA) string {
	px := [4]color.RGBA{tl, tr, bl, br}
	fg, bg := pickPair(px)
	mask := 0
	for i, p := range px {
		if dist(p, fg) <= dist(p, bg) {
			mask |= 1 << i
		}
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm%s",
		fg.R, fg.G, fg.B, bg.R, bg.G, bg.B, quadrantGlyphs[mask])
}

// pickPair chooses the two colours that best represent a 2×2 block: the
// foreground for lit quadrants and the background for the rest. A single colour
// is returned twice and renders as a solid block.
func pickPair(px [4]color.RGBA) (fg, bg color.RGBA) {
	uniq := uniqueColors(px)
	if len(uniq) == 1 {
		return uniq[0], uniq[0]
	}
	best := -1
	for i := range uniq {
		for j := range uniq {
			if i == j {
				continue
			}
			if e := pairErr(px, uniq[i], uniq[j]); best < 0 || e < best {
				best, fg, bg = e, uniq[i], uniq[j]
			}
		}
	}
	return fg, bg
}

func uniqueColors(px [4]color.RGBA) []color.RGBA {
	out := px[:0:0]
	for _, p := range px {
		found := false
		for _, q := range out {
			if p == q {
				found = true
				break
			}
		}
		if !found {
			out = append(out, p)
		}
	}
	return out
}

// pairErr measures how well fg/bg represent the block: each pixel is assigned
// to the closer colour and charged the squared distance to it.
func pairErr(px [4]color.RGBA, fg, bg color.RGBA) int {
	e := 0
	for _, p := range px {
		df, db := dist(p, fg), dist(p, bg)
		if df <= db {
			e += df
		} else {
			e += db
		}
	}
	return e
}

func dist(a, b color.RGBA) int {
	dr, dg, db := int(a.R)-int(b.R), int(a.G)-int(b.G), int(a.B)-int(b.B)
	return dr*dr + dg*dg + db*db
}

// downsampleTo averages src into a wxh RGBA buffer, so each output pixel samples
// its source region regardless of the source size.
func downsampleTo(src image.Image, w, h int) *image.RGBA {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if sw <= 0 || sh <= 0 || w <= 0 || h <= 0 {
		return nil
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			sx0, sx1 := x*sw/w, max(x*sw/w+1, (x+1)*sw/w)
			sy0, sy1 := y*sh/h, max(y*sh/h+1, (y+1)*sh/h)
			var r, g, b, n uint64
			for sy := sy0; sy < sy1; sy++ {
				for sx := sx0; sx < sx1; sx++ {
					pr, pg, pb, _ := src.At(src.Bounds().Min.X+sx, src.Bounds().Min.Y+sy).RGBA()
					r += uint64(pr >> 8)
					g += uint64(pg >> 8)
					b += uint64(pb >> 8)
					n++
				}
			}
			if n == 0 {
				continue
			}
			out.SetRGBA(x, y, color.RGBA{uint8(r / n), uint8(g / n), uint8(b / n), 255})
		}
	}
	return out
}

// buildArtwork prepares a decoded cover for both protocols: the quadrant-block
// canvas and a PNG for the kitty protocol.
func buildArtwork(src image.Image) *artwork {
	cov := &artwork{block: downsampleTo(src, artworkWide, artworkHigh)}
	full := downsampleTo(src, artworkPNGSize, artworkPNGSize)
	if full == nil {
		return cov
	}
	// Keep the border in the same square pixel canvas as the cover. A text
	// border would surround the terminal's cell rectangle, which can letterbox
	// a square image when cells aren't exactly twice as tall as wide.
	border := color.RGBA{128, 128, 128, 255}
	for y := range artworkPNGSize {
		for x := range artworkPNGSize {
			if x < artworkBorderPixels || y < artworkBorderPixels ||
				x >= artworkPNGSize-artworkBorderPixels || y >= artworkPNGSize-artworkBorderPixels {
				full.SetRGBA(x, y, border)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, full); err == nil {
		cov.png = buf.Bytes()
	}
	return cov
}

// fetchArtwork loads the cover for catalogID: from the disk cache when present,
// otherwise from Apple's artwork URL discovered through Cider's amapi
// passthrough, downloading and caching the thumbnail so a track is fetched once.
func fetchArtwork(c Client, catalogID string) tea.Cmd {
	return func() tea.Msg {
		cov, err := loadArtwork(c, catalogID)
		return artworkMsg{catalogID: catalogID, cover: cov, err: err}
	}
}

func loadArtwork(c Client, catalogID string) (*artwork, error) {
	return loadArtworkFor(c, catalogID, "songs")
}

func loadArtworkFor(c Client, catalogID, resource string) (*artwork, error) {
	if cov := cachedArtwork(catalogID); cov != nil {
		return cov, nil
	}
	u, err := artworkURL(c, catalogID, resource)
	if err != nil {
		return nil, err
	}
	raw, err := downloadImage(u)
	if err != nil {
		return nil, err
	}
	img, err := decodeArtwork(raw)
	if err != nil {
		return nil, fmt.Errorf("decode artwork: %w", err)
	}
	// A read-only/full cache must not prevent displaying a downloaded cover.
	_ = saveArtwork(catalogID, raw)
	return buildArtwork(img), nil
}

const artworkDownloadLimit = 4 << 20

// errArtworkTrust is returned when the URL returned by Cider's amapi
// passthrough fails the scheme/host check. A compromised or custom Cider
// endpoint can otherwise point us at local/private addresses (SSRF); the
// Cider token isn't sent on this request but the cover would still be
// fetched from the chosen host.
var errArtworkTrust = errors.New("artwork host not trusted")

// checkArtworkTrust validates the URL the Cider amapi passthrough returned for
// the cover. It enforces https and a host allowlist; anything else is rejected
// before downloadImage opens a TCP connection.
//
// Trust boundary: artwork URLs are emitted by Apple's catalog API and served
// from *.mzstatic.com. Anything else is either a misconfiguration (the user
// pointed Cider at a proxy that rewrites URLs) or an attack (a compromised
// Cider returns a private/loopback host). The check refuses both.
func checkArtworkTrust(u string) error {
	parsed, err := url.Parse(u)
	if err != nil {
		return fmt.Errorf("artwork url parse: %w", err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("%w: scheme must be https, got %q", errArtworkTrust, parsed.Scheme)
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("%w: missing host", errArtworkTrust)
	}
	// Apple owns mzstatic.com; the storefront API only ever returns image
	// URLs under that domain. A custom Cider or a tampering proxy can return
	// any host, so the allowlist is the trust boundary.
	if host != "mzstatic.com" && !strings.HasSuffix(host, ".mzstatic.com") {
		return fmt.Errorf("%w: host %q is not on the Apple CDN allowlist", errArtworkTrust, host)
	}
	// Belt and braces: reject any literal IP that slipped into the host
	// (mzstatic.com is a DNS name, never a literal IP). Catches hosts like
	// `127.0.0.1.mzstatic.com` where the allowlist suffix alone wouldn't,
	// and any future attempt to put a literal IP into a hostname.
	if ip := net.ParseIP(host); ip != nil {
		return fmt.Errorf("%w: literal IP %q rejected", errArtworkTrust, host)
	}
	// The leftmost label of a legitimate *.mzstatic.com hostname is a
	// DNS name (`is1-ssl`, `audio-ssl`, etc.), not a numeric octet. A label
	// that's all digits is suspicious enough to refuse up front, before DNS
	// gets a chance to resolve `1.2.3.4.mzstatic.com`.
	if idx := strings.IndexByte(host, '.'); idx > 0 {
		left := host[:idx]
		allDigits := true
		for _, r := range left {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits && left != "" {
			return fmt.Errorf("%w: numeric leftmost label %q rejected", errArtworkTrust, left)
		}
	}
	return nil
}

// artworkClient is the http.Client used for cover downloads. The default
// http.Client follows redirects up to 10 times, which would defeat the trust
// check: an attacker-controlled host could 302 to a private IP. Apple serves
// artwork bytes directly; redirects are a sign of compromise, not a normal
// path, so the client refuses them.
var artworkClient = &http.Client{
	Timeout: 15 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("artwork redirect refused")
	},
}

func decodeArtwork(raw []byte) (image.Image, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	// Bound decoded memory as well as compressed bytes before allocating pixels.
	if config.Width <= 0 || config.Height <= 0 || config.Width > 4096 || config.Height > 4096 ||
		int64(config.Width)*int64(config.Height) > 4<<20 {
		return nil, errors.New("artwork dimensions too large")
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	return img, err
}

// artworkResource maps an item type to its Apple Music catalog resource
// (songs, albums, playlists, music-videos) or returns "" when the type has
// no single artwork (artists, stations).
func artworkResource(itemType string) string {
	// Cider's queue reports singular types ("song"); Apple's API uses plurals.
	if itemType == "song" || itemType == "album" || itemType == "playlist" || itemType == "music-video" {
		itemType += "s"
	}
	switch {
	case itemType == "songs" || strings.HasSuffix(itemType, "-songs"):
		return "songs"
	case itemType == "albums" || strings.HasSuffix(itemType, "-albums"):
		return "albums"
	case itemType == "playlists" || strings.HasSuffix(itemType, "-playlists"):
		return "playlists"
	case itemType == "music-videos" || strings.HasSuffix(itemType, "-music-videos"):
		return "music-videos"
	}
	return ""
}

// artworkURL asks Apple for the catalogue artwork URL for any item with art
// (songs, albums, playlists, music videos) and rewrites its {w}/{h} template
// to a modest square thumbnail instead of the huge original.
func artworkURL(c Client, catalogID, resource string) (string, error) {
	if resource == "" {
		return "", errors.New("no artwork for this item")
	}
	var resp struct {
		Data []struct {
			Attributes struct {
				Artwork struct {
					URL string `json:"url"`
				} `json:"artwork"`
			} `json:"attributes"`
		} `json:"data"`
	}
	// Library IDs (i. songs, l. albums, p. playlists) are unknown to the
	// catalog; their artwork is served from the user's library instead.
	var path string
	if strings.HasPrefix(catalogID, "i.") || strings.HasPrefix(catalogID, "l.") || strings.HasPrefix(catalogID, "p.") {
		path = "/v1/me/library/" + resource + "/" + url.PathEscape(catalogID)
	} else {
		sf, err := storefront(c)
		if err != nil {
			return "", err
		}
		path = "/v1/catalog/" + url.PathEscape(sf) + "/" + resource + "/" + url.PathEscape(catalogID)
	}
	if err := c.AMAPI(path, &resp); err != nil {
		return "", err
	}
	if len(resp.Data) == 0 || resp.Data[0].Attributes.Artwork.URL == "" {
		return "", errors.New("no artwork for this item")
	}
	size := artworkPNGSize
	u := strings.ReplaceAll(resp.Data[0].Attributes.Artwork.URL, "{w}", strconv.Itoa(size))
	return strings.ReplaceAll(u, "{h}", strconv.Itoa(size)), nil
}

// imageClient and downloadImage are separate so tests can stub image fetching
// against a local server without touching Cider's client. The imageClient alias
// points at artworkClient (defined above) so the trust check and the
// redirect-refusal policy apply automatically.
var imageClient = artworkClient

var downloadImage = func(u string) ([]byte, error) {
	// Validate the URL before opening a TCP connection. The Cider amapi
	// passthrough is the trust source for the host; if it returned a host
	// we don't recognise, refuse rather than dial.
	if err := checkArtworkTrust(u); err != nil {
		return nil, err
	}
	resp, err := imageClient.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("artwork HTTP status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, artworkDownloadLimit+1))
	if len(raw) > artworkDownloadLimit {
		return nil, errors.New("artwork download too large")
	}
	return raw, err
}

func artworkCacheDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "scrumpy", "artwork"), nil
}

func artworkCachePath(catalogID string) (string, error) {
	dir, err := artworkCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, cleanID(catalogID)+".img"), nil
}

// cleanID keeps a cache filename from ever escaping the artwork directory.
func cleanID(id string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		}
		return '-'
	}, id)
}

func cachedArtwork(catalogID string) *artwork {
	path, err := artworkCachePath(catalogID)
	if err != nil {
		return nil
	}
	f, err := os.Open(path)
	legacy := false
	if os.IsNotExist(err) {
		// Migrate on demand rather than copying an entire old cache at startup.
		if dir, dirErr := os.UserCacheDir(); dirErr == nil {
			f, err = os.Open(filepath.Join(dir, "sidecue", "artwork", cleanID(catalogID)+".img"))
			legacy = err == nil
		}
	}
	if err != nil {
		return nil
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, artworkDownloadLimit+1))
	if err != nil || len(raw) > artworkDownloadLimit {
		return nil
	}
	img, err := decodeArtwork(raw)
	if err != nil {
		return nil
	}
	if legacy {
		_ = saveArtwork(catalogID, raw)
	}
	return buildArtwork(img)
}

// saveArtwork writes the raw image beside its destination and renames it over,
// then prunes the cache so a torn write or an unlikely full cache cannot break
// playback. The cache bounds total bytes, oldest-first.
func saveArtwork(catalogID string, raw []byte) error {
	path, err := artworkCachePath(catalogID)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".art-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return err
	}
	pruneArtworkCache(dir, artworkCacheLimit)
	return nil
}

// artworkCacheLimit bounds the on-disk artwork cache across all tracks.
const artworkCacheLimit int64 = 32 << 20 // 32 MiB

// pruneArtworkCache deletes the oldest cached images until the cache is within
// its byte limit. It is best-effort: a read error just stops pruning.
func pruneArtworkCache(dir string, limit int64) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type cached struct {
		name string
		size int64
		mod  int64
	}
	var files []cached
	var total int64
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || info.IsDir() {
			continue
		}
		files = append(files, cached{e.Name(), info.Size(), info.ModTime().UnixNano()})
		total += info.Size()
	}
	if total <= limit {
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod < files[j].mod })
	for _, f := range files {
		if total <= limit {
			break
		}
		_ = os.Remove(filepath.Join(dir, f.name))
		total -= f.size
	}
}
