package git

import (
	"log"
	"path/filepath"
	"rakun/internal/fs"
)

func remoteName(remote string) string {
	base := filepath.Base(remote)
	return base[:len(base)-len(filepath.Ext(base))]
}

func Sync(path string, remotes []string) error {
	err := fs.CreateDirectory(path)
	if err != nil {
		return err
	}
	log.Printf("Synchronizing Git repositories to %s", path)
	for _, remote := range remotes {
		repo := Repository{
			Remote: remote,
			Path:   filepath.Join(path, remoteName(remote)),
		}
		if fs.Exists(repo.Path) {
			log.Printf("Updating %s", repo.Remote)
			err = repo.Pull()
			if err != nil {
				return err
			}
			continue
		}
		log.Printf("Cloning %s", repo.Remote)
		err = repo.Clone()
		if err != nil {
			return err
		}
	}
	return nil
}
