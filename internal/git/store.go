package git

import "sync"

type IndexStore struct {
	index  *Index
	dirty  bool
	mu     sync.Mutex
	output string
}

func LoadIndexStore(output string) (*IndexStore, error) {
	index, err := LoadIndex(output)
	if err != nil {
		return nil, err
	}
	return &IndexStore{
		index:  index,
		output: output,
	}, nil
}

func (s *IndexStore) Current(archivePath string) (RepositoryState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.index.Repositories[archivePath]
	return state, ok
}

func (s *IndexStore) SaveState(state RepositoryState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.index.Repositories[state.ArchivePath] = state
	s.dirty = true
	return nil
}

func (s *IndexStore) Flush() error {
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
