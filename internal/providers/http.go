package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultHTTPTimeout is the default timeout for HTTP requests.
const DefaultHTTPTimeout = 10 * time.Second

// HTTPStatusError is an error returned when an HTTP request returns a non-200 status code.
type HTTPStatusError struct {
	Provider   string
	Status     string
	StatusCode int
	Body       string
}

// Error returns a string representation of the HTTP status error.
func (e HTTPStatusError) Error() string {
	return fmt.Sprintf("%s api returned %s: %s", e.Provider, e.Status, e.Body)
}

// RequireToken returns an error if the token is empty.
func RequireToken(provider string, token string) error {
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("%s token is required", provider)
	}
	return nil
}

// ClientAndBaseURL returns a client and base URL for HTTP requests.
func ClientAndBaseURL(client *http.Client, baseURL string, defaultBaseURL string) (*http.Client, string) {
	if client == nil {
		client = &http.Client{Timeout: DefaultHTTPTimeout}
	}
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return client, baseURL
}

// GetJSON makes a GET request to the given URL and unmarshals the response into dest.
func GetJSON(ctx context.Context, client *http.Client, requestURL string, headers map[string]string, provider string, dest any) (err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := response.Body.Close(); err == nil {
			err = closeErr
		}
	}()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return HTTPStatusError{
			Provider:   provider,
			Status:     response.Status,
			StatusCode: response.StatusCode,
			Body:       string(bodyBytes),
		}
	}

	return json.Unmarshal(bodyBytes, dest)
}
