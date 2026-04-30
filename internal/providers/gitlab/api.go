package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const APIURL = "https://gitlab.com/api/v4"

type Project struct {
	Path              string `json:"path"`
	PathWithNamespace string `json:"path_with_namespace"`
}

type API struct {
	Token   string
	BaseURL string
	Client  *http.Client
}

type httpStatusError struct {
	Status     string
	StatusCode int
	Body       string
}

func (e httpStatusError) Error() string {
	return fmt.Sprintf("gitlab api returned %s: %s", e.Status, e.Body)
}

func NewAPI(baseURL string, token string) (*API, error) {
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("gitlab token is required")
	}
	if baseURL == "" {
		baseURL = APIURL
	}
	return &API{
		Token:   token,
		BaseURL: baseURL,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func APIBaseURL(domain string) string {
	if strings.EqualFold(domain, "gitlab.com") {
		return APIURL
	}
	return "https://" + domain + "/api/v4"
}

func (s API) GetGroupProjects(ctx context.Context, groupPath string) ([]Project, error) {
	client, baseURL := s.clientAndBaseURL()

	var items []Project
	for page := 1; ; page++ {
		query := url.Values{}
		query.Set("include_subgroups", "true")
		query.Set("page", strconv.Itoa(page))
		query.Set("per_page", "100")
		query.Set("with_shared", "false")

		requestURL := fmt.Sprintf("%s/groups/%s/projects?%s", baseURL, url.PathEscape(groupPath), query.Encode())

		var result []Project
		if err := s.getJSON(ctx, client, requestURL, &result); err != nil {
			return nil, err
		}
		if len(result) == 0 {
			break
		}
		items = append(items, result...)
	}

	return items, nil
}

func (s API) clientAndBaseURL() (*http.Client, string) {
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	baseURL := s.BaseURL
	if baseURL == "" {
		baseURL = APIURL
	}
	return client, baseURL
}

func (s API) getJSON(ctx context.Context, client *http.Client, requestURL string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("PRIVATE-TOKEN", s.Token)

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
		return httpStatusError{
			Status:     response.Status,
			StatusCode: response.StatusCode,
			Body:       string(bodyBytes),
		}
	}

	return json.Unmarshal(bodyBytes, dest)
}
