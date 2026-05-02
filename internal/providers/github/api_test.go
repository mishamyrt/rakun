package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"rakun/internal/config"
	"reflect"
	"sort"
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
		case r.URL.Path == "/user":
			_, _ = fmt.Fprint(w, `{"login":"viewer"}`)
		case r.URL.Path == "/users/example/repos" && r.URL.Query().Get("type") == "owner" && r.URL.Query().Get("sort") == "full_name" && r.URL.Query().Get("page") == "1" && r.URL.Query().Get("per_page") == "100":
			_, _ = fmt.Fprint(w, `[{"name":"alpha","full_name":"example/alpha"},{"name":"skip-me","full_name":"example/skip-me"}]`)
		case r.URL.Path == "/users/example/repos" && r.URL.Query().Get("type") == "owner" && r.URL.Query().Get("sort") == "full_name" && r.URL.Query().Get("page") == "2" && r.URL.Query().Get("per_page") == "100":
			_, _ = fmt.Fprint(w, `[]`)
		case r.URL.Path == "/orgs/team/repos" && r.URL.Query().Get("type") == "all" && r.URL.Query().Get("sort") == "full_name" && r.URL.Query().Get("page") == "1" && r.URL.Query().Get("per_page") == "100":
			_, _ = fmt.Fprint(w, `[{"name":"demo","full_name":"team/demo"},{"name":"beta","full_name":"team/beta"}]`)
		case r.URL.Path == "/orgs/team/repos" && r.URL.Query().Get("type") == "all" && r.URL.Query().Get("sort") == "full_name" && r.URL.Query().Get("page") == "2" && r.URL.Query().Get("per_page") == "100":
			_, _ = fmt.Fprint(w, `[]`)
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

	targets, err := collect(context.Background(), config.Group{
		Domain: "github.example.com",
		Type:   "github",
		Token:  config.Token{Value: "test-token", Set: true},
		Repos:  []string{"team/demo"},
		Namespaces: map[string]*config.Namespace{
			"example": {Skip: []string{"skip-me"}},
			"team":    nil,
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
		"https://github.example.com/team/demo.git",
		"https://github.example.com/example/alpha.git",
		"https://github.example.com/team/beta.git",
	}
	if strings.Join(got, "\n") != strings.Join(expected, "\n") {
		t.Fatalf("unexpected remotes:\n%s", strings.Join(got, "\n"))
	}

	expectedRequests := []string{
		"/user",
		"/users/example",
		"/users/team",
		requestURI("/users/example/repos", url.Values{"page": {"1"}, "per_page": {"100"}, "sort": {"full_name"}, "type": {"owner"}}),
		requestURI("/users/example/repos", url.Values{"page": {"2"}, "per_page": {"100"}, "sort": {"full_name"}, "type": {"owner"}}),
		requestURI("/orgs/team/repos", url.Values{"page": {"1"}, "per_page": {"100"}, "sort": {"full_name"}, "type": {"all"}}),
		requestURI("/orgs/team/repos", url.Values{"page": {"2"}, "per_page": {"100"}, "sort": {"full_name"}, "type": {"all"}}),
	}
	sort.Strings(requests)
	sort.Strings(expectedRequests)
	if !reflect.DeepEqual(requests, expectedRequests) {
		t.Fatalf("unexpected requests:\n%v", requests)
	}
}

func TestGetUserRepositoriesUsesAuthenticatedUserEndpointForCurrentUser(t *testing.T) {
	var requests []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.RequestURI())
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("unexpected authorization header: %q", r.Header.Get("Authorization"))
		}

		switch {
		case r.URL.Path == "/user":
			_, _ = fmt.Fprint(w, `{"login":"Example"}`)
		case r.URL.Path == "/user/repos" && r.URL.Query().Get("affiliation") == "owner" && r.URL.Query().Get("visibility") == "all" && r.URL.Query().Get("sort") == "full_name" && r.URL.Query().Get("page") == "1" && r.URL.Query().Get("per_page") == "100":
			_, _ = fmt.Fprint(w, `[{"name":"alpha","full_name":"example/alpha"},{"name":"secret","full_name":"example/secret"}]`)
		case r.URL.Path == "/user/repos" && r.URL.Query().Get("affiliation") == "owner" && r.URL.Query().Get("visibility") == "all" && r.URL.Query().Get("sort") == "full_name" && r.URL.Query().Get("page") == "2" && r.URL.Query().Get("per_page") == "100":
			_, _ = fmt.Fprint(w, `[]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	api := API{
		Token:   "test-token",
		BaseURL: server.URL,
		Client:  server.Client(),
	}

	repositories, err := api.GetUserRepositories(context.Background(), "example")
	if err != nil {
		t.Fatalf("get user repositories: %v", err)
	}

	expectedRepositories := []Repository{
		{Name: "alpha", FullName: "example/alpha"},
		{Name: "secret", FullName: "example/secret"},
	}
	if !reflect.DeepEqual(repositories, expectedRepositories) {
		t.Fatalf("unexpected repositories: %#v", repositories)
	}

	expectedRequests := []string{
		"/user",
		requestURI("/user/repos", url.Values{"affiliation": {"owner"}, "page": {"1"}, "per_page": {"100"}, "sort": {"full_name"}, "visibility": {"all"}}),
		requestURI("/user/repos", url.Values{"affiliation": {"owner"}, "page": {"2"}, "per_page": {"100"}, "sort": {"full_name"}, "visibility": {"all"}}),
	}
	if !reflect.DeepEqual(requests, expectedRequests) {
		t.Fatalf("unexpected requests:\n%v", requests)
	}
}

func TestCollectReposOnlyWithoutAPI(t *testing.T) {
	targets, err := Collect(context.Background(), config.Group{
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
