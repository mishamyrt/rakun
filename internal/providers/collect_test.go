package providers

import (
	"context"
	"fmt"
	"rakun/internal/config"
	"rakun/internal/git"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCollectTargetsCollectsNamespacesInParallelAndPreservesOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	group := config.Group{
		Repos: []string{"direct/repo", "alpha/dupe"},
		Namespaces: map[string]*config.Namespace{
			"gamma": nil,
			"alpha": nil,
			"beta":  nil,
		},
	}

	var started atomic.Int32
	ready := make(chan struct{})
	var readyOnce sync.Once
	totalNamespaces := int32(len(group.Namespaces))

	targets, err := CollectTargets(
		ctx,
		"github",
		group,
		func(repoRef string) (git.RemoteTarget, error) {
			return git.RemoteTarget{URL: fmt.Sprintf("https://example.com/%s.git", repoRef)}, nil
		},
		true,
		func(ctx context.Context, namespace string, _ *config.Namespace) ([]git.RemoteTarget, error) {
			if started.Add(1) == totalNamespaces {
				readyOnce.Do(func() {
					close(ready)
				})
			}

			select {
			case <-ready:
			case <-ctx.Done():
				return nil, ctx.Err()
			}

			switch namespace {
			case "alpha":
				return []git.RemoteTarget{
					{URL: "https://example.com/alpha/dupe.git"},
					{URL: "https://example.com/alpha/extra.git"},
				}, nil
			case "beta":
				return []git.RemoteTarget{
					{URL: "https://example.com/beta/repo.git"},
				}, nil
			case "gamma":
				return []git.RemoteTarget{
					{URL: "https://example.com/gamma/repo.git"},
				}, nil
			default:
				return nil, fmt.Errorf("unexpected namespace %q", namespace)
			}
		},
	)
	if err != nil {
		t.Fatalf("collect targets: %v", err)
	}

	got := make([]string, 0, len(targets))
	for _, target := range targets {
		got = append(got, target.URL)
	}

	expected := []string{
		"https://example.com/direct/repo.git",
		"https://example.com/alpha/dupe.git",
		"https://example.com/alpha/extra.git",
		"https://example.com/beta/repo.git",
		"https://example.com/gamma/repo.git",
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("unexpected targets:\n%v", got)
	}

	if started.Load() != totalNamespaces {
		t.Fatalf("expected %d namespace collectors to start, got %d", totalNamespaces, started.Load())
	}
}

func TestCollectTargetsRequiresAPIForNamespaces(t *testing.T) {
	group := config.Group{
		Namespaces: map[string]*config.Namespace{
			"alpha": nil,
		},
	}

	_, err := CollectTargets(
		context.Background(),
		"github",
		group,
		func(repoRef string) (git.RemoteTarget, error) {
			t.Fatalf("did not expect direct repo collector to be called for %q", repoRef)
			return git.RemoteTarget{}, nil
		},
		false,
		nil,
	)
	if err == nil {
		t.Fatal("expected missing API error")
	}
	if !strings.Contains(err.Error(), "github api is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCollectGroupTargetsUsesGroupTokenCredentials(t *testing.T) {
	group := config.Group{
		Repos: []string{"acme/direct"},
		Token: config.Token{Value: "secret"},
		Namespaces: map[string]*config.Namespace{
			"team": {Skip: []string{"skip-me"}},
		},
	}

	targets, err := CollectGroupTargets(
		context.Background(),
		"github",
		group,
		func(repoRef string, credentials *git.Credentials) (git.RemoteTarget, error) {
			if repoRef != "acme/direct" {
				t.Fatalf("unexpected direct repo: %q", repoRef)
			}
			if credentials == nil || credentials.Username != "git" || credentials.Password != "secret" {
				t.Fatalf("unexpected direct credentials: %#v", credentials)
			}
			return git.RemoteTarget{
				URL:         "https://example.com/" + repoRef + ".git",
				Credentials: credentials,
			}, nil
		},
		func(_ context.Context, namespace string, namespaceConfig *config.Namespace, credentials *git.Credentials) ([]git.RemoteTarget, error) {
			if namespace != "team" {
				t.Fatalf("unexpected namespace: %q", namespace)
			}
			if !reflect.DeepEqual(namespaceConfig, group.Namespaces["team"]) {
				t.Fatalf("unexpected namespace config: %#v", namespaceConfig)
			}
			if credentials == nil || credentials.Username != "git" || credentials.Password != "secret" {
				t.Fatalf("unexpected namespace credentials: %#v", credentials)
			}
			return []git.RemoteTarget{
				{
					URL:         "https://example.com/team/from-api.git",
					Credentials: credentials,
				},
			}, nil
		},
	)
	if err != nil {
		t.Fatalf("collect group targets: %v", err)
	}

	if len(targets) != 2 {
		t.Fatalf("unexpected targets length: %d", len(targets))
	}
	if targets[0].Credentials == nil || targets[1].Credentials == nil {
		t.Fatalf("expected credentials on all targets: %#v", targets)
	}
	if targets[0].Credentials != targets[1].Credentials {
		t.Fatal("expected direct and namespace targets to share one credentials instance")
	}
}
