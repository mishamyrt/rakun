package git

import (
	"path"
)

type Repository struct {
	Remote string
	Path   string
}

func (s *Repository) Clone() error {
	return ExecGit("clone", path.Dir(s.Path), s.Remote)
}

func (s *Repository) Pull() error {
	return ExecGit("pull", s.Path)
}
