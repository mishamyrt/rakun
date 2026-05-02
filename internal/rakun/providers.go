package rakun

import (
	"context"
	"fmt"
	"rakun/internal/config"
	"rakun/internal/git"
	githubprovider "rakun/internal/providers/github"
	gitlabprovider "rakun/internal/providers/gitlab"
)

type groupCollector interface {
	Collect(ctx context.Context, group config.Group) ([]git.RemoteTarget, error)
}

type groupCollectorFunc func(ctx context.Context, group config.Group) ([]git.RemoteTarget, error)

func (f groupCollectorFunc) Collect(ctx context.Context, group config.Group) ([]git.RemoteTarget, error) {
	return f(ctx, group)
}

var groupCollectors = map[string]groupCollector{
	"github": groupCollectorFunc(githubprovider.Collect),
	"gitlab": groupCollectorFunc(gitlabprovider.Collect),
}

func collectGroupTargets(ctx context.Context, group config.Group) ([]git.RemoteTarget, error) {
	collector, ok := groupCollectors[group.Type]
	if !ok {
		return nil, fmt.Errorf("unsupported source type %q", group.Type)
	}
	return collector.Collect(ctx, group)
}
