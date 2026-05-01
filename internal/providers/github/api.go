package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"rakun/internal/providers"
	"strconv"
	"strings"
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

func NewAPI(baseURL string, token string) (*API, error) {
	if err := providers.RequireToken("github", token); err != nil {
		return nil, err
	}
	if baseURL == "" {
		baseURL = APIURL
	}
	return &API{
		Token:   token,
		BaseURL: baseURL,
		Client: &http.Client{
			Timeout: providers.DefaultHTTPTimeout,
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
	client, baseURL := providers.ClientAndBaseURL(s.Client, s.BaseURL, APIURL)

	requestURL := fmt.Sprintf("%s/users/%s", baseURL, url.PathEscape(owner))

	var namespace namespace
	if err := providers.GetJSON(ctx, client, requestURL, map[string]string{
		"Authorization": "Bearer " + s.Token,
	}, "github", &namespace); err != nil {
		return "", err
	}
	return namespace.Type, nil
}

func (s API) searchRepositories(ctx context.Context, qualifier string, owner string) ([]Repository, error) {
	client, baseURL := providers.ClientAndBaseURL(s.Client, s.BaseURL, APIURL)

	var items []Repository
	for page := 1; ; page++ {
		query := url.Values{}
		query.Set("q", qualifier+":"+owner)
		query.Set("per_page", "100")
		query.Set("page", strconv.Itoa(page))

		requestURL := fmt.Sprintf("%s/search/repositories?%s", baseURL, query.Encode())

		var result repositoriesSearchResult
		if err := providers.GetJSON(ctx, client, requestURL, map[string]string{
			"Authorization": "Bearer " + s.Token,
		}, "github", &result); err != nil {
			return nil, err
		}
		if len(result.Items) == 0 {
			break
		}
		items = append(items, result.Items...)
	}

	return items, nil
}
