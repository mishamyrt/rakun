package github

type SearchResponse struct {
	Count int                  `json:"total_count"`
	Items []RepositoryResponse `json:"items"`
}

type RepositoryResponse struct {
	URL  string `json:"html_url"`
	Path string `json:"full_name"`
	Name string `json:"name"`
}
