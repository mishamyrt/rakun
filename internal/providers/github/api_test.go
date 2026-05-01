package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"rakun/internal/config"
	"reflect"
	"strings"
	"testing"
)

func TestCollectGroupWithReposNamespacesAndDedup(t *testing.T) {
	var requests []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.RequestURI())
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("unexpected authorization header: %q", r.Header.Get("Authorization"))
		}

		switch {
		case r.URL.Path == "/users/example":
			_, _ = fmt.Fprint(w, `{"type":"User"}`)
		case r.URL.Path == "/users/team":
			_, _ = fmt.Fprint(w, `{"type":"Organization"}`)
		case r.URL.Path == "/search/repositories" && r.URL.Query().Get("q") == "user:example" && r.URL.Query().Get("page") == "1" && r.URL.Query().Get("per_page") == "100":
			_, _ = fmt.Fprint(w, `{"items":[{"name":"alpha","full_name":"example/alpha"},{"name":"skip-me","full_name":"example/skip-me"}]}`)
		case r.URL.Path == "/search/repositories" && r.URL.Query().Get("q") == "user:example" && r.URL.Query().Get("page") == "2" && r.URL.Query().Get("per_page") == "100":
			_, _ = fmt.Fprint(w, `{"items":[]}`)
		case r.URL.Path == "/search/repositories" && r.URL.Query().Get("q") == "org:team" && r.URL.Query().Get("page") == "1" && r.URL.Query().Get("per_page") == "100":
			_, _ = fmt.Fprint(w, `{"items":[{"name":"demo","full_name":"team/demo"},{"name":"beta","full_name":"team/beta"}]}`)
		case r.URL.Path == "/search/repositories" && r.URL.Query().Get("q") == "org:team" && r.URL.Query().Get("page") == "2" && r.URL.Query().Get("per_page") == "100":
			_, _ = fmt.Fprint(w, `{"items":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	api := &API{
		Token:   "test-token",
		BaseURL: server.URL,
		Client:  server.Client(),
	}

	targets, err := Collect(context.Background(), api, config.Group{
		Domain: "github.example.com",
		Type:   "github",
		Token:  config.Token{Value: "test-token", Set: true},
		Repos:  []string{"team/demo"},
		Namespaces: map[string]*config.Namespace{
			"example": {Skip: []string{"skip-me"}},
			"team":    nil,
		},
	})
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
		"https://github.example.com/team/demo.git",
		"https://github.example.com/example/alpha.git",
		"https://github.example.com/team/beta.git",
	}
	if strings.Join(got, "\n") != strings.Join(expected, "\n") {
		t.Fatalf("unexpected remotes:\n%s", strings.Join(got, "\n"))
	}

	expectedRequests := []string{
		"/users/example",
		requestURI("/search/repositories", url.Values{"page": {"1"}, "per_page": {"100"}, "q": {"user:example"}}),
		requestURI("/search/repositories", url.Values{"page": {"2"}, "per_page": {"100"}, "q": {"user:example"}}),
		"/users/team",
		requestURI("/search/repositories", url.Values{"page": {"1"}, "per_page": {"100"}, "q": {"org:team"}}),
		requestURI("/search/repositories", url.Values{"page": {"2"}, "per_page": {"100"}, "q": {"org:team"}}),
	}
	if !reflect.DeepEqual(requests, expectedRequests) {
		t.Fatalf("unexpected requests:\n%v", requests)
	}
}

func TestCollectReposOnlyWithoutAPI(t *testing.T) {
	targets, err := Collect(context.Background(), nil, config.Group{
		Domain: "github.com",
		Type:   "github",
		Repos:  []string{"example/project"},
	})
	if err != nil {
		t.Fatalf("collect repositories: %v", err)
	}

	if len(targets) != 1 || targets[0].URL != "https://github.com/example/project.git" {
		t.Fatalf("unexpected targets: %#v", targets)
	}
	if targets[0].Credentials != nil {
		t.Fatalf("expected public repo to have no credentials: %#v", targets[0])
	}
}

func TestAPIBaseURL(t *testing.T) {
	if got := APIBaseURL("github.com"); got != APIURL {
		t.Fatalf("unexpected github.com api url: %q", got)
	}
	if got := APIBaseURL("github.enterprise.local"); got != "https://github.enterprise.local/api/v3" {
		t.Fatalf("unexpected enterprise api url: %q", got)
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
