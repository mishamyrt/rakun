package github

import (
	"path/filepath"
	"rakun/internal/set"
	"rakun/providers/git"
)

func collectRemotes(getRepos RepositoriesGetter, param string, ignore set.String) ([]string, error) {
	repos, err := getRepos(param)
	if err != nil {
		return nil, err
	}
	remotes := set.String{}
	for _, repo := range repos {
		if ignore.Contains(param + "/" + repo.Name) {
			continue
		}
		remotes.Append(repo.URL)
	}
	return remotes.Values(), nil
}

func syncRemotes(
	path string,
	getRepos RepositoriesGetter,
	params []string,
	ignore set.String,
) error {
	for _, param := range params {
		remotes, err := collectRemotes(getRepos, param, ignore)
		if err != nil {
			return err
		}
		paramPath := filepath.Join(path, param)
		git.Sync(paramPath, remotes)
		// remotes := set.String{}
		// for _, repo := range repos {
		// 	remotes.Append(repo.URL)
		// }
	}
	return nil
}

func syncOrgsRemotes(path string, api API, orgs []string, ignore set.String) error {
	return syncRemotes(path, api.GetOrgRepositories, orgs, ignore)
}

func syncUsersRemotes(path string, api API, users []string, ignore set.String) error {
	return syncRemotes(path, api.GetUserRepositories, users, ignore)
}
