package registry

import (
	"context"
	"strings"
	"sync"

	"github.com/sujmishra/meridian/packages/identity"
)

type memStore struct {
	mu      sync.RWMutex
	records map[string]Record // keyed by AgentURI.String()
}

// NewMemStore returns a thread-safe, in-memory Store.
// Suitable for development, testing, and single-node deployments.
func NewMemStore() Store {
	return &memStore{records: make(map[string]Record)}
}

func (s *memStore) Put(_ context.Context, record Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[record.AgentURI.String()] = record
	return nil
}

func (s *memStore) Get(_ context.Context, agentURI identity.URI) (Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.records[agentURI.String()]
	if !ok {
		return Record{}, ErrNotFound
	}
	return r, nil
}

func (s *memStore) Delete(_ context.Context, agentURI identity.URI) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.records[agentURI.String()]; !ok {
		return ErrNotFound
	}
	delete(s.records, agentURI.String())
	return nil
}

func (s *memStore) List(_ context.Context, capabilityPrefix string) ([]Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Record
	for _, r := range s.records {
		if capabilityPrefix == "" || strings.HasPrefix(r.CapabilityPath, capabilityPrefix) {
			out = append(out, r)
		}
	}
	return out, nil
}
