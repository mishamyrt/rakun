package github

import (
	"context"
	"fmt"
	"rakun/internal/config"
	"rakun/internal/git"
	"rakun/internal/providers"
	"rakun/internal/taskrun"
	"rakun/pkg/set"
)

type repositoriesGetter func(ctx context.Context, owner string) ([]Repository, error)

func Collect(ctx context.Context, api *API, group config.Group) ([]git.RemoteTarget, error) {
	credentials := git.NewTokenCredentials(group.Token.Value)
	var namespaceCollector providers.NamespaceTargetCollector
	if api != nil {
		namespaceCollector = func(ctx context.Context, namespace string, namespaceConfig *config.Namespace) ([]git.RemoteTarget, error) {
			return collectNamespaceTargets(ctx, api.GetOwnerRepositories, group.Domain, namespace, namespaceConfig, credentials)
		}
	}

	return providers.CollectTargets(ctx, "github", group, func(repoRef string) (git.RemoteTarget, error) {
		return newRemoteTarget(group.Domain, repoRef, credentials)
	}, api != nil, namespaceCollector)
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

	targets := providers.NewTargetAccumulator(len(repos))
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
		targets.Add(target)
	}

	return targets.Targets(), nil
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
