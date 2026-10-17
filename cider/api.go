package cider

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type Page struct {
	Data json.RawMessage `json:"data"`
	Next string          `json:"next"`
	Meta struct {
		Total int `json:"total"`
	} `json:"meta"`
}

// PageIterator fetches one page per Next call. Page returns the most recently
// fetched page, including its unmodified Apple Music items and total count.
type PageIterator struct {
	client *Client
	next   string
	seen   map[string]bool
	page   Page
	err    error
}

func (c *Client) Pages(path string) *PageIterator {
	return &PageIterator{client: c, next: path, seen: make(map[string]bool)}
}

func (it *PageIterator) Next() bool {
	if it.err != nil || it.next == "" {
		return false
	}
	if it.seen[it.next] {
		it.err = errors.New("Apple Music API paging loop")
		return false
	}
	it.seen[it.next] = true
	var page Page
	if err := it.client.AMAPI(it.next, &page); err != nil {
		it.err = err
		return false
	}
	if page.Next != "" && !strings.HasPrefix(page.Next, "/v1/") {
		it.err = errors.New("invalid Apple Music API next path")
		return false
	}
	it.page = page
	it.next = page.Next
	return true
}

func (it *PageIterator) Page() Page { return it.page }
func (it *PageIterator) Err() error { return it.err }

type VolumeState struct {
	Status string  `json:"status"`
	Volume float64 `json:"volume"`
}

type NowPlayingState struct {
	Status string `json:"status"`
	Info   struct {
		Name                string  `json:"name"`
		ArtistName          string  `json:"artistName"`
		AlbumName           string  `json:"albumName"`
		CurrentPlaybackTime float64 `json:"currentPlaybackTime"`
		RemainingTime       float64 `json:"remainingTime"`
		DurationInMillis    int64   `json:"durationInMillis"`
		ShuffleMode         int     `json:"shuffleMode"`
		RepeatMode          int     `json:"repeatMode"`
		PlayParams          struct {
			ID string `json:"id"`
		} `json:"playParams"`
	} `json:"info"`
}

// Title returns the track name, or "" when nothing is loaded.
func (s NowPlayingState) Title() string { return s.Info.Name }

func (c *Client) Volume() (float64, error) {
	var state VolumeState
	err := c.Get("/api/v1/playback/volume", &state)
	return state.Volume, err
}

func (c *Client) NowPlaying() (NowPlayingState, error) {
	var state NowPlayingState
	err := c.Get("/api/v1/playback/now-playing", &state)
	return state, err
}

func (c *Client) LibraryCount() (int, error) {
	var page Page
	err := c.AMAPI("/v1/me/library/songs?limit=1", &page)
	return page.Meta.Total, err
}

func (c *Client) PlayPause() error     { return c.Post("/api/v1/playback/playpause", nil, nil) }
func (c *Client) Next() error          { return c.Post("/api/v1/playback/next", nil, nil) }
func (c *Client) Previous() error      { return c.Post("/api/v1/playback/previous", nil, nil) }
func (c *Client) ToggleShuffle() error { return c.Post("/api/v1/playback/toggle-shuffle", nil, nil) }
func (c *Client) ToggleRepeat() error  { return c.Post("/api/v1/playback/toggle-repeat", nil, nil) }
func (c *Client) Seek(sec float64) error {
	return c.Post("/api/v1/playback/seek", map[string]float64{"position": sec}, nil)
}
func (c *Client) SetVolume(volume float64) error {
	if volume < 0 || volume > 1 {
		return errors.New("volume must be between 0 and 1")
	}
	return c.Post("/api/v1/playback/volume", map[string]float64{"volume": volume}, nil)
}
func (c *Client) Queue(out any) error { return c.Get("/api/v1/playback/queue", out) }

func (c *Client) PlayItem(kind, id string) error {
	return c.Post("/api/v1/playback/play-item", map[string]string{"type": kind, "id": id}, nil)
}
func (c *Client) PlayNext(kind, id string) error {
	return c.Post("/api/v1/playback/play-next", map[string]string{"type": kind, "id": id}, nil)
}
func (c *Client) PlayLater(kind, id string) error {
	return c.Post("/api/v1/playback/play-later", map[string]string{"type": kind, "id": id}, nil)
}

func (c *Client) Storefront() (string, error) {
	var response struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := c.AMAPI("/v1/me/storefront", &response); err != nil {
		return "", err
	}
	if len(response.Data) == 0 {
		return "", errors.New("no Apple Music storefront")
	}
	return response.Data[0].ID, nil
}

func (c *Client) Search(term string, out any) error {
	storefront, err := c.Storefront()
	if err != nil {
		return err
	}
	path := "/v1/catalog/" + url.PathEscape(storefront) + "/search?term=" + url.QueryEscape(term) + "&types=songs,albums,playlists"
	return c.AMAPI(path, out)
}

func (c *Client) LibrarySongs(offset int, out any) error { return c.libraryList("songs", offset, out) }
func (c *Client) LibraryAlbums(offset int, out any) error {
	return c.libraryList("albums", offset, out)
}
func (c *Client) LibraryPlaylists(offset int, out any) error {
	return c.libraryList("playlists", offset, out)
}
func (c *Client) libraryList(kind string, offset int, out any) error {
	if offset < 0 {
		return errors.New("offset must be nonnegative")
	}
	return c.AMAPI("/v1/me/library/"+kind+"?limit=100&offset="+strconv.Itoa(offset), out)
}
func (c *Client) AlbumTracks(id string, out any) error {
	return c.AMAPI(fmt.Sprintf("/v1/me/library/albums/%s/tracks", url.PathEscape(id)), out)
}
func (c *Client) PlaylistTracks(id string, out any) error {
	return c.AMAPI(fmt.Sprintf("/v1/me/library/playlists/%s/tracks", url.PathEscape(id)), out)
}
