package rakun

import (
	"context"
	"errors"
	"os"
	"rakun/internal/config"
	"rakun/internal/git"
	"rakun/internal/taskrun"
	"rakun/pkg/set"

	"golang.org/x/sync/errgroup"
)

// Rakun is the main struct that holds the configuration and state for running tasks.
type Rakun struct {
	jobs   int
	output string
	seen   set.Set[string]
	store  *Store
}

// New creates a new Rakun instance with the given output directory and job concurrency.
func New(output string, jobs int) (*Rakun, error) {
	err := os.MkdirAll(output, 0755)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}

	store, err := LoadStore(output)
	if err != nil {
		return nil, err
	}
	return &Rakun{
		jobs:   jobs,
		output: output,
		seen:   set.New[string](),
		store:  store,
	}, nil
}

// Collect collects tasks from the given groups and returns them as a slice.
func (r *Rakun) Collect(ctx context.Context, groups []config.Group) ([]taskrun.Task, error) {
	r.seen = set.New[string]()
	groupTargets := make([][]git.RemoteTarget, len(groups))
	collectGroup, collectCtx := errgroup.WithContext(ctx)

	for i, group := range groups {
		i := i
		group := group
		collectGroup.Go(func() error {
			targets, err := collectGroupTargets(collectCtx, group)
			if err != nil {
				return err
			}
			groupTargets[i] = targets
			return nil
		})
	}

	if err := collectGroup.Wait(); err != nil {
		return nil, err
	}

	tasks := []taskrun.Task{}
	for _, targets := range groupTargets {
		tasks = append(tasks, r.emitRemoteTargets(targets)...)
	}

	return tasks, nil
}

// Run executes the given tasks using the configured job concurrency and observer.
func (r *Rakun) Run(ctx context.Context, tasks []taskrun.Task, observer taskrun.Observer) (taskrun.Summary, error) {
	summary, executeErr := taskrun.Execute(ctx, tasks, r.jobs, observer)
	flushErr := r.store.Flush()
	if err := errors.Join(executeErr, flushErr); err != nil {
		return summary, err
	}
	return summary, nil
}

func (r *Rakun) emitRemoteTasks(remotes []string) []taskrun.Task {
	targets := make([]git.RemoteTarget, 0, len(remotes))
	for _, remote := range remotes {
		targets = append(targets, git.RemoteTarget{URL: remote})
	}
	return r.emitRemoteTargets(targets)
}

func (r *Rakun) emitRemoteTargets(targets []git.RemoteTarget) []taskrun.Task {
	tasks := make([]taskrun.Task, 0, len(targets))
	for _, target := range targets {
		task := r.emitRemoteTarget(target)
		if task == nil {
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks
}

func (r *Rakun) emitRemoteTarget(target git.RemoteTarget) taskrun.Task {
	spec, err := git.ParseRemote(target.URL)
	if err != nil {
		return taskrun.NewErrorTask(target.URL, target.URL, err)
	}
	if r.seen.Contains(spec.ArchiveRelativePath) {
		return nil
	}
	r.seen.Append(spec.ArchiveRelativePath)

	return syncTask{
		output:      r.output,
		spec:        spec,
		credentials: target.Credentials,
		store:       r.store,
	}
}
