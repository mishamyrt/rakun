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

const DefaultHTTPTimeout = 10 * time.Second

type HTTPStatusError struct {
	Provider   string
	Status     string
	StatusCode int
	Body       string
}

func (e HTTPStatusError) Error() string {
	return fmt.Sprintf("%s api returned %s: %s", e.Provider, e.Status, e.Body)
}

func RequireToken(provider string, token string) error {
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("%s token is required", provider)
	}
	return nil
}

func ClientAndBaseURL(client *http.Client, baseURL string, defaultBaseURL string) (*http.Client, string) {
	if client == nil {
		client = &http.Client{Timeout: DefaultHTTPTimeout}
	}
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return client, baseURL
}

func GetJSON(ctx context.Context, client *http.Client, requestURL string, headers map[string]string, provider string, dest any) error {
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
	defer response.Body.Close()

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
