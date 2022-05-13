package github

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const APIUrl = "https://api.github.com"
const NextPageTag = "rel=\"next\""

type SearchResponse struct {
	Count int          `json:"total_count"`
	Items []Repository `json:"items"`
}

type Repository struct {
	URL  string `json:"html_url"`
	Path string `json:"full_name"`
	Name string `json:"name"`
}

type RepositoriesGetter func(param string) ([]Repository, error)

type API struct {
	Token string
}

func (s *API) GetOrgRepositories(orgName string) ([]Repository, error) {
	return s.SearchRepositories("org:" + orgName)
}

func (s *API) GetUserRepositories(userName string) ([]Repository, error) {
	return s.SearchRepositories("user:" + userName)
}

func (s *API) SearchRepositories(query string) ([]Repository, error) {
	var items []Repository
	page := 1
	for {
		pageItems, isLast, err := s.GetRepositoriesPage(
			APIUrl+"/search/repositories?q="+query,
			page,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, pageItems...)
		if isLast {
			break
		}
		page++
	}
	return items, nil
}

func (s *API) GetRepositoriesPage(url string, page int) ([]Repository, bool, error) {
	// Perform API request
	client := &http.Client{
		Timeout: time.Second * 10,
	}
	req, err := http.NewRequest("GET", url+"&page="+strconv.Itoa(page), nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Authorization", "token "+s.Token)
	response, err := client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer response.Body.Close()
	// Parse response
	bodyBytes, err := io.ReadAll(response.Body)
	var itemsResponse SearchResponse
	err = json.Unmarshal(bodyBytes, &itemsResponse)
	if err != nil {
		return nil, false, err
	}
	isLast := len(response.Header["Link"]) == 0 || !strings.Contains(response.Header["Link"][0], NextPageTag)
	return itemsResponse.Items, isLast, nil
}
