package gitlab

import (
	"context"
	"fmt"
	"rakun/internal/config"
	"rakun/internal/git"
	"rakun/internal/set"
	"rakun/internal/taskrun"
	"sort"
	"strings"
)

type projectsGetter func(ctx context.Context, groupPath string) ([]Project, error)

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
		return nil, fmt.Errorf("gitlab api is required when namespaces are configured")
	}

	namespaces := make([]string, 0, len(group.Namespaces))
	for namespace := range group.Namespaces {
		namespaces = append(namespaces, namespace)
	}
	sort.Strings(namespaces)

	for _, namespace := range namespaces {
		namespaceTargets, err := collectNamespaceTargets(ctx, api.GetGroupProjects, group.Domain, namespace, group.Namespaces[namespace], credentials)
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

func collectNamespaceTargets(ctx context.Context, getProjects projectsGetter, domain string, namespace string, namespaceConfig *config.Namespace, credentials *git.Credentials) ([]git.RemoteTarget, error) {
	projects, err := getProjects(ctx, normalizePathRef(namespace))
	if err != nil {
		return nil, err
	}

	skipNames := set.NewString(nil)
	skipPaths := set.NewString(nil)
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

	targets := make([]git.RemoteTarget, 0, len(projects))
	seen := map[string]bool{}
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
		if seen[target.URL] {
			continue
		}
		seen[target.URL] = true
		targets = append(targets, target)
	}

	return targets, nil
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
