package git

import (
	"path"
	"rakun/internal/fs"
)

type Repository struct {
	Remote string
	Path   string
}

func (s *Repository) Clone() error {
	_, err := ExecGit("clone", path.Dir(s.Path), []string{s.Remote})
	return err
}

func (s *Repository) Pull() error {
	_, err := ExecGit("pull", s.Path, []string{})
	return err
}

func (s *Repository) ResetTo(branch string) error {
	_, err := ExecGit("checkout", s.Path, []string{"-f", branch})
	return err
}

func (s *Repository) GetBranch() (string, error) {
	return ExecGit("rev-parse", s.Path, []string{"--abbrev-ref", "HEAD"})
}

func (s *Repository) HasChanges() bool {
	data, _ := ExecGit("diff", s.Path, []string{"--name-only"})
	return len(data) > 0
}

func (s *Repository) Exists() bool {
	return fs.Exists(s.Path)
}

func (s *Repository) Remove() error {
	return fs.RemoveDirectory(s.Path)
}
