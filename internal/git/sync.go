package git

import (
	"context"
	"os"
	"path/filepath"
	"rakun/internal/fs"
	"rakun/internal/taskrun"
	"time"
)

type TaskBuilder struct {
	output string
	seen   map[string]bool
	store  *IndexStore
}

type SyncTask struct {
	output      string
	spec        RemoteSpec
	credentials *Credentials
	store       *IndexStore
}

func NewTaskBuilder(output string) (*TaskBuilder, error) {
	err := os.MkdirAll(output, 0755)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}

	store, err := LoadIndexStore(output)
	if err != nil {
		return nil, err
	}
	return &TaskBuilder{
		output: output,
		seen:   map[string]bool{},
		store:  store,
	}, nil
}

func (s *TaskBuilder) EmitRemoteTasks(remotes []string) []taskrun.Task {
	targets := make([]RemoteTarget, 0, len(remotes))
	for _, remote := range remotes {
		targets = append(targets, RemoteTarget{URL: remote})
	}
	return s.EmitRemoteTargets(targets)
}

func (s *TaskBuilder) EmitRemoteTargets(targets []RemoteTarget) []taskrun.Task {
	tasks := make([]taskrun.Task, 0, len(targets))
	for _, target := range targets {
		task := s.EmitRemoteTarget(target)
		if task == nil {
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks
}

func (s *TaskBuilder) EmitRemoteTarget(target RemoteTarget) taskrun.Task {
	spec, err := ParseRemote(target.URL)
	if err != nil {
		return taskrun.NewErrorTask(target.URL, target.URL, err)
	}
	if s.seen[spec.ArchiveRelativePath] {
		return nil
	}
	s.seen[spec.ArchiveRelativePath] = true

	return SyncTask{
		output:      s.output,
		spec:        spec,
		credentials: target.Credentials,
		store:       s.store,
	}
}

func (s *TaskBuilder) EmitRemoteTask(remote string) taskrun.Task {
	return s.EmitRemoteTarget(RemoteTarget{URL: remote})
}

func (s *TaskBuilder) Flush() error {
	return s.store.Flush()
}

func (s SyncTask) ID() string {
	return s.spec.ArchiveRelativePath
}

func (s SyncTask) Title() string {
	return s.spec.DisplayName
}

func (s SyncTask) Run(ctx context.Context, reporter taskrun.Reporter) taskrun.Result {
	reporter.Stage(0.08, "Resolving remote HEAD")
	archivePath := filepath.Join(s.output, s.spec.ArchiveRelativePath)
	remoteHead, err := ResolveRemoteHead(ctx, s.spec.Remote, s.credentials)
	if err != nil {
		return taskrun.Result{Error: err}
	}

	currentState, hasState := s.store.Current(s.spec.ArchiveRelativePath)
	if hasState &&
		currentState.Commit == remoteHead.Commit &&
		currentState.Branch == remoteHead.Branch &&
		fs.Exists(archivePath) {
		return taskrun.Result{
			Changed: false,
			Summary: "Up to date",
		}
	}

	state := RepositoryState{
		Remote:      s.spec.Remote,
		Branch:      remoteHead.Branch,
		Commit:      remoteHead.Commit,
		ArchivePath: s.spec.ArchiveRelativePath,
		UpdatedAt:   time.Now().UTC(),
	}

	if fs.Exists(archivePath) {
		if err := syncArchive(ctx, archivePath, s.spec, s.credentials, remoteHead, reporter); err == nil {
			if err := s.store.SaveState(state); err != nil {
				return taskrun.Result{Error: err}
			}
			return taskrun.Result{
				Changed: true,
				Summary: "Updated archive",
			}
		}
		reporter.Stage(0.22, "Archive invalid, recloning")
	}

	status := "Created archive"
	if fs.Exists(archivePath) {
		status = "Rebuilt archive"
	}
	if err := rebuildArchive(ctx, archivePath, s.spec, s.credentials, remoteHead, reporter); err != nil {
		return taskrun.Result{Error: err}
	}
	if err := s.store.SaveState(state); err != nil {
		return taskrun.Result{Error: err}
	}
	return taskrun.Result{
		Changed: true,
		Summary: status,
	}
}

func syncArchive(ctx context.Context, archivePath string, spec RemoteSpec, credentials *Credentials, remoteHead RemoteHead, reporter taskrun.Reporter) error {
	tempDir, err := os.MkdirTemp("", "rakun-archive-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	reporter.Stage(0.32, "Extracting archive")
	repoPath, err := ExtractArchive(archivePath, tempDir)
	if err != nil {
		return err
	}

	repo := Repository{
		Remote:      spec.Remote,
		Path:        repoPath,
		Credentials: credentials,
	}
	reporter.Stage(0.64, "Fetching updates")
	if err := repo.SyncTo(ctx, remoteHead.Branch, remoteHead.Commit); err != nil {
		return err
	}
	reporter.Stage(0.86, "Packing archive")
	return CreateArchive(archivePath, repo.Path)
}

func rebuildArchive(ctx context.Context, archivePath string, spec RemoteSpec, credentials *Credentials, remoteHead RemoteHead, reporter taskrun.Reporter) error {
	tempDir, err := os.MkdirTemp("", "rakun-clone-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	repo := Repository{
		Remote:      spec.Remote,
		Path:        filepath.Join(tempDir, spec.RepositoryName),
		Credentials: credentials,
	}
	reporter.Stage(0.32, "Cloning repository")
	if err := repo.Clone(ctx); err != nil {
		return err
	}
	reporter.Stage(0.64, "Aligning checkout")
	if err := repo.SyncTo(ctx, remoteHead.Branch, remoteHead.Commit); err != nil {
		return err
	}
	reporter.Stage(0.86, "Packing archive")
	return CreateArchive(archivePath, repo.Path)
}
