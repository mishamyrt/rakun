package providers

import (
	"context"
	"fmt"
	"rakun/internal/config"
	"rakun/internal/git"
	"sort"
)

type NamespaceTargetCollector func(ctx context.Context, namespace string, namespaceConfig *config.Namespace) ([]git.RemoteTarget, error)

type TargetAccumulator struct {
	seen    map[string]struct{}
	targets []git.RemoteTarget
}

func NewTargetAccumulator(capacity int) *TargetAccumulator {
	return &TargetAccumulator{
		seen:    make(map[string]struct{}, capacity),
		targets: make([]git.RemoteTarget, 0, capacity),
	}
}

func (a *TargetAccumulator) Add(target git.RemoteTarget) bool {
	if _, ok := a.seen[target.URL]; ok {
		return false
	}
	a.seen[target.URL] = struct{}{}
	a.targets = append(a.targets, target)
	return true
}

func (a *TargetAccumulator) AddAll(targets []git.RemoteTarget) {
	for _, target := range targets {
		a.Add(target)
	}
}

func (a *TargetAccumulator) Targets() []git.RemoteTarget {
	return a.targets
}

func SortedNamespaces(namespaces map[string]*config.Namespace) []string {
	sorted := make([]string, 0, len(namespaces))
	for namespace := range namespaces {
		sorted = append(sorted, namespace)
	}
	sort.Strings(sorted)
	return sorted
}

func CollectTargets(
	ctx context.Context,
	provider string,
	group config.Group,
	newTarget func(repoRef string) (git.RemoteTarget, error),
	hasAPI bool,
	collectNamespaceTargets NamespaceTargetCollector,
) ([]git.RemoteTarget, error) {
	targets := NewTargetAccumulator(len(group.Repos))
	for _, repoRef := range group.Repos {
		target, err := newTarget(repoRef)
		if err != nil {
			return nil, err
		}
		targets.Add(target)
	}

	if len(group.Namespaces) == 0 {
		return targets.Targets(), nil
	}
	if !hasAPI {
		return nil, fmt.Errorf("%s api is required when namespaces are configured", provider)
	}

	for _, namespace := range SortedNamespaces(group.Namespaces) {
		namespaceTargets, err := collectNamespaceTargets(ctx, namespace, group.Namespaces[namespace])
		if err != nil {
			return nil, err
		}
		targets.AddAll(namespaceTargets)
	}

	return targets.Targets(), nil
}
