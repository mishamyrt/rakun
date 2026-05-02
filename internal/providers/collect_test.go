package providers

import (
	"context"
	"fmt"
	"rakun/internal/config"
	"rakun/internal/git"
	"reflect"
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
