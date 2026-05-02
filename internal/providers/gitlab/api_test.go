package gitlab

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"rakun/internal/config"
	"reflect"
	"strings"
	"testing"
)

func TestCollectGroupWithReposNamespacesAndDedup(t *testing.T) {
	var requests []string

	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.URL.RequestURI())
		if r.Header.Get("PRIVATE-TOKEN") != "test-token" {
			t.Fatalf("unexpected private token header: %q", r.Header.Get("PRIVATE-TOKEN"))
		}

		statusCode := http.StatusOK
		body := ""
		switch r.URL.RequestURI() {
		case requestURI("/api/v4/groups/platform%2Fcore/projects", url.Values{
			"include_subgroups": {"true"},
			"page":              {"1"},
			"per_page":          {"100"},
			"with_shared":       {"false"},
		}):
			body = `[{"path":"demo","path_with_namespace":"platform/core/demo"},{"path":"skip-me","path_with_namespace":"platform/core/skip-me"},{"path":"tooling","path_with_namespace":"platform/core/tools/tooling"},{"path":"beta","path_with_namespace":"platform/core/subgroup/beta"}]`
		case requestURI("/api/v4/groups/platform%2Fcore/projects", url.Values{
			"include_subgroups": {"true"},
			"page":              {"2"},
			"per_page":          {"100"},
			"with_shared":       {"false"},
		}):
			body = `[]`
		default:
			statusCode = http.StatusNotFound
			body = `{"message":"404 Not Found"}`
		}
		return &http.Response{
			Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
			StatusCode: statusCode,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	})}

	api := &API{
		Token:   "test-token",
		BaseURL: "https://gitlab.example.com/api/v4",
		Client:  client,
	}

	targets, err := collect(context.Background(), config.Group{
		Domain: "gitlab.example.com",
		Type:   "gitlab",
		Token:  config.Token{Value: "test-token", Set: true},
		Repos:  []string{"platform/core/demo"},
		Namespaces: map[string]*config.Namespace{
			"platform/core": {Skip: []string{"skip-me", "platform/core/tools/tooling"}},
		},
	}, api)
	if err != nil {
		t.Fatalf("collect repositories: %v", err)
	}

	got := []string{}
	for _, target := range targets {
		got = append(got, target.URL)
		if target.Credentials == nil || target.Credentials.Password != "test-token" {
			t.Fatalf("unexpected credentials on target %#v", target)
		}
	}

	expected := []string{
		"https://gitlab.example.com/platform/core/demo.git",
		"https://gitlab.example.com/platform/core/subgroup/beta.git",
	}
	if strings.Join(got, "\n") != strings.Join(expected, "\n") {
		t.Fatalf("unexpected remotes:\n%s", strings.Join(got, "\n"))
	}

	expectedRequests := []string{
		requestURI("/api/v4/groups/platform%2Fcore/projects", url.Values{
			"include_subgroups": {"true"},
			"page":              {"1"},
			"per_page":          {"100"},
			"with_shared":       {"false"},
		}),
		requestURI("/api/v4/groups/platform%2Fcore/projects", url.Values{
			"include_subgroups": {"true"},
			"page":              {"2"},
			"per_page":          {"100"},
			"with_shared":       {"false"},
		}),
	}
	if !reflect.DeepEqual(requests, expectedRequests) {
		t.Fatalf("unexpected requests:\n%v", requests)
	}
}

func TestCollectReposOnlyWithoutAPI(t *testing.T) {
	targets, err := Collect(context.Background(), config.Group{
		Domain: "gitlab.com",
		Type:   "gitlab",
		Repos:  []string{"platform/core/project"},
	})
	if err != nil {
		t.Fatalf("collect repositories: %v", err)
	}

	if len(targets) != 1 || targets[0].URL != "https://gitlab.com/platform/core/project.git" {
		t.Fatalf("unexpected targets: %#v", targets)
	}
	if targets[0].Credentials != nil {
		t.Fatalf("expected public repo to have no credentials: %#v", targets[0])
	}
}

func TestAPIBaseURL(t *testing.T) {
	if got := APIBaseURL("gitlab.com"); got != APIURL {
		t.Fatalf("unexpected gitlab.com api url: %q", got)
	}
	if got := APIBaseURL("gitlab.example.com"); got != "https://gitlab.example.com/api/v4" {
		t.Fatalf("unexpected self-hosted api url: %q", got)
	}
}

func TestNewAPIRejectsEmptyToken(t *testing.T) {
	_, err := NewAPI(APIURL, "")
	if err == nil {
		t.Fatal("expected token validation error")
	}
	if !strings.Contains(err.Error(), "token is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func requestURI(path string, query url.Values) string {
	if len(query) == 0 {
		return path
	}
	return path + "?" + query.Encode()
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
