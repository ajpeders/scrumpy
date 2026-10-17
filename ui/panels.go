package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// menuLines keeps browsing available while lyrics are visible.
// Narrow terminals stack the panels; very short views preserve the menu.
func (m Model) menuLines(w, n int) []string {
	if n < 3 || w < 12 {
		return m.listLines(w, n)
	}
	if w < 64 {
		if n < 6 {
			return m.listLines(w, n)
		}
		panelH := min(n/2, 10)
		return append(m.listLines(w, n-panelH), m.sidePanel(w, panelH)...)
	}
	// The lyrics sidebar takes most of the width, so it grows with the terminal
	// instead of freezing at a narrow column. The cap keeps a comfortable
	// reading measure on very wide terminals and leaves the list usable.
	panelW := min(100, max(24, w*11/20))
	menuW := w - panelW - 1
	left, right := m.listLines(menuW, n), m.sidePanel(panelW, n)
	for i := range left {
		left[i] = pad(left[i], menuW) + " " + right[i]
	}
	return left
}

func (m Model) sidePanel(w, n int) []string {
	return panelBox("Lyrics", m.lyricsLines(max(0, w-2), max(0, n-2)), w, n)
}

func panelBox(title string, body []string, w, n int) []string {
	if n < 2 || w < 4 {
		return make([]string, max(0, n))
	}
	inner := w - 2
	top := ansi.Truncate("─ "+title+" "+strings.Repeat("─", w), inner, "")
	out := []string{dimStyle.Render("╭" + top + "╮")}
	for i := range n - 2 {
		line := ""
		if i < len(body) {
			line = body[i]
		}
		out = append(out, dimStyle.Render("│")+pad(line, inner)+dimStyle.Render("│"))
	}
	return append(out, dimStyle.Render("╰"+strings.Repeat("─", inner)+"╯"))
}

func (m Model) settingsLines(w, n int) []string {
	rows := make([]string, 0, n)
	if n <= 0 {
		return rows
	}
	rows = append(rows, titleStyle.Render(" Settings · appearance"))
	options := append([]string(nil), themeNames...)
	background := "Off (use terminal)"
	if m.backgroundOn {
		background = "On (use theme)"
	}
	options = append(options, "Background: "+background)
	visualizer := "Spectrum"
	if m.waveform {
		visualizer = "Waveform"
	}
	options = append(options, "Visualizer: "+visualizer)
	start := max(0, m.themeCursor-max(0, n-2))
	for i := start; i < len(options) && len(rows) < n; i++ {
		name := options[i]
		label := "   " + name
		if name == m.theme {
			label += " (active)"
		}
		if i == m.themeCursor {
			label = selectedStyle.Render(" > " + strings.TrimSpace(label))
		}
		rows = append(rows, ansi.Truncate(label, w, "…"))
	}
	for len(rows) < n {
		rows = append(rows, "")
	}
	return rows
}

// itemDetailsLines expands the metadata already available for the selected
// item. It follows keyboard selection without network requests or playback.
func (m Model) itemDetailsLines(item Item, w, n int) []string {
	if w < 8 || n == 0 {
		return nil
	}
	fields := [][2]string{{"Title", item.Title}, {"Artist / info", item.Subtitle}, {"Type", item.Type}, {"ID", item.ID}}
	if item.ID != "" && item.ID == m.now.ID {
		fields = append(fields, [2]string{"Album", m.now.AlbumName})
		if m.now.Duration > 0 {
			fields = append(fields, [2]string{"Duration", clock(m.now.Duration)})
		}
	}
	var rows []string
	for _, field := range fields {
		if field[1] == "" {
			continue
		}
		label := field[0] + ": " + displayText(field[1])
		for _, line := range strings.Split(ansi.Hardwrap(label, w-6, true), "\n") {
			rows = append(rows, dimStyle.Render("   │ ")+textStyle.Render(line))
			if len(rows) == n {
				return rows
			}
		}
	}
	return rows
}

// liveWord progressively highlights a timed word rather than lighting the
// entire word as soon as it starts. Cell-based cuts also handle wide text.
func liveWord(word lyricWord, end, pos float64) string {
	text := strings.TrimSpace(displayText(word.text))
	if pos < word.start {
		return textStyle.Render(text)
	}
	if end <= word.start || pos >= end {
		return selectedStyle.Render(text)
	}
	width := ansi.StringWidth(text)
	filled := int(float64(width) * (pos - word.start) / (end - word.start))
	return selectedStyle.Render(ansi.Cut(text, 0, filled)) + textStyle.Render(ansi.Cut(text, filled, width))
}
