package dht

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/sujmishra/meridian/packages/identity"
	"github.com/sujmishra/meridian/packages/registry"
)

type endpointEntry struct {
	endpoints map[registry.Protocol]string
	expiresAt time.Time // zero means no expiry
}

type memNode struct {
	mu      sync.RWMutex
	entries map[string]endpointEntry // keyed by agentURI.String()
}

// NewMemNode returns an in-memory DHT Node for single-node and development use.
// It stores endpoint mappings locally and supports prefix-based capability lookup.
// Replace with a real distributed DHT implementation for multi-node deployments.
func NewMemNode() Node {
	return &memNode{entries: make(map[string]endpointEntry)}
}

func (n *memNode) Lookup(_ context.Context, agentURI identity.URI) (map[registry.Protocol]string, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	e, ok := n.entries[agentURI.String()]
	if !ok {
		return nil, ErrNotFound
	}
	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		return nil, ErrNotFound
	}
	out := make(map[registry.Protocol]string, len(e.endpoints))
	for k, v := range e.endpoints {
		out[k] = v
	}
	return out, nil
}

func (n *memNode) Put(_ context.Context, agentURI identity.URI, endpoints map[registry.Protocol]string, ttl time.Duration) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	n.entries[agentURI.String()] = endpointEntry{endpoints: endpoints, expiresAt: exp}
	return nil
}

func (n *memNode) Delete(_ context.Context, agentURI identity.URI) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	delete(n.entries, agentURI.String())
	return nil
}

func (n *memNode) FindCapability(_ context.Context, capabilityPrefix string) ([]identity.URI, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	prefix := strings.TrimRight(capabilityPrefix, "/")
	now := time.Now()
	var out []identity.URI
	for key, e := range n.entries {
		if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
			continue
		}
		uri, err := identity.Parse(key)
		if err != nil {
			continue
		}
		if strings.HasPrefix(uri.CapabilityPath, prefix) {
			out = append(out, uri)
		}
	}
	return out, nil
}
