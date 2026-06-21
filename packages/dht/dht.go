package dht

import (
	"context"
	"time"

	"github.com/sujmishra/meridian/packages/identity"
	"github.com/sujmishra/meridian/packages/registry"
)

// Node is a participant in the DHT network for a single trust root partition.
type Node interface {
	// Lookup resolves a stable agent:// URI to its current network endpoints.
	// Returns ErrNotFound if no mapping exists.
	Lookup(ctx context.Context, agentURI identity.URI) (map[registry.Protocol]string, error)

	// Put stores or updates the endpoint mapping for an agent URI.
	// Called by the registry after a successful Register or Update.
	Put(ctx context.Context, agentURI identity.URI, endpoints map[registry.Protocol]string, ttl time.Duration) error

	// Delete removes the mapping for an agent URI.
	// Called by the registry after a successful Deregister.
	Delete(ctx context.Context, agentURI identity.URI) error

	// FindCapability returns agent URIs whose capability path matches the prefix.
	// This is the Level-1 coarse discovery backed by DHT.
	FindCapability(ctx context.Context, capabilityPrefix string) ([]identity.URI, error)
}

// Config holds configuration for a DHT node.
type Config struct {
	// TrustRoot scopes this node to a single organization's partition.
	TrustRoot string

	// BootstrapPeers are known DHT node addresses used for initial routing table population.
	BootstrapPeers []string

	// ReplicationFactor is the number of nodes that store each key.
	ReplicationFactor int

	// TTL is the default time-to-live for DHT entries.
	TTL time.Duration
}

// ErrNotFound is returned when a lookup finds no matching entry.
var ErrNotFound = errorString("dht: agent URI not found")

type errorString string

func (e errorString) Error() string { return string(e) }
