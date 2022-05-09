package github

import (
	"git_sync/internal/git"
)

func toDescriptions(repos []RepositoryResponse) []git.RepoDescription {
	descriptions := []git.RepoDescription{}
	for _, repo := range repos {
		descriptions = append(descriptions, git.RepoDescription{
			Name: repo.Name,
			URL:  repo.URL,
		})
	}
	return descriptions
}

func CollectOrgsRepositories(api API, orgs []string, ignore []string) ([]git.RepoGroup, error) {
	groups := []git.RepoGroup{}
	for _, org := range orgs {
		repos, err := api.GetUserRepositories(org)
		if err != nil {
			return nil, err
		}
		groups = append(groups, git.RepoGroup{
			Dir: "github/" + org,
			Repositories: toDescriptions(
				FilterRepos(repos, ignore),
			),
		})
	}
	return groups, nil
}

func CollectUsersRepositories(api API, users []string, ignore []string) ([]git.RepoGroup, error) {
	groups := []git.RepoGroup{}
	for _, userName := range users {
		repos, err := api.GetUserRepositories(userName)
		if err != nil {
			return nil, err
		}
		groups = append(groups, git.RepoGroup{
			Dir: "github/" + userName,
			Repositories: toDescriptions(
				FilterRepos(repos, ignore),
			),
		})
	}
	return groups, nil
}

func CollectRepositories(config Config) ([]git.RepoGroup, error) {
	groups := []git.RepoGroup{}
	api := API{config.Token}
	if len(config.Users) > 0 {
		repos, err := CollectUsersRepositories(api, config.Users, config.Ignore)
		if err != nil {
			return nil, err
		}
		groups = append(groups, repos...)
	}
	if len(config.Organizations) > 0 {
		repos, err := CollectOrgsRepositories(api, config.Organizations, config.Ignore)
		if err != nil {
			return nil, err
		}
		groups = append(groups, repos...)
	}
	return groups, nil
}
