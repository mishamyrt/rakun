package gitlab

import (
	"context"
	"rakun/internal/config"
	"rakun/internal/git"
	"reflect"
	"strings"
	"testing"
)

type stubGroupProjectsAPI struct {
	calls   []string
	results map[string][]Project
}

func (s *stubGroupProjectsAPI) GetGroupProjects(_ context.Context, groupPath string) ([]Project, error) {
	s.calls = append(s.calls, groupPath)
	return s.results[groupPath], nil
}

func TestCollectBuildsDirectTargetsWithoutAPI(t *testing.T) {
	group := config.Group{
		Domain: "gitlab.example.com",
		Repos:  []string{"acme/demo"},
	}

	targets, err := Collect(context.Background(), group)
	if err != nil {
		t.Fatalf("collect targets: %v", err)
	}

	want := []git.RemoteTarget{
		{URL: "https://gitlab.example.com/acme/demo.git"},
	}
	if !reflect.DeepEqual(targets, want) {
		t.Fatalf("unexpected targets: %#v", targets)
	}
}

func TestCollectUsesAPIForNamespaces(t *testing.T) {
	api := &stubGroupProjectsAPI{
		results: map[string][]Project{
			"group/sub": {
				{Path: "keep", PathWithNamespace: "group/sub/keep"},
			},
		},
	}

	group := config.Group{
		Domain: "gitlab.example.com",
		Token:  config.Token{Value: "secret"},
		Repos:  []string{"acme/direct"},
		Namespaces: map[string]*config.Namespace{
			"group/sub": nil,
		},
	}

	targets, err := collect(context.Background(), group, api)
	if err != nil {
		t.Fatalf("collect targets: %v", err)
	}

	want := []string{
		"https://gitlab.example.com/acme/direct.git",
		"https://gitlab.example.com/group/sub/keep.git",
	}
	got := make([]string, 0, len(targets))
	for _, target := range targets {
		got = append(got, target.URL)
		if target.Credentials == nil || target.Credentials.Password != "secret" {
			t.Fatalf("expected token credentials on target %#v", target)
		}
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected target URLs: %#v", got)
	}
	if !reflect.DeepEqual(api.calls, []string{"group/sub"}) {
		t.Fatalf("unexpected API calls: %#v", api.calls)
	}
}

func TestCollectNamespaceTargetsSkipsByNameAndPathAndUsesFallbackPath(t *testing.T) {
	credentials := &git.Credentials{Username: "git", Password: "secret"}

	targets, err := collectNamespaceTargets(
		context.Background(),
		func(_ context.Context, groupPath string) ([]Project, error) {
			if groupPath != "group/sub" {
				t.Fatalf("unexpected group path: %q", groupPath)
			}
			return []Project{
				{Path: "skip-name", PathWithNamespace: "group/sub/skip-name"},
				{Path: "skip-path", PathWithNamespace: "group/sub/skip-path"},
				{Path: "fallback"},
				{Path: "keep", PathWithNamespace: "group/sub/keep"},
			}, nil
		},
		"gitlab.example.com",
		"group/sub",
		&config.Namespace{Skip: []string{" skip-name ", "/group/sub/skip-path/"}},
		credentials,
	)
	if err != nil {
		t.Fatalf("collect namespace targets: %v", err)
	}

	want := []string{
		"https://gitlab.example.com/group/sub/fallback.git",
		"https://gitlab.example.com/group/sub/keep.git",
	}
	got := make([]string, 0, len(targets))
	for _, target := range targets {
		got = append(got, target.URL)
		if target.Credentials != credentials {
			t.Fatalf("expected shared credentials pointer, got %#v", target.Credentials)
		}
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected target URLs: %#v", got)
	}
}

func TestCollectNamespaceTargetsRejectsEmptyProjectPath(t *testing.T) {
	_, err := collectNamespaceTargets(
		context.Background(),
		func(context.Context, string) ([]Project, error) {
			return []Project{{}}, nil
		},
		"gitlab.example.com",
		"group/sub",
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected empty project path error")
	}
	if !strings.Contains(err.Error(), "gitlab project path is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}
