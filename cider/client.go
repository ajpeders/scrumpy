package cider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

var (
	ErrNotRunning = errors.New("Cider isn't running (connection refused)")
	ErrBadToken   = errors.New("Cider rejected the API token (403)")
	ErrTimeout    = errors.New("Cider API request timed out")
)

// HTTPError reports a non-auth HTTP failure without including any response
// bodies, which could contain private library information.
type HTTPError struct {
	StatusCode int
}

func (e *HTTPError) Error() string { return fmt.Sprintf("Cider HTTP status %d", e.StatusCode) }

type Client struct {
	base  *url.URL
	token string
	http  *http.Client
}

func New(baseURL, token string) (*Client, error) {
	if baseURL == "" {
		baseURL = DefaultURL
	}
	u, err := url.Parse(baseURL)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("base URL must be an HTTP(S) server URL without credentials, query or fragment")
	}
	return &Client{base: u, token: token, http: &http.Client{Timeout: 15 * time.Second, CheckRedirect: noRedirect}}, nil
}

// noRedirect stops redirects: Go only strips standard auth headers when
// following one, so a redirect would otherwise forward apptoken.
func noRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// SetHTTPClient permits replacing the transport for tests or custom deployment.
// Redirects remain forbidden.
func (c *Client) SetHTTPClient(client *http.Client) {
	if client != nil {
		copy := *client
		copy.CheckRedirect = noRedirect
		c.http = &copy
	}
}

func (c *Client) request(method, path string, body, out any) error {
	// An absolute path from the API must not be allowed to redirect a token to
	// another host, including through a `next` link.
	u, err := url.Parse(path)
	if err != nil || u == nil || u.IsAbs() || u.Host != "" || !strings.HasPrefix(u.Path, "/") || strings.HasPrefix(path, "//") {
		return errors.New("invalid Cider API path")
	}
	endpoint := *c.base
	endpoint.Path = strings.TrimRight(c.base.Path, "/") + u.Path
	endpoint.RawQuery = u.RawQuery
	endpoint.Fragment = ""
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, endpoint.String(), reader)
	if err != nil {
		return errors.New("invalid Cider request")
	}
	if c.token != "" {
		req.Header.Set("apptoken", c.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if isConnectionRefused(err) {
			return ErrNotRunning
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return ErrTimeout
		}
		return errors.New("Cider request failed")
	}
	defer func() {
		// Commands often ignore their small JSON acknowledgement. Read it to
		// EOF before closing so HTTP/1 keep-alive connections can be reused.
		// Bound the drain for unexpected/oversized responses.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		_ = resp.Body.Close()
	}()
	if resp.StatusCode == http.StatusForbidden {
		return ErrBadToken
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPError{StatusCode: resp.StatusCode}
	}
	if out != nil {
		if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(out); err != nil {
			return fmt.Errorf("decode Cider response: %w", err)
		}
	}
	return nil
}

func (c *Client) Get(path string, out any) error { return c.request(http.MethodGet, path, nil, out) }
func (c *Client) Post(path string, body, out any) error {
	return c.request(http.MethodPost, path, body, out)
}

func isConnectionRefused(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED)
}

// AMAPI unwraps Cider's {"data": AppleResponse} envelope.
func (c *Client) AMAPI(path string, out any) error {
	if !strings.HasPrefix(path, "/v1/") {
		return errors.New("invalid Apple Music API path")
	}
	var response struct {
		Data json.RawMessage `json:"data"`
	}
	if err := c.Post("/api/v1/amapi/run-v3", map[string]string{"path": path}, &response); err != nil {
		return err
	}
	if len(response.Data) == 0 {
		return errors.New("missing Apple Music API data")
	}
	if out != nil {
		if err := json.Unmarshal(response.Data, out); err != nil {
			return fmt.Errorf("decode Apple Music API response: %w", err)
		}
	}
	return nil
}
