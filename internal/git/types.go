package git

type RepoDescription struct {
	Name string
	URL  string
}

type RepoGroup struct {
	Dir          string
	Repositories []RepoDescription
}
