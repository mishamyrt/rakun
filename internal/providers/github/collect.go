package github

import (
	"context"
	"fmt"
	"rakun/internal/config"
	"rakun/internal/git"
	"rakun/internal/taskrun"
	"rakun/pkg/set"
	"sort"
)

type repositoriesGetter func(ctx context.Context, owner string) ([]Repository, error)

func Collect(ctx context.Context, api *API, group config.Group) ([]git.RemoteTarget, error) {
	credentials := git.NewTokenCredentials(group.Token.Value)
	seen := map[string]bool{}
	targets := make([]git.RemoteTarget, 0, len(group.Repos))

	for _, repoRef := range group.Repos {
		target, err := newRemoteTarget(group.Domain, repoRef, credentials)
		if err != nil {
			return nil, err
		}
		if seen[target.URL] {
			continue
		}
		seen[target.URL] = true
		targets = append(targets, target)
	}

	if len(group.Namespaces) == 0 {
		return targets, nil
	}
	if api == nil {
		return nil, fmt.Errorf("github api is required when namespaces are configured")
	}

	namespaces := make([]string, 0, len(group.Namespaces))
	for namespace := range group.Namespaces {
		namespaces = append(namespaces, namespace)
	}
	sort.Strings(namespaces)

	for _, namespace := range namespaces {
		namespaceTargets, err := collectNamespaceTargets(ctx, api.GetOwnerRepositories, group.Domain, namespace, group.Namespaces[namespace], credentials)
		if err != nil {
			return nil, err
		}
		for _, target := range namespaceTargets {
			if seen[target.URL] {
				continue
			}
			seen[target.URL] = true
			targets = append(targets, target)
		}
	}

	return targets, nil
}

func EmitTasks(ctx context.Context, api *API, group config.Group, builder *git.TaskBuilder) ([]taskrun.Task, error) {
	targets, err := Collect(ctx, api, group)
	if err != nil {
		return nil, err
	}
	return builder.EmitRemoteTargets(targets), nil
}

func collectNamespaceTargets(
	ctx context.Context,
	getRepos repositoriesGetter,
	domain string,
	namespace string,
	namespaceConfig *config.Namespace,
	credentials *git.Credentials,
) ([]git.RemoteTarget, error) {
	repos, err := getRepos(ctx, namespace)
	if err != nil {
		return nil, err
	}

	skip := set.New[string]()
	if namespaceConfig != nil {
		skip.Append(namespaceConfig.Skip...)
	}

	targets := make([]git.RemoteTarget, 0, len(repos))
	seen := map[string]bool{}
	for _, repo := range repos {
		if skip.Contains(repo.Name) {
			continue
		}

		repoRef := repo.FullName
		if repoRef == "" {
			repoRef = namespace + "/" + repo.Name
		}

		target, err := newRemoteTarget(domain, repoRef, credentials)
		if err != nil {
			return nil, err
		}
		if seen[target.URL] {
			continue
		}
		seen[target.URL] = true
		targets = append(targets, target)
	}

	return targets, nil
}

func newRemoteTarget(domain string, repoRef string, credentials *git.Credentials) (git.RemoteTarget, error) {
	owner, repo, err := config.SplitRepoRef(repoRef)
	if err != nil {
		return git.RemoteTarget{}, err
	}

	return git.RemoteTarget{
		URL:         fmt.Sprintf("https://%s/%s/%s.git", domain, owner, repo),
		Credentials: credentials,
	}, nil
}
