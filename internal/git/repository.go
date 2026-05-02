package git

import (
	"context"
	"fmt"
	"path/filepath"
)

// Repository describes a local checkout and how to access its remote.
type Repository struct {
	Remote      string
	Path        string
	Credentials *Credentials
}

// Clone clones the repository into its configured path.
func (s *Repository) Clone(ctx context.Context) error {
	return s.CloneBranch(ctx, "")
}

// CloneBranch clones the repository and checks out the requested branch.
func (s *Repository) CloneBranch(ctx context.Context, branch string) error {
	args := make([]string, 0, 4)
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, s.Remote, filepath.Base(s.Path))

	_, err := ExecGitWithCredentials(ctx, "clone", filepath.Dir(s.Path), args, s.Remote, s.Credentials)
	return err
}

// Fetch fetches and prunes refs from origin.
func (s *Repository) Fetch(ctx context.Context) error {
	_, err := ExecGitWithCredentials(ctx, "fetch", s.Path, []string{"--prune", "origin"}, s.Remote, s.Credentials)
	return err
}

// Checkout force-checks out branch in the local repository.
func (s *Repository) Checkout(ctx context.Context, branch string) error {
	_, err := ExecGit(ctx, "checkout", s.Path, []string{"-f", branch})
	return err
}

// ResetHard resets the repository to ref with --hard.
func (s *Repository) ResetHard(ctx context.Context, ref string) error {
	_, err := ExecGit(ctx, "reset", s.Path, []string{"--hard", ref})
	return err
}

// Clean removes untracked files from the repository.
func (s *Repository) Clean(ctx context.Context) error {
	_, err := ExecGit(ctx, "clean", s.Path, []string{"-fdx"})
	return err
}

// SetOriginURL updates the origin remote URL to match Repository.Remote.
func (s *Repository) SetOriginURL(ctx context.Context) error {
	_, err := ExecGit(ctx, "remote", s.Path, []string{"set-url", "origin", s.Remote})
	return err
}

// HeadCommit returns the current HEAD commit hash.
func (s *Repository) HeadCommit(ctx context.Context) (string, error) {
	return ExecGit(ctx, "rev-parse", s.Path, []string{"HEAD"})
}

// VerifyHeadCommit ensures the current HEAD matches commit.
func (s *Repository) VerifyHeadCommit(ctx context.Context, commit string) error {
	headCommit, err := s.HeadCommit(ctx)
	if err != nil {
		return err
	}
	if headCommit != commit {
		return fmt.Errorf("repository %s ended on commit %s instead of %s", s.Path, headCommit, commit)
	}
	return nil
}

// SyncTo syncs the repository to origin/branch and verifies the resulting commit.
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

	return s.VerifyHeadCommit(ctx, commit)
}
