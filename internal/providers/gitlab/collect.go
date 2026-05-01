package gitlab

import (
	"context"
	"fmt"
	"rakun/internal/config"
	"rakun/internal/git"
	"rakun/internal/providers"
	"rakun/internal/taskrun"
	"rakun/pkg/set"
	"strings"
)

type projectsGetter func(ctx context.Context, groupPath string) ([]Project, error)

func Collect(ctx context.Context, api *API, group config.Group) ([]git.RemoteTarget, error) {
	credentials := git.NewTokenCredentials(group.Token.Value)
	var namespaceCollector providers.NamespaceTargetCollector
	if api != nil {
		namespaceCollector = func(ctx context.Context, namespace string, namespaceConfig *config.Namespace) ([]git.RemoteTarget, error) {
			return collectNamespaceTargets(ctx, api.GetGroupProjects, group.Domain, namespace, namespaceConfig, credentials)
		}
	}

	return providers.CollectTargets(ctx, "gitlab", group, func(repoRef string) (git.RemoteTarget, error) {
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

func collectNamespaceTargets(ctx context.Context, getProjects projectsGetter, domain string, namespace string, namespaceConfig *config.Namespace, credentials *git.Credentials) ([]git.RemoteTarget, error) {
	projects, err := getProjects(ctx, normalizePathRef(namespace))
	if err != nil {
		return nil, err
	}

	skipNames := set.New[string]()
	skipPaths := set.New[string]()
	if namespaceConfig != nil {
		for _, value := range namespaceConfig.Skip {
			normalized := normalizePathRef(value)
			if strings.Contains(normalized, "/") {
				skipPaths.Append(normalized)
				continue
			}
			skipNames.Append(normalized)
		}
	}

	targets := providers.NewTargetAccumulator(len(projects))
	for _, project := range projects {
		projectPath := normalizePathRef(project.PathWithNamespace)
		if projectPath == "" && project.Path != "" {
			projectPath = normalizePathRef(namespace) + "/" + normalizePathRef(project.Path)
		}
		if projectPath == "" {
			return nil, fmt.Errorf("gitlab project path is empty in namespace %q", namespace)
		}

		if skipPaths.Contains(projectPath) || skipNames.Contains(project.Path) {
			continue
		}

		target, err := newRemoteTarget(domain, projectPath, credentials)
		if err != nil {
			return nil, err
		}
		targets.Add(target)
	}

	return targets.Targets(), nil
}

func newRemoteTarget(domain string, projectRef string, credentials *git.Credentials) (git.RemoteTarget, error) {
	projectRef = normalizePathRef(projectRef)
	if err := config.ValidateGitLabProjectRef(projectRef); err != nil {
		return git.RemoteTarget{}, err
	}

	return git.RemoteTarget{
		URL:         fmt.Sprintf("https://%s/%s.git", domain, projectRef),
		Credentials: credentials,
	}, nil
}

func normalizePathRef(value string) string {
	return strings.Trim(strings.TrimSpace(value), "/")
}
