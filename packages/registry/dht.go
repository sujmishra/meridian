package registry

import (
	"context"
	"time"

	"github.com/sujmishra/meridian/packages/identity"
)

// DHT is the subset of the distributed hash-table interface the registry uses
// to keep decentralised endpoint resolution in sync with the authoritative
// store. Defined here (not in packages/dht) to avoid a circular import:
// packages/dht already imports packages/registry for the Protocol type.
type DHT interface {
	// Put stores or refreshes the endpoint map for an agent URI.
	Put(ctx context.Context, agentURI identity.URI, endpoints map[Protocol]string, ttl time.Duration) error

	// Delete removes the endpoint map for an agent URI.
	Delete(ctx context.Context, agentURI identity.URI) error

	// FindCapability returns agent URIs whose capability path starts with
	// capabilityPrefix. Used for Level-1 (coarse) discovery.
	FindCapability(ctx context.Context, capabilityPrefix string) ([]identity.URI, error)
}
