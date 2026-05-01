package rakun

import "sync"

// Store keeps repository state in memory and flushes it to disk.
type Store struct {
	index  *Index
	dirty  bool
	mu     sync.Mutex
	output string
}

// LoadStore loads the store for the given output path.
func LoadStore(output string) (*Store, error) {
	index, err := LoadIndex(output)
	if err != nil {
		return nil, err
	}
	return &Store{
		index:  index,
		output: output,
	}, nil
}

// Current returns the stored state for archivePath, if present.
func (s *Store) Current(archivePath string) (RepositoryState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.index.Repositories[archivePath]
	return state, ok
}

// SaveState stores a repository state in memory and marks the store dirty.
func (s *Store) SaveState(state RepositoryState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.index.Repositories[state.ArchivePath] = state
	s.dirty = true
	return nil
}

// Flush persists pending store changes to disk.
func (s *Store) Flush() error {
	s.mu.Lock()
	if !s.dirty {
		s.mu.Unlock()
		return nil
	}

	snapshot := s.index.Clone()
	s.dirty = false
	s.mu.Unlock()

	if err := snapshot.Save(s.output); err != nil {
		s.mu.Lock()
		s.dirty = true
		s.mu.Unlock()
		return err
	}
	return nil
}
