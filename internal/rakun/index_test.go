package rakun

import (
	"os"
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

func TestStoreFlushPersistsDirtyState(t *testing.T) {
	output := t.TempDir()
	store, err := LoadStore(output)
	if err != nil {
		t.Fatalf("load store: %v", err)
	}

	state := RepositoryState{
		Remote:      "https://github.com/example/repo.git",
		Branch:      "main",
		Commit:      "abc123",
		ArchivePath: "github.com/example/repo.tar.gz",
	}
	if err := store.SaveState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	if _, err := os.Stat(filepath.Join(output, IndexFileName)); !os.IsNotExist(err) {
		t.Fatalf("index should not be persisted before flush: %v", err)
	}

	if err := store.Flush(); err != nil {
		t.Fatalf("flush store: %v", err)
	}

	index, err := LoadIndex(output)
	if err != nil {
		t.Fatalf("load flushed index: %v", err)
	}
	if index.Repositories[state.ArchivePath].Commit != state.Commit {
		t.Fatalf("unexpected flushed state: %#v", index.Repositories[state.ArchivePath])
	}
}
