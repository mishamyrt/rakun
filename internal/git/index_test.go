package git

import (
	"path/filepath"
	"testing"
)

func TestIndexRoundTrip(t *testing.T) {
	output := t.TempDir()
	index := &Index{
		Version: IndexVersion,
		Repositories: map[string]RepositoryState{
			"github.com/example/repo.tar.gz": {
				Remote:      "https://github.com/example/repo.git",
				Branch:      "main",
				Commit:      "abc123",
				ArchivePath: "github.com/example/repo.tar.gz",
			},
		},
	}

	if err := index.Save(output); err != nil {
		t.Fatalf("save index: %v", err)
	}

	loaded, err := LoadIndex(output)
	if err != nil {
		t.Fatalf("load index: %v", err)
	}
	state, ok := loaded.Repositories["github.com/example/repo.tar.gz"]
	if !ok {
		t.Fatalf("missing saved repository state: %#v", loaded.Repositories)
	}
	if state.Commit != "abc123" {
		t.Fatalf("unexpected commit: %q", state.Commit)
	}
	if _, err := LoadIndex(filepath.Join(output, "missing")); err != nil {
		t.Fatalf("load missing index: %v", err)
	}
}
