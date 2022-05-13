package github

import (
	"path/filepath"
	"rakun/internal/fs"
	"rakun/internal/set"
)

func Sync(rootPath string, config Config) error {
	path := filepath.Join(rootPath, "github")
	err := fs.CreateDirectory(path)
	if err != nil {
		return err
	}
	ignore := set.CreateString(config.Ignore)
	api := API{
		Token: config.Token,
	}
	if len(config.Users) > 0 {
		err = syncUsersRemotes(path, api, config.Users, ignore)
		if err != nil {
			return err
		}
	}
	if len(config.Organizations) > 0 {
		err = syncOrgsRemotes(path, api, config.Organizations, ignore)
		if err != nil {
			return err
		}
	}
	// fmt.Println(list.Remotes)
	return nil
}
