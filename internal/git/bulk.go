package git

import (
	"git_sync/internal/fs"
	"log"
	"os"
)

func SyncGroup(dir string, group RepoGroup) error {
	groupPath := dir + "/" + group.Dir
	err := os.MkdirAll(groupPath, 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}
	log.Println("Syncing " + groupPath)
	for _, repo := range group.Repositories {
		repoPath := groupPath + "/" + repo.Name
		if fs.Exists(repoPath) {
			log.Println("Pulling " + repo.Name + "...")
			Pull(repoPath)
			continue
		}
		log.Println("Cloning " + repo.Name + "...")
		Clone(repo.URL, groupPath)
	}
	return nil
}

func SyncRepos(dir string, groups []RepoGroup) error {
	for _, group := range groups {
		err := SyncGroup(dir, group)
		if err != nil {
			return err
		}
	}
	return nil
}
