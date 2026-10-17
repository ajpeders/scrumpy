package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// menuLines fills the bottom panel with the active list. Lyrics live in their
// own band below the visualizer (or inside the art band when artwork is on);
// the bottom panel stays a clean view of the queue/queue/library/etc.
func (m Model) menuLines(w, n int) []string {
	return m.listLines(w, n)
}

func (m Model) sidePanel(w, n int) []string {
	if !m.lyricsOn {
		return make([]string, max(0, n))
	}
	return panelBox("Lyrics", m.lyricsLines(max(0, w-2), max(0, n-2)), w, n)
}

// detailsPanelCoverMinW is the minimum panel width for the d details box to
// include a left-side cover. Below it the cover column is dropped to leave a
// usable metadata measure.
const detailsPanelCoverMinW = 44

func panelBox(title string, body []string, w, n int) []string {
	if n < 2 || w < 4 {
		return make([]string, max(0, n))
	}
	inner := w - 2
	top := strings.Repeat("─", inner)
	if title != "" {
		top = ansi.Truncate("─ "+title+" "+strings.Repeat("─", w), inner, "")
	}
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
	artwork := "Off"
	if m.artwork {
		artwork = "On"
	}
	options = append(options, "Album art: "+artwork)
	lyrics := "Off"
	if m.lyricsOn {
		lyrics = "On"
	}
	options = append(options, "Lyrics: "+lyrics)
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
	if w < 18 || n < 4 {
		return nil
	}
	if item.ID != "" && item.ID == m.now.ID {
		if item.Album == "" {
			item.Album = m.now.AlbumName
		}
		if item.Duration <= 0 {
			item.Duration = m.now.Duration
		}
		if item.Subtitle == "" {
			item.Subtitle = m.now.ArtistName
		}
	}
	fields := [][2]string{{"Title", item.Title}}
	if isArtist(item) {
		if item.Genre == "" {
			item.Genre = item.Subtitle
		}
	} else {
		fields = append(fields, [2]string{"Artist", item.Subtitle})
	}
	fields = append(fields, [2]string{"Album", item.Album})
	if item.Duration > 0 {
		fields = append(fields, [2]string{"Duration", clock(item.Duration)})
	}
	fields = append(fields, [2]string{"Genre", item.Genre})
	released := item.ReleaseDate
	if date, err := time.Parse(time.RFC3339, released); err == nil {
		released = date.Format("2006-01-02")
	}
	fields = append(fields, [2]string{"Released", released})
	if item.TrackNumber > 0 {
		fields = append(fields, [2]string{"Track", fmt.Sprint(item.TrackNumber)})
	}
	if item.DiscNumber > 0 {
		fields = append(fields, [2]string{"Disc", fmt.Sprint(item.DiscNumber)})
	}
	kind := strings.TrimPrefix(item.Type, "library-")
	for _, resource := range []string{"song", "album", "artist", "playlist"} {
		if kind == resource+"s" {
			kind = resource
		}
	}
	fields = append(fields, [2]string{"Type", kind}, [2]string{"ID", item.ID})
	cover := m.detailsCover
	hasCover := len(cover) > 0 && m.artwork && w >= detailsPanelCoverMinW
	hasCover = hasCover && artworkResource(item.Type) != ""
	var body []string
	coverRows := 0
	coverCols := 0
	if hasCover {
		coverRows = len(cover)
		coverCols = ansi.StringWidth(cover[0])
		if artworkMode == artKitty && m.detailsKitty != nil {
			// A kitty image paints all its cells whatever the text says, so
			// shrink it to the panel's body rows instead of spilling out.
			coverRows = min(artworkRows, n-2)
			coverCols = coverRows * artworkCols / artworkRows
			cover = make([]string, coverRows)
			if coverRows >= 3 {
				cover[0] = m.detailsKitty.get(detailsImageID, coverCols, coverRows)
			} else {
				hasCover = false
			}
		}
	}
	innerW := w - 2
	metaW := innerW
	if hasCover {
		metaW = innerW - coverCols - 1 // 1-cell gutter
	}
	for _, field := range fields {
		if field[1] == "" {
			continue
		}
		style := textStyle
		if field[0] == "Type" || field[0] == "ID" {
			style = faintStyle
		}
		for i, line := range strings.Split(ansi.Wrap(displayText(field[1]), metaW-11, ""), "\n") {
			label := strings.Repeat(" ", 11)
			if i == 0 {
				label = dimStyle.Render(fmt.Sprintf(" %-9s ", field[0]))
			}
			body = append(body, label+style.Render(line))
		}
	}
	if !hasCover {
		return panelBox("Details", body, w, min(n, len(body)+2))
	}
	// Compose the panel body so the cover sits on the left and metadata fills
	// the right column. The cover is `coverRows` tall in block mode and 1 row
	// in kitty mode (the image protocol paints over the cells below).
	total := max(coverRows, len(body))
	composed := make([]string, total)
	for i := range composed {
		left := strings.Repeat(" ", coverCols)
		if i < coverRows {
			left = pad(cover[i], coverCols)
		}
		right := ""
		if i < len(body) {
			right = pad(body[i], metaW)
		} else {
			right = strings.Repeat(" ", metaW)
		}
		composed[i] = left + " " + right
	}
	return panelBox("Details", composed, w, min(n, total+2))
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
