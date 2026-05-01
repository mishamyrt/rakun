package gitlab

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"rakun/internal/providers"
	"strconv"
	"strings"
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

func NewAPI(baseURL string, token string) (*API, error) {
	if err := providers.RequireToken("gitlab", token); err != nil {
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
	if strings.EqualFold(domain, "gitlab.com") {
		return APIURL
	}
	return "https://" + domain + "/api/v4"
}

func (s API) GetGroupProjects(ctx context.Context, groupPath string) ([]Project, error) {
	client, baseURL := providers.ClientAndBaseURL(s.Client, s.BaseURL, APIURL)

	var items []Project
	for page := 1; ; page++ {
		query := url.Values{}
		query.Set("include_subgroups", "true")
		query.Set("page", strconv.Itoa(page))
		query.Set("per_page", "100")
		query.Set("with_shared", "false")

		requestURL := fmt.Sprintf("%s/groups/%s/projects?%s", baseURL, url.PathEscape(groupPath), query.Encode())

		var result []Project
		if err := providers.GetJSON(ctx, client, requestURL, map[string]string{
			"PRIVATE-TOKEN": s.Token,
		}, "gitlab", &result); err != nil {
			return nil, err
		}
		if len(result) == 0 {
			break
		}
		items = append(items, result...)
	}

	return items, nil
}
