package providers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGetJSONDecodesOKResponse(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"name":"demo"}`)),
			Request:    r,
		}, nil
	})}

	var dest struct {
		Name string `json:"name"`
	}
	if err := GetJSON(context.Background(), client, "https://example.com/api", nil, "example", &dest); err != nil {
		t.Fatalf("GetJSON returned error: %v", err)
	}
	if dest.Name != "demo" {
		t.Fatalf("unexpected decoded payload: %#v", dest)
	}
}

func TestGetJSONRejectsTrailingJSON(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"name":"demo"}{"name":"extra"}`)),
			Request:    r,
		}, nil
	})}

	var dest struct {
		Name string `json:"name"`
	}
	if err := GetJSON(context.Background(), client, "https://example.com/api", nil, "example", &dest); err == nil {
		t.Fatal("expected trailing JSON error")
	}
}

func TestGetJSONLimitsErrorBody(t *testing.T) {
	body := strings.Repeat("x", maxHTTPStatusErrorBodySize+128)
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			Status:     "502 Bad Gateway",
			StatusCode: http.StatusBadGateway,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	})}

	var dest struct{}
	err := GetJSON(context.Background(), client, "https://example.com/api", nil, "example", &dest)
	if err == nil {
		t.Fatal("expected HTTPStatusError")
	}

	var statusErr HTTPStatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("expected HTTPStatusError, got %T", err)
	}
	if statusErr.StatusCode != http.StatusBadGateway {
		t.Fatalf("unexpected status code: %d", statusErr.StatusCode)
	}
	if !strings.HasSuffix(statusErr.Body, "... (truncated)") {
		t.Fatalf("expected truncated body suffix, got %q", statusErr.Body)
	}
	if len(statusErr.Body) != maxHTTPStatusErrorBodySize+len("... (truncated)") {
		t.Fatalf("unexpected body length: %d", len(statusErr.Body))
	}
	if statusErr.Body[:32] != body[:32] {
		t.Fatalf("unexpected body prefix: %q", statusErr.Body[:32])
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
