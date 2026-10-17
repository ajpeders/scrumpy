// Package live connects to Cider's socket.io endpoint using Engine.IO v4 HTTP
// long polling. Cider's application event names and payloads have not yet been
// observed: consumers must continue periodic HTTP polling even while connected.
package live

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Kind identifies a transport state change or an unmapped application event.
type Kind uint8

const (
	Connected Kind = iota + 1
	Disconnected
	Event
)

// Signal is a hint to refresh HTTP state, not an authoritative playback state.
// PollRequired is always true until Cider's event coverage is verified live.
type Signal struct {
	Kind         Kind
	PollRequired bool
	Name         string
	Payload      json.RawMessage // raw socket.io argument array (including the name)
	Err          error
}

// Client uses a single polling session. It never sends application events.
type Client struct {
	endpoint *url.URL
	token    string
	http     *http.Client
}

func New(baseURL, token string) (*Client, error) {
	if baseURL == "" {
		baseURL = "http://localhost:10767"
	}
	u, err := url.Parse(baseURL)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("invalid Cider server URL")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/socket.io/"
	return &Client{endpoint: u, token: token, http: &http.Client{Timeout: 65 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse // never forward apptoken to another host
	}}}, nil
}

// SetHTTPClient replaces the transport for testing. Redirects remain forbidden.
func (c *Client) SetHTTPClient(client *http.Client) {
	if client != nil {
		copy := *client
		copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		c.http = &copy
	}
}

// Run reconnects until ctx is cancelled. The caller should consume signals
// concurrently; slow consumers may miss events, so periodic HTTP polling is
// required regardless of connection state. Run closes signals on return.
func (c *Client) Run(ctx context.Context, signals chan<- Signal) {
	defer close(signals)
	backoff := time.Second
	for ctx.Err() == nil {
		err := c.session(ctx, signals)
		if ctx.Err() != nil {
			return
		}
		c.emit(signals, Signal{Kind: Disconnected, Err: err})
		// A rejected token cannot recover by reconnecting with the same token.
		var status *StatusError
		if errors.As(err, &status) && status.Code == http.StatusForbidden {
			return
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
		if backoff < 16*time.Second {
			backoff *= 2
		}
	}
}

func (c *Client) emit(ch chan<- Signal, s Signal) {
	s.PollRequired = true
	select {
	case ch <- s:
	default:
	}
}

type StatusError struct{ Code int }

func (e *StatusError) Error() string { return fmt.Sprintf("socket.io HTTP status %d", e.Code) }

const maxPacket = 1 << 20

func (c *Client) request(ctx context.Context, method, sid, body string) (string, error) {
	u := *c.endpoint
	q := url.Values{"EIO": {"4"}, "transport": {"polling"}}
	if sid != "" {
		q.Set("sid", sid)
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, method, u.String(), strings.NewReader(body))
	if err != nil {
		return "", err
	}
	if c.token != "" {
		req.Header.Set("apptoken", c.token)
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", &StatusError{resp.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxPacket+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxPacket {
		return "", errors.New("socket.io packet too large")
	}
	return string(data), nil
}

type handshake struct {
	SID          string `json:"sid"`
	PingInterval int64  `json:"pingInterval"`
	PingTimeout  int64  `json:"pingTimeout"`
}

func parseHandshake(packet string) (handshake, error) {
	var h handshake
	if !strings.HasPrefix(packet, "0") || json.Unmarshal([]byte(packet[1:]), &h) != nil || h.SID == "" || h.PingInterval <= 0 || h.PingTimeout <= 0 {
		return h, errors.New("invalid Engine.IO handshake")
	}
	return h, nil
}

func (c *Client) session(ctx context.Context, signals chan<- Signal) error {
	open, err := c.request(ctx, http.MethodGet, "", "")
	if err != nil {
		return err
	}
	h, err := parseHandshake(open)
	if err != nil {
		return err
	}
	// Engine.IO opens the transport first, then joins the default socket.io namespace.
	if _, err := c.request(ctx, http.MethodPost, h.SID, "40"); err != nil {
		return err
	}
	joined := false
	for ctx.Err() == nil {
		packets, err := c.request(ctx, http.MethodGet, h.SID, "")
		if err != nil {
			return err
		}
		for _, packet := range strings.Split(packets, "\x1e") {
			if packet == "" {
				continue
			}
			switch {
			case packet == "2": // server heartbeat: pong on the same session
				if _, err := c.request(ctx, http.MethodPost, h.SID, "3"); err != nil {
					return err
				}
			case packet == "1" || packet == "41":
				return errors.New("socket.io connection closed")
			case strings.HasPrefix(packet, "44"):
				return errors.New("socket.io namespace rejected")
			case strings.HasPrefix(packet, "40"):
				if !joined {
					joined = true
					c.emit(signals, Signal{Kind: Connected})
				}
			case strings.HasPrefix(packet, "42") && joined:
				if event, ok := parseEvent(packet); ok {
					c.emit(signals, event)
				}
			}
		}
	}
	return ctx.Err()
}

// parseEvent accepts only the default namespace's plain JSON event packets.
// Binary attachments, acknowledgements and custom namespaces are not mapped.
func parseEvent(packet string) (Signal, bool) {
	if !strings.HasPrefix(packet, "42[") {
		return Signal{}, false
	}
	var args []json.RawMessage
	if err := json.Unmarshal([]byte(packet[2:]), &args); err != nil || len(args) == 0 {
		return Signal{}, false
	}
	var name string
	if json.Unmarshal(args[0], &name) != nil || name == "" {
		return Signal{}, false
	}
	return Signal{Kind: Event, Name: name, Payload: json.RawMessage(packet[2:]), PollRequired: true}, true
}
