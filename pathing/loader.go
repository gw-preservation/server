package pathing

import (
	"errors"
	"fmt"
	"sync"
)

var ErrNoArchive = errors.New("store has no backing archive")

// Store caches parsed navmesh data keyed by map file id; a lazy Store loads on
// demand, and loaded data is shared read-only across instances.
type Store struct {
	mu      sync.RWMutex
	archive *Archive // for on-demand loads; nil for eager/test stores
	data    map[uint32]*PathData
	failed  map[uint32]error // file ids that could not be read or parsed
}

func NewStore() *Store {
	return &Store{
		data:   make(map[uint32]*PathData),
		failed: make(map[uint32]error),
	}
}

func NewLazyStore(gwdatPath string) (*Store, error) {
	archive, err := Open(gwdatPath)
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	store := NewStore()
	store.archive = archive
	return store, nil
}

func (s *Store) EnsureLoaded(id uint32) (*PathData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sd, ok := s.data[id]; ok {
		return sd, nil
	}
	if err, ok := s.failed[id]; ok {
		return nil, err
	}
	if s.archive == nil {
		return nil, ErrNoArchive
	}
	content, err := s.archive.GetFile(id)
	if err != nil {
		err = fmt.Errorf("read map file 0x%08x: %w", id, err)
		s.failed[id] = err
		return nil, err
	}
	sd, err := ParsePathData(content)
	if err != nil {
		err = fmt.Errorf("map file 0x%08x: %w", id, err)
		s.failed[id] = err
		return nil, err
	}
	s.data[id] = sd
	return sd, nil
}

func (s *Store) PathDataForFileID(id uint32) *PathData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[id]
}

func (s *Store) Set(id uint32, data *PathData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[id] = data
}
