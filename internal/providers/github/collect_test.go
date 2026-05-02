package github

import (
	"context"
	"rakun/internal/config"
	"rakun/internal/git"
	"reflect"
	"testing"
)

type stubOwnerRepositoriesAPI struct {
	calls   []string
	results map[string][]Repository
}

func (s *stubOwnerRepositoriesAPI) GetOwnerRepositories(_ context.Context, owner string) ([]Repository, error) {
	s.calls = append(s.calls, owner)
	return s.results[owner], nil
}

func TestCollectBuildsDirectTargetsWithoutAPI(t *testing.T) {
	group := config.Group{
		Domain: "git.example.com",
		Repos:  []string{"acme/demo"},
	}

	targets, err := Collect(context.Background(), group)
	if err != nil {
		t.Fatalf("collect targets: %v", err)
	}

	want := []git.RemoteTarget{
		{URL: "https://git.example.com/acme/demo.git"},
	}
	if !reflect.DeepEqual(targets, want) {
		t.Fatalf("unexpected targets: %#v", targets)
	}
}

func TestCollectUsesAPIForNamespaces(t *testing.T) {
	api := &stubOwnerRepositoriesAPI{
		results: map[string][]Repository{
			"org": {
				{Name: "keep", FullName: "org/keep"},
			},
		},
	}

	group := config.Group{
		Domain: "git.example.com",
		Token:  config.Token{Value: "secret"},
		Repos:  []string{"acme/direct"},
		Namespaces: map[string]*config.Namespace{
			"org": nil,
		},
	}

	targets, err := collect(context.Background(), group, api)
	if err != nil {
		t.Fatalf("collect targets: %v", err)
	}

	want := []string{
		"https://git.example.com/acme/direct.git",
		"https://git.example.com/org/keep.git",
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
	if !reflect.DeepEqual(api.calls, []string{"org"}) {
		t.Fatalf("unexpected API calls: %#v", api.calls)
	}
}

func TestCollectNamespaceTargetsSkipsConfiguredReposAndUsesFallbackRepoRef(t *testing.T) {
	credentials := &git.Credentials{Username: "git", Password: "secret"}

	targets, err := collectNamespaceTargets(
		context.Background(),
		func(_ context.Context, owner string) ([]Repository, error) {
			if owner != "org" {
				t.Fatalf("unexpected owner: %q", owner)
			}
			return []Repository{
				{Name: "skip", FullName: "org/skip"},
				{Name: "fallback"},
				{Name: "keep", FullName: "org/keep"},
			}, nil
		},
		"git.example.com",
		"org",
		&config.Namespace{Skip: []string{"skip"}},
		credentials,
	)
	if err != nil {
		t.Fatalf("collect namespace targets: %v", err)
	}

	want := []string{
		"https://git.example.com/org/fallback.git",
		"https://git.example.com/org/keep.git",
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
