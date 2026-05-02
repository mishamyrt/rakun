package github

import (
	"context"
	"fmt"
	"rakun/internal/config"
	"rakun/internal/git"
	"rakun/internal/providers"
	"rakun/pkg/set"
)

type repositoriesGetter func(ctx context.Context, owner string) ([]Repository, error)

type ownerRepositoriesAPI interface {
	GetOwnerRepositories(ctx context.Context, owner string) ([]Repository, error)
}

// Collect resolves GitHub repository targets from the configured group.
func Collect(ctx context.Context, group config.Group) ([]git.RemoteTarget, error) {
	var api ownerRepositoriesAPI
	if len(group.Namespaces) > 0 {
		createdAPI, err := NewAPI(APIBaseURL(group.Domain), group.Token.Value)
		if err != nil {
			return nil, err
		}
		api = createdAPI
	}

	return collect(ctx, group, api)
}

func collect(ctx context.Context, group config.Group, api ownerRepositoriesAPI) ([]git.RemoteTarget, error) {
	var namespaceCollector providers.CredentialsNamespaceTargetCollector
	if api != nil {
		namespaceCollector = func(ctx context.Context, namespace string, namespaceConfig *config.Namespace, credentials *git.Credentials) ([]git.RemoteTarget, error) {
			return collectNamespaceTargets(ctx, api.GetOwnerRepositories, group.Domain, namespace, namespaceConfig, credentials)
		}
	}

	return providers.CollectGroupTargets(ctx, "github", group, func(repoRef string, credentials *git.Credentials) (git.RemoteTarget, error) {
		return newRemoteTarget(group.Domain, repoRef, credentials)
	}, namespaceCollector)
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
