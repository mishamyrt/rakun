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

func syncRemote(remote string, path string) error {
	repo := Repository{
		Remote: remote,
		Path:   path,
	}
	resyncBroken := func() error {
		log.Printf("Repository %s is broken.", repo.Path)
		err := repo.Remove()
		if err != nil {
			return err
		}
		return syncRemote(remote, path)
	}
	if repo.Exists() {
		branch, err := repo.GetBranch()
		if err != nil {
			return resyncBroken()
		}
		hasChanges := repo.HasChanges()
		if hasChanges {
			log.Printf("Repository %s has changes", repo.Path)
			err = repo.ResetTo(branch)
			if err != nil {
				log.Println("Reset", err.Error())
				return resyncBroken()
			}
		}
		log.Printf("Updating %s", repo.Remote)
		err = repo.Pull()
		if err != nil {
			return err
		}
		// Check status after update
		if repo.HasChanges() {
			return resyncBroken()
		}
		return nil
	}
	log.Printf("Cloning %s", repo.Remote)
	err := repo.Clone()
	if err != nil {
		return err
	}
	return nil
}

func Sync(path string, remotes []string) error {
	err := fs.CreateDirectory(path)
	if err != nil {
		return err
	}
	log.Printf("Synchronizing Git repositories to %s", path)
	for _, remote := range remotes {
		repoPath := filepath.Join(path, remoteName(remote))
		err = syncRemote(remote, repoPath)
		if err != nil {
			log.Printf("Couldn't sync repository %s. '%s'", remote, err.Error())
		}
	}
	return nil
}
