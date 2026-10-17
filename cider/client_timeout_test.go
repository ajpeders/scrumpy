package cider

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

type timeoutTransport struct{}

func (timeoutTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, context.DeadlineExceeded
}

func TestRequestTimeoutIsIdentifiable(t *testing.T) {
	c, err := New("", "")
	if err != nil {
		t.Fatal(err)
	}
	c.SetHTTPClient(&http.Client{Transport: timeoutTransport{}})
	if err := c.Get("/api/v1/playback/queue", nil); !errors.Is(err, ErrTimeout) {
		t.Fatalf("timeout reported as %v", err)
	}
}
