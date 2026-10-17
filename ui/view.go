package ui

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// fixedRows counts the rows around the visualizer and list: header, now
// playing, seek bar and the footer's key lines. The tab row is added whenever
// the list is shown.
func (m Model) fixedRows() int {
	w := max(1, m.width)
	lyricsBand := 0
	lyricsBreathing := 0
	if m.lyricsBandShown() {
		lyricsBand = m.lyricsBandRows()
		// One breathing row sits between the boxed lyrics panel and the seek
		// bar (see View()), keeping the band from butting up against the
		// transport. Layout needs to reserve it so the bottom panel can't
		// steal it from the list.
		lyricsBreathing = 1
	}
	footerRows := len(m.footerLines(m.bottomWidth()))
	if m.bottomBordered() {
		footerRows = 2 // the top border and the bottom border carrying the hints
	}
	// The visualizer leaves a blank row above artwork, in addition to the
	// breathing row between artwork and the seek bar.
	vizArtGap := btoi(m.artworkShown() && m.vizAvailable())
	return 1 + lyricsBand + lyricsBreathing + len(m.nowLines()) + btoi(m.artworkShown()) + vizArtGap +
		len(m.seekLines(w)) + footerRows
}

func (m Model) bottomBordered() bool {
	return !m.fullScreenViz() && m.width >= 12 && m.height >= 10
}

func (m Model) bottomWidth() int {
	return max(1, m.width-2*btoi(m.bottomBordered()))
}

func (m Model) vizAvailable() bool  { return (m.viz != nil || m.vizFrames != nil) && !m.vizClosed }
func (m Model) fullScreenViz() bool { return m.fullViz && m.vizAvailable() }

// lyricsBandRows is the height of the boxed lyrics panel placed below the
// now-playing line when album art is hidden. In the normal layout the band is
// a modest strip so the queue below stays usable; in full-screen visualizer
// mode (`v`) it expands so the lyrics read like the main feature.
//
// Minimum 5 rows in normal mode: 2 for box borders, 3 for content so the
// current line can be centred with a line above and below it (the lyrics
// follow algorithm scrolls on the current line, not just after it).
func (m Model) lyricsBandRows() int {
	if m.fullScreenViz() {
		return min(12, max(6, m.height/3))
	}
	return min(7, max(5, m.height/6))
}

// lyricsBandShown reports whether the lyrics belong in their own band beneath
// the visualizer. Artwork already has its own boxed lyrics (see artBand), and
// a session-only `y` press tucks the lyrics away, so neither replaces this band.
func (m Model) lyricsBandShown() bool {
	return m.lyricsOn && !m.lyricsHidden && !m.artworkShown() && m.width >= 24
}

// layout gives the visualizer about 60% of the free rows and the list the rest
// (at least 3). Without a visualizer, or with too few rows for one, the list
// takes everything. fixedRows already accounts for the artwork band and the
// lyrics band, so neither needs to be subtracted here.
func (m Model) layout() (vizRows, listRows int) {
	free := m.height - m.fixedRows()
	if !m.fullScreenViz() && !m.bottomBordered() {
		free-- // tab row
	}
	free = max(0, free)
	switch {
	case !m.vizAvailable():
		// Tiny view with no borders or visualizer: keep the list visible even
		// when there's only a single row of slack. fit() will drop the
		// footer's line if the bottom panel runs over, which is the right
		// trade — the menu tells the user what they can do.
		if !m.bottomBordered() && free < 1 {
			return 0, 1
		}
		return 0, free
	case m.fullScreenViz():
		return max(0, free-btoi(m.artworkShown())), 0
	}
	listRows = max(3, free*2/5)
	if m.settings || m.help {
		listRows = min(free, max(listRows, 12))
	}
	if vizRows = free - listRows; vizRows < 2 {
		// Not enough room for both a 2-row minimum visualizer and a 3-row
		// minimum list. At the same time, when the terminal is too small for
		// the visualizer's floor, we still want to keep the list reachable —
		// especially after `y` hides the lyrics and leaves the bottom panel
		// as the user's only navigation. Give the list at least one row when
		// the bottom is unbordered, mirroring the no-viz fallback above.
		if !m.bottomBordered() && free < 1 {
			return 0, 1
		}
		return 0, free
	}
	return vizRows, listRows
}

// coverStandIn is a zero-width APC that holds the kitty cover's place while the
// frame is laid out.
const coverStandIn = "\x1b_Gcover\x1b\\"

// View lays out the frame around a stand-in for the kitty cover, then swaps the
// cover back in. The cover is a base64 PNG of up to megabytes, and measuring it
// in pad, fit and lipgloss on every visualizer redraw cost more than a frame.
func (m Model) View() string {
	cover := ""
	if len(m.coverLines) > 0 && strings.HasPrefix(m.coverLines[0], "\x1b_G") {
		cover = m.coverLines[0]
		// Copy: m.coverLines shares its backing array with the stored model.
		m.coverLines = append([]string{coverStandIn}, m.coverLines[1:]...)
	}
	return strings.Replace(m.view(), coverStandIn, cover, 1)
}

func (m Model) view() string {
	w := max(1, m.width)
	lines := []string{m.headerLine(w)}
	// Graphics outlive text redraws. Explicitly remove our placement whenever
	// its band is hidden (including offline and narrow-window views).
	if artworkMode == artKitty && (m.offline || m.now.Title == "" || len(m.coverLines) == 0 || !m.artworkShown()) {
		lines[0] = fmt.Sprintf("\x1b_Ga=d,d=I,i=%d,q=2;\x1b\\", kittyImageID) + lines[0]
	}
	if m.offline {
		lines = append(lines, m.offlineLines(w, m.height-1)...)
		return m.themedFrame(fit(dropDetailsImage(lines), w, m.height), w)
	}
	vizRows, listRows := m.layout()
	lyricsRows := 0
	// In full-screen visualizer mode the bars fill the body. When `?`
	// opens the help overlay, the lyrics band competes with help for rows
	// and the user has explicitly asked for help; let help win. Pressing
	// `?` again (or `esc`) closes help and the lyrics come back.
	if m.lyricsBandShown() && (vizRows > 0 || !m.vizAvailable()) && !(m.fullScreenViz() && m.help) {
		lyricsRows = m.lyricsBandRows()
	}
	if vizRows > 0 && !(m.help && m.fullScreenViz()) {
		if m.waveform {
			samples := m.samples
			if !m.now.Playing {
				samples = nil
			}
			lines = append(lines, renderWaveform(samples, w, vizRows)...)
		} else {
			lines = append(lines, renderSpectrum(m.meter.levels, m.meter.peaks, w, vizRows)...)
		}
		if m.artworkShown() {
			lines = append(lines, "")
		}
	}
	lines = append(lines, m.nowLines()...)
	if m.artworkShown() {
		lines = append(lines, "") // breathing room between artwork and transport
	}
	if lyricsRows > 0 {
		// Lyrics panel sits between the now-playing metadata and the seek bar
		// at full terminal width so long wrapped lines stay readable; the list
		// in the bottom panel keeps browsing in reach below it.
		innerW := w - 2
		body := m.lyricsLines(max(0, innerW-2), lyricsRows-2)
		lines = append(lines, panelBox("Lyrics", body, w, lyricsRows)...)
		lines = append(lines, "") // breathing room between lyrics and seek bar
	}
	lines = append(lines, m.seekLines(w)...)
	if !m.fullScreenViz() {
		bw := m.bottomWidth()
		var bottom []string
		if !m.bottomBordered() {
			bottom = append(bottom, m.tabLine())
		}
		switch {
		case m.settings:
			bottom = append(bottom, m.settingsLines(bw, listRows)...)
		case m.help:
			bottom = append(bottom, helpLines(bw, listRows)...)
		default:
			bottom = append(bottom, m.menuLines(bw, listRows)...)
		}
		if m.bottomBordered() {
			bottom = panelBox("", bottom, w, len(bottom)+2)
			bottom[0] = m.tabBorder(w)
			bottom[len(bottom)-1] = m.footerBorder(w)
		} else {
			bottom = append(bottom, m.footerLines(bw)...)
		}
		lines = append(lines, bottom...)
	} else {
		if m.help {
			// Full-screen visualizer mode hides the list panel, but help
			// is still reachable (? toggles it). The viz rows are skipped
			// above when both are active, so the help overlay can use the
			// remaining body height — minus the lyrics band, its breathing
			// row, now-playing, the seek bar and the footer.
			body := m.height - 1 - len(m.footerLines(w))
			body -= len(m.nowLines()) + len(m.seekLines(w))
			body -= lyricsRows
			if lyricsRows > 0 {
				body-- // breathing row between lyrics panel and seek bar
			}
			hl := helpLines(w, max(0, body))
			lines = append(lines, hl...)
		}
		lines = append(lines, m.footerLines(w)...)
	}
	return m.themedFrame(fit(dropDetailsImage(lines), w, m.height), w)
}

// dropDetailsImage deletes the d panel's kitty image when no line places it
// (panel closed, too narrow, or scrolled out), since graphics outlive redraws.
func dropDetailsImage(lines []string) []string {
	if artworkMode != artKitty || len(lines) == 0 {
		return lines
	}
	place := fmt.Sprintf(",i=%d,", detailsImageID)
	for _, l := range lines {
		if strings.Contains(l, place) {
			return lines
		}
	}
	lines[0] = fmt.Sprintf("\x1b_Ga=d,d=I,i=%d,q=2;\x1b\\", detailsImageID) + lines[0]
	return lines
}

func (m Model) themedFrame(frame string, w int) string {
	style := lipgloss.NewStyle().Width(w)
	if m.backgroundOn && themeBackground != nil {
		style = style.Background(themeBackground)
	}
	if themeForeground != nil {
		style = style.Foreground(themeForeground)
	}
	rendered := style.Render(frame)
	if themeForeground != nil {
		// Inner Lip Gloss styles end with SGR 0, clearing the frame's colours.
		// Restore the base colours after each reset so plain text and gaps do
		// not inherit stale terminal cells when changing themes. Preserve the
		// terminal's own background when the Background option is off.
		base := lipgloss.NewStyle().Foreground(themeForeground)
		if m.backgroundOn && themeBackground != nil {
			base = base.Background(themeBackground)
		}
		colours := base.Render(" ")
		if end := strings.IndexByte(colours, 'm'); end >= 0 {
			rendered = strings.ReplaceAll(rendered, "\x1b[0m", "\x1b[0m"+colours[:end+1])
		}
	}
	return rendered
}

// fit guarantees the frame never wraps or scrolls: each line is cut to the
// width and the frame to the height.
func fit(lines []string, w, h int) string {
	if h > 0 && len(lines) > h {
		lines = lines[:h]
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, w, "…")
	}
	return strings.Join(lines, "\n")
}

// spread places left and right on one line of width w.
func spread(left, right string, w int) string {
	gap := w - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 1 {
		return left
	}
	return left + strings.Repeat(" ", gap) + right
}

// pad truncates s to exactly w cells, padding with spaces.
func pad(s string, w int) string {
	t := ansi.Truncate(s, w, "…")
	return t + strings.Repeat(" ", max(0, w-ansi.StringWidth(t)))
}

// Metadata is remote text, not terminal markup. Remove escape sequences and
// control characters before styling so metadata/lyrics cannot alter the screen.
func displayText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, ansi.Strip(s))
}

func (m Model) headerLine(w int) string {
	vol := int(math.Round(m.now.Volume * 100))
	filled := min(5, max(0, (vol+10)/20))
	right := dimStyle.Render("vol ") +
		accentStyle.Render(strings.Repeat("█", filled)) +
		faintStyle.Render(strings.Repeat("░", 5-filled)) +
		dimStyle.Render(fmt.Sprintf(" %3d%% ", vol))
	return spread(" "+titleStyle.Render("scrumpy"), right, w)
}

var barBlocks = []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

// fracBlock returns the glyph for a bar of the given level within the row
// spanning [bottom, top] of the 0..1 range.
func fracBlock(level, bottom, top float64) string {
	if level >= top {
		return "█"
	}
	if level > bottom {
		i := int((level - bottom) / (top - bottom) * float64(len(barBlocks)-1))
		return barBlocks[max(0, min(i, len(barBlocks)-1))]
	}
	return " "
}

// resample reduces v to n values, keeping each group's maximum.
func resample(v []float64, n int) []float64 {
	out := make([]float64, n)
	if len(v) < n {
		return out
	}
	for i := range out {
		lo, hi := i*len(v)/n, max((i+1)*len(v)/n, i*len(v)/n+1)
		for _, x := range v[lo:hi] {
			out[i] = max(out[i], x)
		}
	}
	return out
}

// renderSpectrum draws bars filling width×height, with fractional tops, peak
// caps and a green→yellow→red gradient by row. Adapted from cliamp (MIT).
func renderSpectrum(levels, peaks []float64, width, height int) []string {
	lines := make([]string, height)
	bars := min(len(levels), (width+1)/3) // at least 2 columns + 1 gap each
	if bars == 0 {
		return lines
	}
	levels, peaks = resample(levels, bars), resample(peaks, bars)
	cols := width - (bars - 1)
	for row := range height {
		bottom := float64(height-1-row) / float64(height)
		top := float64(height-row) / float64(height)
		var b strings.Builder
		for i := range bars {
			cell := fracBlock(levels[i], bottom, top)
			if cell == " " && peaks[i] > max(bottom, 0.02) && peaks[i] <= top {
				cell = "▀"
			}
			if cell == " " && row == height-1 {
				cell = "▁" // a visible floor during silence
			}
			b.WriteString(strings.Repeat(cell, cols/bars+btoi(i < cols%bars)))
			if i < bars-1 {
				b.WriteByte(' ')
			}
		}
		lines[row] = spectrumStyle(bottom).Render(b.String())
	}
	return lines
}

// waveformNoiseFloor drops samples whose absolute value is below this
// threshold before drawing the envelope. Codec DC offset, capture
// jitter and PipeWire's reporting of quiet passages all leak a tiny
// non-zero value that the old renderWaveform scaled to a visible row.
// 0.01 is conservative: a real signal at -20 dBFS (≈ 0.10) is still
// fifty times louder than the floor, so this only eats true silence.
const waveformNoiseFloor = 0.01

// renderWaveform draws the latest PCM hop as a vertical min/max envelope. Each
// column represents a time slice, preserving transients even on narrow screens.
//
// A raw per-column min/max is fragile: capture jitter, codec DC offset and
// isolated spike samples all push the envelope out to the full height even
// during quiet passages. Two defenses:
//
//  1. Noise gate: samples below waveformNoiseFloor count as zero before the
//     min/max is computed, so sub-floor jitter doesn't paint a visible bar.
//
//  2. Percentile clamp per column: after collecting the column's samples, we
//     drop the top and bottom 10% before taking the min/max. A lone outlier
//     (e.g. a single 0.4 sample in an otherwise silent column) is bounded
//     by its surviving neighbours, so the envelope reflects what the bulk of
//     the column sounds like.
func renderWaveform(samples []float64, width, height int) []string {
	if width <= 0 || height <= 0 {
		return nil
	}
	grid := make([][]rune, height)
	for y := range grid {
		grid[y] = []rune(strings.Repeat(" ", width))
	}
	centre := (height - 1) / 2
	halfHeight := float64(height-1) / 2
	for x := range width {
		col := columnSamples(samples, x, width)
		clamped := percentileClamp(col)
		gated := noiseGate(clamped, waveformNoiseFloor)
		var hi, lo float64
		for _, v := range gated {
			if v > hi {
				hi = v
			}
			if v < lo {
				lo = v
			}
		}
		top := max(0, min(height-1, centre-int(math.Round(hi*halfHeight))))
		bottom := max(0, min(height-1, centre-int(math.Round(lo*halfHeight))))
		for y := top; y <= bottom; y++ {
			grid[y][x] = '│'
		}
	}
	lines := make([]string, height)
	for y, row := range grid {
		lines[y] = playingStyle.Render(string(row))
	}
	return lines
}

// columnSamples returns the slice of `samples` that map to column `x` of a
// waveform of the given width. The mapping mirrors what renderWaveform
// was doing before: each column covers a contiguous slice whose bounds
// round in the same way.
func columnSamples(samples []float64, x, width int) []float64 {
	if len(samples) == 0 {
		return nil
	}
	start := x * len(samples) / width
	end := max(start+1, (x+1)*len(samples)/width)
	if end > len(samples) {
		end = len(samples)
	}
	return samples[start:end]
}

// percentileClamp drops the top and bottom 10% of the column's samples
// (at least one from each end) before returning what's left, so a lone
// spike cannot stretch the envelope to full height. Trimming the signed
// extremes, not by magnitude, keeps symmetric audio symmetric. Columns
// of length two or less are returned unchanged: every sample is needed.
func percentileClamp(col []float64) []float64 {
	n := len(col)
	if n <= 2 {
		return col
	}
	cp := make([]float64, n)
	copy(cp, col)
	const dropFraction = 0.10
	drop := max(1, int(float64(n)*dropFraction))
	sort.Float64s(cp)
	return cp[drop : n-drop]
}

// noiseGate zeros every sample below the floor. Applied after the
// percentile clamp so the clamp doesn't drag a low outlier upward by
// re-ranking it next to its silent neighbours.
func noiseGate(samples []float64, floor float64) []float64 {
	out := make([]float64, len(samples))
	for i, v := range samples {
		if v > -floor && v < floor {
			out[i] = 0
		} else {
			out[i] = v
		}
	}
	return out
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (m Model) nowLines() []string {
	if m.now.Title == "" {
		meta := []string{" " + dimStyle.Render("♪ Nothing playing")}
		if m.artworkShown() {
			return m.artBand(meta)
		}
		return meta
	}
	// Glyphs here must be one column in every terminal: ▶ and ⏸ have emoji
	// forms some terminals draw two wide, which wraps lines and garbles the
	// frame.
	icon := playingStyle.Render("▸ ")
	if !m.now.Playing {
		icon = accentStyle.Render("‖ ")
	}
	meta := []string{" " + icon + textStyle.Bold(true).Render(displayText(m.now.Title))}
	var details []string
	if m.now.ArtistName != "" {
		details = append(details, textStyle.Render(displayText(m.now.ArtistName)))
	}
	if m.now.AlbumName != "" {
		details = append(details, dimStyle.Render(displayText(m.now.AlbumName)))
	}
	if len(details) > 0 {
		meta = append(meta, "   "+strings.Join(details, dimStyle.Render(" · ")))
	}

	if m.artworkShown() {
		return m.artBand(meta)
	}
	return meta
}

// artworkShown reports whether the artwork band fits. Reserve it even while
// metadata or a cover is loading, so track changes cannot resize the visualizer.
func (m Model) artworkShown() bool {
	// Keep the seek bar, tabs, footer and at least three browsing rows visible.
	w := max(1, m.width)
	cols, rows := m.artworkBandSize()
	minHeight := rows + 6 + len(m.seekLines(w)) + len(m.footerLines(m.bottomWidth())) + btoi(m.bottomBordered())
	return m.artwork &&
		m.width >= cols+24 && m.height >= minHeight
}

func (m Model) artworkBandSize() (cols, rows int) {
	if artworkMode == artKitty {
		return m.artworkSize() // high-resolution PNG contains its border
	}
	return artworkBandCols, artworkBandRows
}

func (m Model) artworkSize() (cols, rows int) {
	if m.coverCols > 0 && m.coverRows > 0 {
		return m.coverCols, m.coverRows
	}
	return artworkCols, artworkRows
}

// artBand lays the cover on the left and the metadata on the right, with the
// lyrics in a boxed block directly under the song and artist, for as many rows
// as the cover spans.
func (m Model) artBand(meta []string) []string {
	art := m.coverLines
	if m.now.Title == "" {
		art = nil
	}
	artW, rows := m.artworkBandSize()
	const gap = 2
	if artworkMode != artKitty {
		art = panelBox("", art, artW, rows)
	}
	rightW := m.width - artW - gap

	metaRows := len(meta) // 1 (title) or 2 (title + artist/album)
	lyricsRows := rows - metaRows
	var lyricsBox []string
	if m.lyricsOn && !m.lyricsHidden && lyricsRows >= 2 {
		body := m.lyricsLines(max(0, rightW-2), lyricsRows-2)
		lyricsBox = panelBox("Lyrics", body, rightW, lyricsRows)
	}

	out := make([]string, rows)
	for i := range rows {
		cover := ""
		if i < len(art) {
			cover = art[i]
		}
		line := pad(cover, artW) + strings.Repeat(" ", gap)
		switch {
		case i < metaRows:
			line += pad(meta[i], rightW)
		case i-metaRows < len(lyricsBox):
			line += lyricsBox[i-metaRows]
		default:
			line += strings.Repeat(" ", rightW)
		}
		out[i] = line
	}
	return out
}

func (m Model) seekLine(w int) string {
	lines := m.seekLines(w)
	return lines[len(lines)/2]
}

func (m Model) seekLines(w int) []string {
	pos := m.livePos()
	right := dimStyle.Render(fmt.Sprintf("  %s / %s ", clock(pos), clock(m.now.Duration)))
	shuffle := faintStyle.Render("Shuffle")
	if m.now.ShuffleMode == "on" {
		shuffle = accentStyle.Render("Shuffle")
	}
	repeat := "Repeat"
	if m.now.RepeatMode == "one" {
		repeat += " 1"
	}
	if m.now.RepeatMode == "one" || m.now.RepeatMode == "all" {
		repeat = accentStyle.Render(repeat)
	} else {
		repeat = faintStyle.Render(repeat)
	}
	right += shuffle + " " + repeat + " "
	// One cell of outer padding on both sides, plus the two border columns.
	barW := w - 3 - ansi.StringWidth(right)
	if barW < 4 {
		return []string{pad(" "+right, max(0, w))}
	}
	progress := strings.Repeat(" ", barW)
	if m.now.Duration > 0 {
		progress = seekProgress(pos/m.now.Duration, barW)
	}
	return []string{
		" " + dimStyle.Render("[") + progress + dimStyle.Render("]") + right,
	}
}

// seekProgress keeps the track thin, with a half-cell tip for interpolated time.
func seekProgress(progress float64, w int) string {
	if w <= 0 {
		return ""
	}
	units := int(min(1, max(0, progress)) * float64(w) * 2)
	full, fraction := units/2, units%2
	filled := strings.Repeat("━", full)
	if fraction > 0 {
		filled += "╸"
	}
	return accentStyle.Render(filled) + strings.Repeat(" ", w-full-btoi(fraction > 0))
}

func (m Model) tabBorder(w int) string {
	label := ansi.Truncate(m.tabLine(), max(0, w-4), "…")
	return dimStyle.Render("╭─") + label +
		dimStyle.Render(" "+strings.Repeat("─", max(0, w-4-ansi.StringWidth(label)))+"╮")
}

func (m Model) footerBorder(w int) string {
	footer := m.footerLines(m.bottomWidth())
	if len(footer) == 0 {
		return dimStyle.Render("╰" + strings.Repeat("─", w-2) + "╯")
	}
	label := ansi.Truncate(strings.TrimPrefix(footer[0], " "), max(0, w-6), "…")
	return dimStyle.Render("╰─ ") + label +
		dimStyle.Render(" "+strings.Repeat("─", w-5-ansi.StringWidth(label))+"╯")
}

func (m Model) tabLine() string {
	var b strings.Builder
	b.WriteByte(' ')
	for i, name := range tabNames {
		if Tab(i) == m.tab {
			b.WriteString(activeTab.Render(" " + name + " "))
		} else {
			b.WriteString(tabStyle.Render(" " + name + " "))
		}
	}
	b.WriteString("  ")
	if m.detail {
		b.WriteString(accentStyle.Render("‹ " + displayText(m.detailTitle)))
		if m.artistType != "" && m.artistType != "library-artists" {
			view := "albums"
			if m.artistSongs {
				view = "top songs"
			}
			b.WriteString(dimStyle.Render(" · " + view))
		}
		b.WriteString("  ")
	}
	filter := m.active().filter
	switch {
	case m.input:
		label := map[scope]string{catalogScope: "Apple Music", libraryScope: "library", filterScope: "filter"}[m.scope()]
		if m.scope() == catalogScope {
			label += " · " + libraryKindNames[m.searchKind]
		}
		b.WriteString(dimStyle.Render(label+" / ") + textStyle.Render(m.query) + accentStyle.Render("▌"))
	case filter != "":
		b.WriteString(dimStyle.Render("filter ") + textStyle.Render(filter) + faintStyle.Render("  esc clear"))
	case m.detail:
		b.WriteString(faintStyle.Render("esc back"))
	case m.tab == Search:
		next := strings.ToLower(libraryKindNames[(m.searchKind+1)%3])
		b.WriteString(dimStyle.Render(libraryKindNames[m.searchKind]) + faintStyle.Render(" · l "+next))
		if m.searchTerm != "" {
			b.WriteString(dimStyle.Render(" · / ") + textStyle.Render(m.searchTerm))
		}
	case m.tab == Library && m.libraryTerm != "":
		b.WriteString(dimStyle.Render("search ") + textStyle.Render(m.libraryTerm) + faintStyle.Render("  esc clear"))
	case m.tab == Library:
		next := strings.ToLower(libraryKindNames[(m.libraryKind+1)%3])
		b.WriteString(dimStyle.Render(libraryKindNames[m.libraryKind]) +
			faintStyle.Render(" · l "+next+" · o ") + dimStyle.Render(orderLabel[m.order[Library]]))
	case m.tab == Playlists:
		b.WriteString(faintStyle.Render("o ") + dimStyle.Render(orderLabel[m.order[Playlists]]))
	}
	return b.String()
}

// typeGlyph maps a catalog result type to a compact marker so Search rows
// distinguish albums, artists and playlists at a glance. Songs carry no marker
// (they are the default). Colour reinforces the letter/shape and stays on the
// terminal palette, so it follows theme changes.
func typeGlyph(t string) (string, lipgloss.Style) {
	switch {
	case strings.Contains(t, "artist"):
		return "*", artistStyle
	case strings.Contains(t, "album"):
		return "A", albumStyle
	case strings.Contains(t, "playlist"):
		return "P", playlistStyle
	}
	return "", songStyle
}

// currentQueueID is the queue entry playing now. Cider's queue carries no
// current flag, but its entry IDs match now-playing's. It is "" when no
// single row can be identified (e.g. the playing song is queued twice).
func (m Model) currentQueueID() string {
	rows := m.lists[Queue].rows
	if i, ok := playingIndex(rows, m.queueCurrent, m.now.ID); ok {
		return rows[i].ID
	}
	return ""
}

// rowSubtitle returns the metadata that goes in the right-hand column
// of a bottom-panel row: "Artist" alone, or "Artist · Album" when the
// album context isn't already supplied by the panel.
//
// Rules:
//   - Always show the artist. If subtitle is empty, the field is left
//     blank — we never invent artists from unrelated fields.
//   - Show the album when the row IS a song (queue, search songs, library
//     songs, playlist tracks, artist top-songs). The row's Album field is
//     the source.
//   - Suppress the album when the row IS the container being browsed
//     (Library albums/artists/playlists, Playlists tab, album tracks
//     drilled into from an album). The detail header or the row's Title
//     already names the container, so repeating the album is noise.
func rowSubtitle(item Item, m Model) string {
	artist := strings.TrimSpace(displayText(item.Subtitle))
	album := ""
	if shouldShowAlbum(item, m) {
		album = strings.TrimSpace(displayText(item.Album))
	}
	switch {
	case artist != "" && album != "":
		return artist + " · " + album
	case artist != "":
		return artist
	case album != "":
		return album
	}
	return ""
}

// shouldShowAlbum reports whether the bottom-panel row should display
// the album field next to the artist. Suppressed when the row itself is
// the album/artist/playlist being browsed, or when the detail header
// already names the album (album-tracks drill-down).
func shouldShowAlbum(item Item, m Model) bool {
	if item.Album == "" {
		return false
	}
	// Album tracks inside an album detail view: the detail header
	// already names the album, so repeating it adds nothing.
	if m.detail && (m.detailKind == Library || m.detailKind == Playlists) {
		// Drill-down from Library/Playlists always names the parent.
		// Library → album tracks, Playlists → playlist tracks. Both
		// show the parent name in the tab border; album is noise on
		// album tracks but the playlist's name is not the album of its
		// tracks, so the album is still useful there.
		if m.detailKind == Library {
			return false
		}
	}
	t := item.Type
	switch t {
	case "albums", "library-albums":
		return false
	case "artists", "library-artists":
		return false
	case "playlists", "library-playlists":
		return false
	}
	return true
}

func (m Model) listLines(w, n int) []string {
	if n <= 0 {
		return nil
	}
	l := m.active()
	visible := l.shown()
	loading := l.loading || (m.tab == Queue && !m.detail && m.queueLoading)
	var details []string
	if m.expanded && l.cursor < len(visible) {
		details = m.itemDetailsLines(visible[l.cursor], w, max(0, n-1-btoi(loading)))
	}
	shown := n - len(details)
	if loading {
		shown = max(0, shown-1)
	}
	titleW := max(8, (w-4)*3/5)
	artistW := max(0, w-4-titleW-2)
	rows := make([]string, 0, n)
	start := max(0, l.cursor-shown+1)
	for i := start; i < min(len(visible), start+shown); i++ {
		v := visible[i]
		mark, title, sub := "  ", textStyle, dimStyle
		if m.tab == Queue && !m.detail && v.ID != "" && v.ID == m.currentQueueID() {
			mark, title = playingStyle.Render("▸ "), playingStyle
		}
		if i == l.cursor {
			if mark == "  " {
				mark = selectedStyle.Render("> ")
			} else {
				mark = selectedStyle.Render("▸ ") // keep the playing marker under the cursor
			}
			title = selectedStyle
		}
		tag := ""
		if m.tab == Search {
			if g, st := typeGlyph(v.Type); g != "" {
				tag = st.Render(g) + " "
			}
		}
		tw := titleW
		if tag != "" {
			tw -= 2 // the glyph plus its trailing space
		}
		// Compact metadata: subtitle becomes "Artist" or "Artist · Album"
		// depending on row type and panel context. Album is suppressed
		// when the row IS the album/artist/playlist being browsed (the
		// detail header already names it) and shown when the row is a
		// song inside a parent container (queue, search, library songs,
		// playlist tracks, artist top-songs).
		subText := rowSubtitle(v, m)
		rows = append(rows, " "+mark+tag+title.Render(pad(displayText(v.Title), tw))+"  "+sub.Render(ansi.Truncate(subText, artistW, "…")))
		if i == l.cursor {
			rows = append(rows, details...)
		}
	}
	switch {
	case loading:
		rows = append(rows, faintStyle.Render("   Loading…"))
	case len(rows) == 0 && m.tab == Queue && !m.detail && !m.queueRetryAt.IsZero():
		rows = append(rows, faintStyle.Render("   Queue unavailable · refresh failed"))
	case len(rows) == 0 && l.filter != "":
		rows = append(rows, faintStyle.Render("   No matches"))
	case len(rows) == 0 && m.tab == Search:
		rows = append(rows, faintStyle.Render("   Press / to search Apple Music"))
	case len(rows) == 0:
		rows = append(rows, faintStyle.Render("   Nothing here"))
	}
	for len(rows) < n {
		rows = append(rows, "")
	}
	return rows[:n]
}

// footerLines shows essential contextual controls; ? lists every key.
func (m Model) footerLines(w int) []string {
	if m.status != "" {
		return []string{" " + errorStyle.Render(displayText(m.status))}
	}
	if m.notice != "" && time.Now().Before(m.noticeUntil) {
		return []string{" " + playingStyle.Render("✓ "+displayText(m.notice))}
	}
	var hints [][2]string
	switch {
	case m.settings:
		hints = [][2]string{{"↑↓", "select"}, {"↵", "apply / toggle"}, {"esc", "back"}, {"q", "quit"}}
	case m.input && m.scope() == filterScope:
		hints = [][2]string{{"↵", "done"}, {"esc", "clear"}}
	case m.input:
		// While typing into the search/filter field, ? and q are typed
		// into the query, not bound to "keys" or "quit". Don't advertise
		// them; the only global escape is esc.
		hints = [][2]string{{"↵", "search"}, {"esc", "cancel"}}
	case m.help:
		hints = [][2]string{{"?", "close"}, {"q", "quit"}}
	default:
		hints = [][2]string{{"/", "search"}}
		queueFiltered := m.tab == Queue && !m.detail && m.active().filter != ""
		switch {
		case queueFiltered:
			// Cider's queue endpoints index by absolute position, which a
			// filtered view doesn't preserve; jump/remove/clear all become
			// no-ops while the filter is on. Surface the restriction so the
			// user knows esc is the way out, and drop the now-dead hints.
			hints = append(hints, [2]string{"esc", "clear filter"})
		case m.tab == Queue && !m.detail:
			hints = append(hints, [2]string{"↵", "jump"}, [2]string{"x", "remove"}, [2]string{"c", "clear"})
		default:
			hints = append(hints, [2]string{"↵", "play / open"}, [2]string{"a", "enqueue"})
		}
		hints = append(hints, [2]string{"d", "details"}, [2]string{"␣", "play/pause"}, [2]string{"n/p", "skip"})
		if m.detail {
			if m.artistType != "" && m.artistType != "library-artists" {
				hints = append(hints, [2]string{"l", "albums / top songs"})
			}
			hints = append(hints, [2]string{"esc", "back"})
		}
		hints = append(hints, [2]string{"?", "keys"}, [2]string{"q", "quit"})
	}
	// A single line, dropping the least essential hints as width shrinks.
	// The default mode appends ? and q last; help/settings/input already
	// included them above. Avoid duplicating them at the front.
	essential := [][2]string{}
	for _, h := range hints {
		if h[0] == "?" || h[0] == "q" {
			essential = append(essential, h)
		}
	}
	alreadyEssential := map[string]bool{}
	for _, h := range essential {
		alreadyEssential[h[0]] = true
	}
	var rest [][2]string
	for _, h := range hints {
		if !alreadyEssential[h[0]] {
			rest = append(rest, h)
		}
	}
	var b strings.Builder
	write := func(h [2]string) bool {
		item := keyStyle.Render(h[0]) + dimStyle.Render(" "+h[1])
		sep := "  "
		if b.Len() == 0 {
			sep = " "
		}
		if b.Len() > 0 && ansi.StringWidth(b.String()+sep+item) > w {
			return false
		}
		b.WriteString(sep)
		b.WriteString(item)
		return true
	}
	for _, h := range essential {
		write(h)
	}
	for _, h := range rest {
		if !write(h) {
			break
		}
	}
	if b.Len() == 0 {
		return []string{}
	}
	return []string{ansi.Truncate(b.String(), w, "…")}
}

var helpKeys = [][2]string{
	{"␣", "play / pause"}, {"n  p", "next / previous"}, {"←  →", "seek 5 s"}, {"+  -", "volume"},
	{"s", "toggle shuffle"}, {"r", "cycle repeat"}, {"P", "play all"}, {"↵", "play / open / jump"},
	{"a", "enqueue (outside Queue)"}, {"x", "remove (Queue tab)"}, {"c", "clear (Queue tab)"}, {"Y", "copy item to clipboard"}, {"/", "search / filter"}, {"o", "sort"},
	{"d", "expand item details"},
	{"l", "songs / albums / artists"}, {"⇥  ⇧⇥", "next / prev tab"}, {"1-4", "go to tab"}, {"j  k", "move"}, {"esc", "back / clear"},
	{"y", "hide / restore lyrics"}, {",", "settings"}, {"v", "full-screen visualizer"}, {"b", "spectrum / waveform"}, {"q", "quit"},
}

// helpLines lays the key list out in two columns when there is room.
func helpLines(w, n int) []string {
	cols := 1
	if w >= 64 {
		cols = 2
	}
	per := (len(helpKeys) + cols - 1) / cols
	colW := (w - 2) / cols
	lines := make([]string, 0, n)
	for row := 0; row < per && len(lines) < n; row++ {
		line := " "
		for c := range cols {
			if i := c*per + row; i < len(helpKeys) {
				line += pad(keyStyle.Render(fmt.Sprintf(" %-8s", helpKeys[i][0]))+dimStyle.Render(helpKeys[i][1]), colW)
			}
		}
		lines = append(lines, line)
	}
	for len(lines) < n {
		lines = append(lines, "")
	}
	return lines
}

// lyricsLines shows synced lyrics with the current line centred: sung lines
// grey, the current line bold accent (word by word when Apple times words),
// upcoming lines in the normal colour. Lyric lines wrap within the panel.
func (m Model) lyricsLines(w, n int) []string {
	out := make([]string, n)
	if n == 0 {
		return out
	}
	msg := ""
	switch {
	case m.now.ID == "":
		msg = "Nothing playing"
	case m.lyricsLoading:
		msg = "Loading lyrics…"
	case m.lyricsErr != nil || len(m.lyrics) == 0:
		msg = "No lyrics for this song"
	}
	if msg != "" {
		out[n/2] = faintStyle.Render("    " + msg)
		return out
	}
	pos := m.livePos()
	cur := currentLine(m.lyrics, pos)
	if m.lyrics[len(m.lyrics)-1].start == 0 { // unsynced: show from the top
		cur = -1
	}
	var rows []string
	currentRow := 0
	for i, l := range m.lyrics {
		if i == max(cur, 0) {
			currentRow = len(rows)
		}
		prefix := "    "
		var content string
		style := textStyle
		switch {
		case i < cur:
			style = dimStyle
		case i == cur && len(l.words) > 0:
			var b strings.Builder
			prefix = "  ▸ "
			for j, word := range l.words {
				if j > 0 {
					b.WriteByte(' ')
				}
				end := l.end
				if j+1 < len(l.words) {
					end = l.words[j+1].start
				}
				b.WriteString(liveWord(word, end, pos))
			}
			content = b.String()
		case i == cur:
			prefix = "  ▸ "
			style = selectedStyle
		}
		// Word-wrap with a 1-col right margin; overlong words still hard-break.
		// Whole-line styles are applied per row: a style opened once would be
		// reset by the next row's indent. Timed words carry their own styles.
		styled := content != ""
		if !styled {
			content = displayText(l.text)
		}
		wrapped := ansi.Wrap(content, max(1, w-5), "")
		for j, part := range strings.Split(wrapped, "\n") {
			indent := "    "
			if j == 0 {
				indent = prefix
			}
			if i == cur {
				indent = selectedStyle.Render(indent)
			}
			if !styled {
				part = style.Render(part)
			}
			rows = append(rows, indent+part)
		}
	}
	top := currentRow - n/2
	for row := range out {
		if i := top + row; i >= 0 && i < len(rows) {
			out[row] = rows[i]
		}
	}
	return out
}

func (m Model) offlineLines(w, h int) []string {
	block := []string{
		errorStyle.Render("Cider isn't running (or could not be reached)"),
		dimStyle.Render("Retrying every 2 s…"),
		faintStyle.Render(m.status),
		"",
		keyStyle.Render(" r ") + dimStyle.Render(" retry now ") +
			keyStyle.Render(" q ") + dimStyle.Render(" quit"),
	}
	lines := make([]string, max(0, (h-len(block))/2))
	for _, s := range block {
		lines = append(lines, lipgloss.PlaceHorizontal(w, lipgloss.Center, s))
	}
	return lines
}
