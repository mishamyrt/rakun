package rakun

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"rakun/internal/archive"
	"rakun/internal/fs"
	"rakun/internal/git"
	"rakun/internal/taskrun"
	"time"
)

type syncTask struct {
	output      string
	spec        git.RemoteSpec
	credentials *git.Credentials
	store       *Store
}

func (s syncTask) ID() string {
	return s.spec.ArchiveRelativePath
}

func (s syncTask) Title() string {
	return s.spec.DisplayName
}

func (s syncTask) Run(ctx context.Context, reporter taskrun.Reporter) taskrun.Result {
	reporter.Stage(0.08, "Resolving remote HEAD")
	archivePath := filepath.Join(s.output, s.spec.ArchiveRelativePath)
	remoteHead, err := git.ResolveRemoteHead(ctx, s.spec.Remote, s.credentials)
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
		} else if !errors.Is(err, archive.ErrInvalid) {
			return taskrun.Result{Error: err}
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

func syncArchive(ctx context.Context, archivePath string, spec git.RemoteSpec, credentials *git.Credentials, remoteHead git.RemoteHead, reporter taskrun.Reporter) (err error) {
	tempDir, err := os.MkdirTemp("", "rakun-archive-*")
	if err != nil {
		return err
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); err == nil {
			err = removeErr
		}
	}()

	reporter.Stage(0.32, "Extracting archive")
	repoPath, err := archive.ExtractArchive(archivePath, tempDir)
	if err != nil {
		return err
	}

	repo := git.Repository{
		Remote:      spec.Remote,
		Path:        repoPath,
		Credentials: credentials,
	}
	reporter.Stage(0.64, "Fetching updates")
	if err := repo.SyncTo(ctx, remoteHead.Branch, remoteHead.Commit); err != nil {
		return err
	}
	reporter.Stage(0.86, "Packing archive")
	return archive.CreateArchive(archivePath, repo.Path)
}

func rebuildArchive(ctx context.Context, archivePath string, spec git.RemoteSpec, credentials *git.Credentials, remoteHead git.RemoteHead, reporter taskrun.Reporter) (err error) {
	tempDir, err := os.MkdirTemp("", "rakun-clone-*")
	if err != nil {
		return err
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); err == nil {
			err = removeErr
		}
	}()

	repo := git.Repository{
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
	return archive.CreateArchive(archivePath, repo.Path)
}
