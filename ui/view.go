package ui

import (
	"fmt"
	"math"
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
	return 2 + len(m.nowLines()) + len(m.footerLines(max(1, m.width)))
}

func (m Model) vizAvailable() bool  { return (m.viz != nil || m.vizFrames != nil) && !m.vizClosed }
func (m Model) fullScreenViz() bool { return m.fullViz && m.vizAvailable() }

// layout gives the visualizer about 60% of the free rows and the list the rest
// (at least 3). Without a visualizer, or with too few rows for one, the list
// takes everything.
func (m Model) layout() (vizRows, listRows int) {
	free := m.height - m.fixedRows()
	if !m.fullScreenViz() && !m.lyricsExpanded {
		free-- // tab row
	}
	free = max(0, free)
	switch {
	case !m.vizAvailable():
		return 0, free
	case m.fullScreenViz():
		return free, 0
	}
	listRows = max(3, free*2/5)
	if m.settings || m.help {
		listRows = min(free, max(listRows, 12))
	}
	if vizRows = free - listRows; vizRows < 2 {
		return 0, free
	}
	return vizRows, listRows
}

func (m Model) View() string {
	w := max(1, m.width)
	lines := []string{m.headerLine(w)}
	if m.offline {
		lines = append(lines, m.offlineLines(w, m.height-1)...)
		return m.themedFrame(fit(lines, w, m.height), w)
	}
	vizRows, listRows := m.layout()
	if vizRows > 0 {
		if m.waveform {
			samples := m.samples
			if !m.now.Playing {
				samples = nil
			}
			lines = append(lines, renderWaveform(samples, w, vizRows)...)
		} else {
			lines = append(lines, renderSpectrum(m.meter.levels, m.meter.peaks, w, vizRows)...)
		}
	}
	lines = append(lines, m.nowLines()...)
	lines = append(lines, m.seekLine(w))
	if !m.fullScreenViz() {
		if !m.lyricsExpanded {
			lines = append(lines, m.tabLine())
		}
		switch {
		case m.settings:
			lines = append(lines, m.settingsLines(w, listRows)...)
		case m.help:
			lines = append(lines, helpLines(w, listRows)...)
		case m.lyricsExpanded:
			lines = append(lines, m.sidePanel(w, listRows)...)
		default:
			lines = append(lines, m.menuLines(w, listRows)...)
		}
	}
	lines = append(lines, m.footerLines(w)...)
	return m.themedFrame(fit(lines, w, m.height), w)
}

func (m Model) themedFrame(frame string, w int) string {
	style := lipgloss.NewStyle().Width(w)
	if m.backgroundOn && themeBackground != nil {
		style = style.Background(themeBackground)
	}
	if themeForeground != nil {
		style = style.Foreground(themeForeground)
	}
	return style.Render(frame)
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
	return spread(" "+titleStyle.Render("cider-tui"), right, w)
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

// renderWaveform draws the latest PCM hop as a vertical min/max envelope. Each
// column represents a time slice, preserving transients even on narrow screens.
func renderWaveform(samples []float64, width, height int) []string {
	if width <= 0 || height <= 0 {
		return nil
	}
	grid := make([][]rune, height)
	for y := range grid {
		grid[y] = []rune(strings.Repeat(" ", width))
	}
	centre := (height - 1) / 2
	for x := range width {
		lo, hi := 0.0, 0.0
		if len(samples) > 0 {
			start := x * len(samples) / width
			end := max(start+1, (x+1)*len(samples)/width)
			for i := start; i < min(end, len(samples)); i++ {
				lo = min(lo, samples[i])
				hi = max(hi, samples[i])
			}
		}
		top := max(0, min(height-1, centre-int(math.Round(hi*float64(height-1)/2))))
		bottom := max(0, min(height-1, centre-int(math.Round(lo*float64(height-1)/2))))
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

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (m Model) nowLines() []string {
	if m.now.Title == "" {
		return []string{" " + dimStyle.Render("♪ Nothing playing")}
	}
	// Glyphs here must be one column in every terminal: ▶ and ⏸ have emoji
	// forms some terminals draw two wide, which wraps lines and garbles the
	// frame.
	icon := playingStyle.Render("▸ ")
	if !m.now.Playing {
		icon = accentStyle.Render("‖ ")
	}
	title := " " + icon + textStyle.Bold(true).Render(displayText(m.now.Title))
	var details []string
	if m.now.ArtistName != "" {
		details = append(details, textStyle.Render(displayText(m.now.ArtistName)))
	}
	if m.now.AlbumName != "" {
		details = append(details, dimStyle.Render(displayText(m.now.AlbumName)))
	}
	if len(details) == 0 {
		return []string{title}
	}
	return []string{title, "   " + strings.Join(details, dimStyle.Render(" · "))}
}

func (m Model) seekLine(w int) string {
	right := dimStyle.Render(fmt.Sprintf("  %s / %s ", clock(m.now.Position), clock(m.now.Duration)))
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
	barW := w - 1 - ansi.StringWidth(right)
	if barW < 4 {
		return " " + right
	}
	if m.now.Duration <= 0 {
		return " " + faintStyle.Render(strings.Repeat("─", barW)) + right
	}
	head := int(min(1, max(0, m.now.Position/m.now.Duration)) * float64(barW-1))
	return " " + accentStyle.Render(strings.Repeat("━", head)+"●") +
		faintStyle.Render(strings.Repeat("─", barW-1-head)) + right
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
		b.WriteString(accentStyle.Render("‹ "+displayText(m.detailTitle)) + "  ")
	}
	filter := m.active().filter
	switch {
	case m.input:
		label := map[scope]string{catalogScope: "Apple Music", libraryScope: "library", filterScope: "filter"}[m.scope()]
		b.WriteString(dimStyle.Render(label+" / ") + textStyle.Render(m.query) + accentStyle.Render("▌"))
	case filter != "":
		b.WriteString(dimStyle.Render("filter ") + textStyle.Render(filter) + faintStyle.Render("  esc clear"))
	case m.detail:
		b.WriteString(faintStyle.Render("esc back"))
	case m.tab == Search && m.searchTerm != "":
		b.WriteString(dimStyle.Render("/ ") + textStyle.Render(m.searchTerm))
	case m.tab == Library && m.libraryFull != nil:
		b.WriteString(dimStyle.Render("search ") + textStyle.Render(m.libraryTerm) + faintStyle.Render("  esc clear"))
	case m.tab == Library && m.libraryAlbums:
		b.WriteString(dimStyle.Render("Albums") + faintStyle.Render(" · l songs · o ") + dimStyle.Render(orderLabel[m.order[Library]]))
	case m.tab == Library:
		b.WriteString(dimStyle.Render("Songs") + faintStyle.Render(" · l albums · o ") + dimStyle.Render(orderLabel[m.order[Library]]))
	case m.tab == Playlists:
		b.WriteString(faintStyle.Render("o ") + dimStyle.Render(orderLabel[m.order[Playlists]]))
	}
	return b.String()
}

// currentQueueID is the queue entry playing now. Cider's queue carries no
// current flag, but its entry IDs match now-playing's.
func (m Model) currentQueueID() string {
	if m.queueCurrent != "" {
		return m.queueCurrent
	}
	return m.now.ID
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
		details = m.itemDetailsLines(visible[l.cursor], w, max(0, n-1))
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
		rows = append(rows, " "+mark+title.Render(pad(displayText(v.Title), titleW))+"  "+sub.Render(ansi.Truncate(displayText(v.Subtitle), artistW, "…")))
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
		hints = [][2]string{{"↑↓", "select"}, {"↵", "apply / toggle"}, {"esc", "back"}}
	case m.input && m.scope() == filterScope:
		hints = [][2]string{{"↵", "done"}, {"esc", "clear"}}
	case m.input:
		hints = [][2]string{{"↵", "search"}, {"esc", "cancel"}}
	case m.help:
		hints = [][2]string{{"?", "close"}}
	case m.lyricsExpanded:
		hints = [][2]string{{"y/esc", "restore menu"}, {"␣", "play/pause"}, {"n/p", "skip"}, {"?", "keys"}, {"q", "quit"}}
	default:
		hints = [][2]string{{"/", "search"}}
		if m.tab == Queue && !m.detail {
			hints = append(hints, [2]string{"↵", "jump"}, [2]string{"x", "remove"}, [2]string{"c", "clear"})
		} else {
			hints = append(hints, [2]string{"↵", "play / open"}, [2]string{"a", "enqueue"})
		}
		hints = append(hints, [2]string{"d", "details"}, [2]string{"␣", "play/pause"}, [2]string{"n/p", "skip"})
		if m.detail {
			hints = append(hints, [2]string{"esc", "back"})
		}
		hints = append(hints, [2]string{"?", "keys"}, [2]string{"q", "quit"})
	}
	var lines []string
	line := ""
	for _, h := range hints {
		item := "  " + keyStyle.Render(h[0]) + dimStyle.Render(" "+h[1])
		if line != "" && ansi.StringWidth(line+item) > w {
			lines = append(lines, line)
			line = ""
		}
		line += item
	}
	return append(lines, line)
}

var helpKeys = [][2]string{
	{"␣", "play / pause"}, {"n  p", "next / previous"}, {"←  →", "seek 5 s"}, {"+  -", "volume"},
	{"s", "toggle shuffle"}, {"r", "cycle repeat"}, {"P", "play all"}, {"↵", "play / open / jump"},
	{"a", "enqueue (outside Queue)"}, {"x", "remove (Queue tab)"}, {"c", "clear (Queue tab)"}, {"/", "search / filter"}, {"o", "sort"},
	{"d", "expand item details"},
	{"l", "songs / albums"}, {"⇥  ⇧⇥", "next / prev tab"}, {"1-4", "go to tab"}, {"j  k", "move"}, {"esc", "back / clear"},
	{"y", "expand / restore lyrics"}, {",", "settings"}, {"v", "full-screen visualizer"}, {"b", "spectrum / waveform"}, {"q", "quit"},
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
		keyStyle.Render(" q ") + dimStyle.Render(" quit"),
	}
	lines := make([]string, max(0, (h-len(block))/2))
	for _, s := range block {
		lines = append(lines, lipgloss.PlaceHorizontal(w, lipgloss.Center, s))
	}
	return lines
}
