package ui

import (
	"encoding/xml"
	"errors"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// lyricWord is a timed word (or syllable) of a line; only "Word"-timed TTML
// has them.
type lyricWord struct {
	start float64
	text  string
}

type lyricLine struct {
	start, end float64 // seconds; both 0 for unsynced lyrics
	text       string
	words      []lyricWord
}

// parseTTML reads Apple Music's lyrics TTML: each <p> is a line with begin/end
// times, and word-timed lyrics wrap words in <span begin=…>.
func parseTTML(ttml string) ([]lyricLine, error) {
	dec := xml.NewDecoder(strings.NewReader(ttml))
	var lines []lyricLine
	var line *lyricLine
	var word *lyricWord
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				line = &lyricLine{start: ttmlTime(attr(t, "begin")), end: ttmlTime(attr(t, "end"))}
			case "span":
				if line != nil && attr(t, "begin") != "" {
					word = &lyricWord{start: ttmlTime(attr(t, "begin"))}
				}
			}
		case xml.CharData:
			if line != nil {
				line.text += string(t)
				if word != nil {
					word.text += string(t)
				}
			}
		case xml.EndElement:
			switch {
			case t.Name.Local == "span" && word != nil:
				line.words = append(line.words, *word)
				word = nil
			case t.Name.Local == "p" && line != nil:
				line.text = strings.Join(strings.Fields(line.text), " ")
				lines = append(lines, *line)
				line = nil
			}
		}
	}
	if len(lines) == 0 {
		return nil, errors.New("no lyric lines")
	}
	return lines, nil
}

func attr(e xml.StartElement, name string) string {
	for _, a := range e.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// ttmlTime parses "12.3", "1:02.345" or "1:02:03.4" (optionally with an "s"
// suffix) into seconds; anything else is 0.
func ttmlTime(s string) float64 {
	var total float64
	for _, part := range strings.Split(strings.TrimSuffix(s, "s"), ":") {
		v, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return 0
		}
		total = total*60 + v
	}
	return total
}

// currentLine is the index of the last line started at pos, or -1 before the
// first line.
func currentLine(lines []lyricLine, pos float64) int {
	cur := -1
	for i, l := range lines {
		if l.start <= pos && (l.start > 0 || i == 0) {
			cur = i
		}
	}
	return cur
}

type lyricsMsg struct {
	trackID string
	lines   []lyricLine
	err     error
}

// A dedicated clock keeps lyrics animating even without a spectrum stream.
// Init starts one chain, independent of playback polls.
type lyricsFrameMsg struct{}

func lyricsFrame() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(time.Time) tea.Msg { return lyricsFrameMsg{} })
}

// fetchLyrics gets Apple's TTML through Cider's API proxy. It needs the
// song's catalog ID; library IDs have no lyrics resource.
func fetchLyrics(c Client, trackID, catalogID string) tea.Cmd {
	return func() tea.Msg {
		sf, err := storefront(c)
		if err != nil {
			return lyricsMsg{trackID: trackID, err: err}
		}
		var resp struct {
			Data []struct {
				Attributes struct {
					TTML string `json:"ttml"`
				} `json:"attributes"`
			} `json:"data"`
		}
		path := "/v1/catalog/" + url.PathEscape(sf) + "/songs/" + url.PathEscape(catalogID) + "/lyrics"
		if err := c.AMAPI(path, &resp); err != nil {
			return lyricsMsg{trackID: trackID, err: err}
		}
		if len(resp.Data) == 0 {
			return lyricsMsg{trackID: trackID, err: errors.New("no lyrics")}
		}
		lines, err := parseTTML(resp.Data[0].Attributes.TTML)
		return lyricsMsg{trackID: trackID, lines: lines, err: err}
	}
}

func isCatalogID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
