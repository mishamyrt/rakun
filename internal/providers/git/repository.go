package git

import (
	"context"
	"fmt"
	"path/filepath"
)

type Repository struct {
	Remote      string
	Path        string
	Credentials *Credentials
}

func (s *Repository) Clone(ctx context.Context) error {
	_, err := ExecGitWithCredentials(ctx, "clone", filepath.Dir(s.Path), []string{s.Remote, filepath.Base(s.Path)}, s.Remote, s.Credentials)
	return err
}

func (s *Repository) Fetch(ctx context.Context) error {
	_, err := ExecGitWithCredentials(ctx, "fetch", s.Path, []string{"--prune", "origin"}, s.Remote, s.Credentials)
	return err
}

func (s *Repository) Checkout(ctx context.Context, branch string) error {
	_, err := ExecGit(ctx, "checkout", s.Path, []string{"-f", branch})
	return err
}

func (s *Repository) ResetHard(ctx context.Context, ref string) error {
	_, err := ExecGit(ctx, "reset", s.Path, []string{"--hard", ref})
	return err
}

func (s *Repository) Clean(ctx context.Context) error {
	_, err := ExecGit(ctx, "clean", s.Path, []string{"-fdx"})
	return err
}

func (s *Repository) SetOriginURL(ctx context.Context) error {
	_, err := ExecGit(ctx, "remote", s.Path, []string{"set-url", "origin", s.Remote})
	return err
}

func (s *Repository) HeadCommit(ctx context.Context) (string, error) {
	return ExecGit(ctx, "rev-parse", s.Path, []string{"HEAD"})
}

func (s *Repository) SyncTo(ctx context.Context, branch string, commit string) error {
	if err := s.SetOriginURL(ctx); err != nil {
		return err
	}
	if err := s.Fetch(ctx); err != nil {
		return err
	}
	if err := s.Checkout(ctx, branch); err != nil {
		return err
	}
	if err := s.ResetHard(ctx, "origin/"+branch); err != nil {
		return err
	}
	if err := s.Clean(ctx); err != nil {
		return err
	}

	headCommit, err := s.HeadCommit(ctx)
	if err != nil {
		return err
	}
	if headCommit != commit {
		return fmt.Errorf("repository %s ended on commit %s instead of %s", s.Path, headCommit, commit)
	}
	return nil
}
