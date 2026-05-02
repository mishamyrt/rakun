package providers

import (
	"context"
	"fmt"
	"rakun/internal/config"
	"rakun/internal/git"
	"sort"

	"golang.org/x/sync/errgroup"
)

// NamespaceTargetCollector is a function that collects targets for a given namespace.
type NamespaceTargetCollector func(
	ctx context.Context,
	namespace string,
	namespaceConfig *config.Namespace,
) ([]git.RemoteTarget, error)

// CredentialsTargetFactory builds a target using group-scoped credentials.
type CredentialsTargetFactory func(repoRef string, credentials *git.Credentials) (git.RemoteTarget, error)

// CredentialsNamespaceTargetCollector collects namespace targets using group-scoped credentials.
type CredentialsNamespaceTargetCollector func(
	ctx context.Context,
	namespace string,
	namespaceConfig *config.Namespace,
	credentials *git.Credentials,
) ([]git.RemoteTarget, error)

// TargetAccumulator is a struct that accumulates unique git.RemoteTargets.
type TargetAccumulator struct {
	seen    map[string]struct{}
	targets []git.RemoteTarget
}

// NewTargetAccumulator creates a new TargetAccumulator with the given capacity.
func NewTargetAccumulator(capacity int) *TargetAccumulator {
	return &TargetAccumulator{
		seen:    make(map[string]struct{}, capacity),
		targets: make([]git.RemoteTarget, 0, capacity),
	}
}

// Add adds a target to the accumulator if it has not been seen before.
func (a *TargetAccumulator) Add(target git.RemoteTarget) bool {
	if _, ok := a.seen[target.URL]; ok {
		return false
	}
	a.seen[target.URL] = struct{}{}
	a.targets = append(a.targets, target)
	return true
}

// AddAll adds all targets to the accumulator, skipping any that have already been seen.
func (a *TargetAccumulator) AddAll(targets []git.RemoteTarget) {
	for _, target := range targets {
		a.Add(target)
	}
}

// Targets returns the accumulated targets.
func (a *TargetAccumulator) Targets() []git.RemoteTarget {
	return a.targets
}

// SortedNamespaces returns a sorted list of namespace names.
func SortedNamespaces(namespaces map[string]*config.Namespace) []string {
	sorted := make([]string, 0, len(namespaces))
	for namespace := range namespaces {
		sorted = append(sorted, namespace)
	}
	sort.Strings(sorted)
	return sorted
}

// CollectGroupTargets collects direct repo and namespace targets using group-scoped credentials.
func CollectGroupTargets(
	ctx context.Context,
	provider string,
	group config.Group,
	newTarget CredentialsTargetFactory,
	collectNamespaceTargets CredentialsNamespaceTargetCollector,
) ([]git.RemoteTarget, error) {
	credentials := git.NewTokenCredentials(group.Token.Value)

	var namespaceCollector NamespaceTargetCollector
	if collectNamespaceTargets != nil {
		namespaceCollector = func(ctx context.Context, namespace string, namespaceConfig *config.Namespace) ([]git.RemoteTarget, error) {
			return collectNamespaceTargets(ctx, namespace, namespaceConfig, credentials)
		}
	}

	return CollectTargets(ctx, provider, group, func(repoRef string) (git.RemoteTarget, error) {
		return newTarget(repoRef, credentials)
	}, collectNamespaceTargets != nil, namespaceCollector)
}

// CollectTargets collects targets for a given provider and group.
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

	namespaces := SortedNamespaces(group.Namespaces)
	namespaceTargets := make([][]git.RemoteTarget, len(namespaces))
	collectGroup, collectCtx := errgroup.WithContext(ctx)

	for i, namespace := range namespaces {
		i := i
		namespace := namespace
		namespaceConfig := group.Namespaces[namespace]
		collectGroup.Go(func() error {
			collected, err := collectNamespaceTargets(collectCtx, namespace, namespaceConfig)
			if err != nil {
				return err
			}
			namespaceTargets[i] = collected
			return nil
		})
	}

	if err := collectGroup.Wait(); err != nil {
		return nil, err
	}

	for _, collected := range namespaceTargets {
		targets.AddAll(collected)
	}

	return targets.Targets(), nil
}
