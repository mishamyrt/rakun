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

const maxHTTPStatusErrorBodySize = 32 << 10

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

	if response.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(io.LimitReader(response.Body, maxHTTPStatusErrorBodySize+1))
		if err != nil {
			return err
		}
		body := string(bodyBytes)
		if len(bodyBytes) > maxHTTPStatusErrorBodySize {
			body = string(bodyBytes[:maxHTTPStatusErrorBodySize]) + "... (truncated)"
		}
		return HTTPStatusError{
			Provider:   provider,
			Status:     response.Status,
			StatusCode: response.StatusCode,
			Body:       body,
		}
	}

	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(dest); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("%s api returned invalid JSON response", provider)
		}
		return err
	}
	return nil
}
