package github

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

const APIURL = "https://api.github.com"

type Repository struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
}

type namespace struct {
	Type string `json:"type"`
}

type repositoriesSearchResult struct {
	Items []Repository `json:"items"`
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
	return fmt.Sprintf("github api returned %s: %s", e.Status, e.Body)
}

func NewAPI(baseURL string, token string) (*API, error) {
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("github token is required")
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
	if strings.EqualFold(domain, "github.com") {
		return APIURL
	}
	return "https://" + domain + "/api/v3"
}

func (s API) GetOrgRepositories(ctx context.Context, orgName string) ([]Repository, error) {
	return s.searchRepositories(ctx, "org", orgName)
}

func (s API) GetUserRepositories(ctx context.Context, userName string) ([]Repository, error) {
	return s.searchRepositories(ctx, "user", userName)
}

func (s API) GetOwnerRepositories(ctx context.Context, owner string) ([]Repository, error) {
	namespaceType, err := s.getNamespaceType(ctx, owner)
	if err != nil {
		return nil, err
	}

	switch namespaceType {
	case "Organization":
		return s.GetOrgRepositories(ctx, owner)
	case "User":
		return s.GetUserRepositories(ctx, owner)
	default:
		return nil, fmt.Errorf("github namespace %q has unsupported type %q", owner, namespaceType)
	}
}

func (s API) getNamespaceType(ctx context.Context, owner string) (string, error) {
	client, baseURL := s.clientAndBaseURL()

	requestURL := fmt.Sprintf("%s/users/%s", baseURL, url.PathEscape(owner))

	var namespace namespace
	if err := s.getJSON(ctx, client, requestURL, &namespace); err != nil {
		return "", err
	}
	return namespace.Type, nil
}

func (s API) searchRepositories(ctx context.Context, qualifier string, owner string) ([]Repository, error) {
	client, baseURL := s.clientAndBaseURL()

	var items []Repository
	for page := 1; ; page++ {
		query := url.Values{}
		query.Set("q", qualifier+":"+owner)
		query.Set("per_page", "100")
		query.Set("page", strconv.Itoa(page))

		requestURL := fmt.Sprintf("%s/search/repositories?%s", baseURL, query.Encode())

		var result repositoriesSearchResult
		if err := s.getJSON(ctx, client, requestURL, &result); err != nil {
			return nil, err
		}
		if len(result.Items) == 0 {
			break
		}
		items = append(items, result.Items...)
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
	req.Header.Set("Authorization", "Bearer "+s.Token)

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
