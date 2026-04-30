package git

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const IndexFileName = ".rakun-index.json"
const IndexVersion = 1

type RepositoryState struct {
	Remote      string    `json:"remote"`
	Branch      string    `json:"branch"`
	Commit      string    `json:"commit"`
	ArchivePath string    `json:"archivePath"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Index struct {
	Version      int                        `json:"version"`
	Repositories map[string]RepositoryState `json:"repositories"`
}

func LoadIndex(output string) (*Index, error) {
	indexPath := filepath.Join(output, IndexFileName)
	if !fsExists(indexPath) {
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

func fsExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
