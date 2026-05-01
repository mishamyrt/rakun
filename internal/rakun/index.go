package rakun

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// IndexFileName is the name of the index file used to store repository state.
const IndexFileName = ".rakun-index.json"

// IndexVersion is the version of the index file format.
const IndexVersion = 1

// RepositoryState holds the state of a repository, including its remote URL, branch, commit, and archive path.
type RepositoryState struct {
	Remote      string    `json:"remote"`
	Branch      string    `json:"branch"`
	Commit      string    `json:"commit"`
	ArchivePath string    `json:"archivePath"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Index holds the state of all repositories, including their remote URL,
// branch, commit, and archive path.
type Index struct {
	Version      int                        `json:"version"`
	Repositories map[string]RepositoryState `json:"repositories"`
}

// Clone returns a deep copy of the Index.
func (i *Index) Clone() *Index {
	repositories := make(map[string]RepositoryState, len(i.Repositories))
	for archivePath, state := range i.Repositories {
		repositories[archivePath] = state
	}
	return &Index{
		Version:      i.Version,
		Repositories: repositories,
	}
}

// LoadIndex loads the index from the output directory.
// If the index file does not exist, a new index is returned.
func LoadIndex(output string) (*Index, error) {
	indexPath := filepath.Join(output, IndexFileName)
	if !exists(indexPath) {
		return &Index{
			Version:      IndexVersion,
			Repositories: map[string]RepositoryState{},
		}, nil
	}

	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}

	var index Index
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}
	if index.Version == 0 {
		index.Version = IndexVersion
	}
	if index.Repositories == nil {
		index.Repositories = map[string]RepositoryState{}
	}
	return &index, nil
}

// Save saves the index to the output directory.
func (i *Index) Save(output string) error {
	indexPath := filepath.Join(output, IndexFileName)
	data, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := indexPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, indexPath)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
