package github

func contains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}

func FilterRepos(repos []RepositoryResponse, ignoreList []string) []RepositoryResponse {
	keys := make(map[string]bool)
	list := []RepositoryResponse{}
	for _, repo := range repos {
		if contains(ignoreList, repo.Path) {
			continue
		}
		if _, value := keys[repo.Path]; !value {
			keys[repo.Path] = true
			list = append(list, repo)
		}
	}
	return list
}
