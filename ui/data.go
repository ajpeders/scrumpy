package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// storefront reads the signed-in user's Apple Music storefront so catalog
// searches use the correct region rather than a hardcoded one.
func storefront(c Client) (string, error) {
	var response struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := c.AMAPI("/v1/me/storefront", &response); err != nil {
		return "", err
	}
	if len(response.Data) == 0 || response.Data[0].ID == "" {
		return "", errors.New("no Apple Music storefront")
	}
	return response.Data[0].ID, nil
}

// Cider's queue is a bare array, unlike the other playback GET responses.
func parseQueue(raw json.RawMessage) ([]Item, string, error) {
	value := strings.TrimSpace(string(raw))
	if value == "false" || value == "true" || value == "null" {
		return nil, "", errors.New("Cider's queue API returned no queue list (playback may still be connected)")
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, "", err
	}
	rows := make([]Item, 0, len(entries))
	current := ""
	for _, entry := range entries {
		var q struct {
			ID, Type, Title, Name, ArtistName string
			IsCurrent, Current                bool
			Attributes                        struct{ Name, ArtistName string } `json:"attributes"`
			Item                              *appleItem                        `json:"item"`
		}
		if err := json.Unmarshal(entry, &q); err != nil {
			return nil, "", err
		}
		row := Item{ID: q.ID, Type: q.Type, Title: q.Title, Subtitle: q.ArtistName}
		if row.Title == "" {
			row.Title = q.Name
		}
		if row.Title == "" {
			row.Title = q.Attributes.Name
		}
		if row.Subtitle == "" {
			row.Subtitle = q.Attributes.ArtistName
		}
		if q.Item != nil {
			if row.ID == "" {
				row.ID = q.Item.ID
			}
			if row.Title == "" {
				row.Title = q.Item.Attributes.Name
			}
			if row.Subtitle == "" {
				row.Subtitle = q.Item.Attributes.ArtistName
			}
		}
		if q.IsCurrent || q.Current {
			current = row.ID
		}
		rows = append(rows, row)
	}
	return rows, current, nil
}

func loadQueue(c Client) tea.Cmd {
	return func() tea.Msg {
		var msg queueMsg
		for attempt := 0; attempt < 2; attempt++ {
			var raw json.RawMessage
			err := c.Get("/api/v1/playback/queue", &raw)
			if err == nil {
				msg.rows, msg.current, err = parseQueue(raw)
			}
			msg.err = err
			if err == nil || !incompleteQueueResponse(err) || attempt == 1 {
				return msg
			}
			// Retry only the read, never the playback mutation. The delay runs
			// inside the command, not on the UI event loop.
			time.Sleep(150 * time.Millisecond)
		}
		return msg
	}
}

func incompleteQueueResponse(err error) bool {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var syntax *json.SyntaxError
	return errors.As(err, &syntax) && syntax.Error() == "unexpected end of JSON input"
}

type searchResults struct {
	Results struct {
		Songs     page `json:"songs"`
		Albums    page `json:"albums"`
		Playlists page `json:"playlists"`
	} `json:"results"`
}

func search(c Client, term string, gen int) tea.Cmd {
	return func() tea.Msg {
		storefront, err := storefront(c)
		if err != nil {
			return loadedMsg{tab: Search, gen: gen, err: err}
		}
		var result searchResults
		err = c.AMAPI(searchPath(storefront, term), &result)
		if err != nil {
			return loadedMsg{tab: Search, gen: gen, err: err}
		}
		var p page
		for _, group := range []struct {
			name string
			data page
		}{
			{"songs", result.Results.Songs}, {"albums", result.Results.Albums}, {"playlists", result.Results.Playlists},
		} {
			for _, item := range group.data.Data {
				if item.Type == "" {
					item.Type = group.name
				}
				p.Data = append(p.Data, item)
			}
		}
		return loadedMsg{tab: Search, gen: gen, page: p}
	}
}

// librarySearch uses Apple's library search for songs, or albums in album
// mode. Its playlist search returns nothing, so playlists are filtered locally.
func librarySearch(c Client, term string, albums bool, gen int) tea.Cmd {
	kind := "library-songs"
	if albums {
		kind = "library-albums"
	}
	return load(c, Library, false, gen, "/v1/me/library/search?types="+kind+"&limit=25&term="+url.QueryEscape(term))
}

func errorStatus(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if strings.Contains(strings.ToLower(s), "token") || strings.Contains(s, "403") {
		return fmt.Sprintf("Cider token error: %s. Create an API token in Cider or set CIDER_TOKEN.", s)
	}
	return s
}
