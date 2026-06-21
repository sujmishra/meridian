package registry

import (
	"context"
	"errors"

	"github.com/sujmishra/meridian/packages/identity"
)

var (
	ErrNotFound         = errors.New("registry: agent not found")
	ErrAlreadyExists    = errors.New("registry: agent already registered")
	ErrInvalidRecord    = errors.New("registry: record failed validation")
	ErrAttestationFail  = errors.New("registry: attestation verification failed")
)

// Registry is the write-authoritative interface for agent records.
// All mutating operations require a valid PASETO attestation.
type Registry interface {
	// Register adds a new agent record. Returns ErrAlreadyExists if the agent URI
	// is already registered. Validates the attestation before writing.
	Register(ctx context.Context, record Record) error

	// Update replaces an existing agent's mutable fields (endpoints, health).
	// The AgentURI and TrustRoot are immutable after registration.
	Update(ctx context.Context, agentURI identity.URI, patch RecordPatch) error

	// Deregister removes an agent record. Requires attestation from the same trust root.
	Deregister(ctx context.Context, agentURI identity.URI) error

	// Get returns the record for a specific agent URI.
	Get(ctx context.Context, agentURI identity.URI) (Record, error)

	// Discover returns agents matching the query. See package-level Query type.
	Discover(ctx context.Context, q Query) ([]Record, error)
}

// RecordPatch carries the mutable fields that can be updated after registration.
// Only non-zero fields are applied.
type RecordPatch struct {
	Endpoints map[Protocol]string
	Health    HealthStatus
	// Attestation must be refreshed when endpoints change.
	Attestation string
}

// Store is the persistence interface consumed by the Registry implementation.
// Implementations may be in-memory, PostgreSQL, etcd, etc.
type Store interface {
	Put(ctx context.Context, record Record) error
	Get(ctx context.Context, agentURI identity.URI) (Record, error)
	Delete(ctx context.Context, agentURI identity.URI) error
	// List returns all records matching the given capability prefix.
	// An empty prefix returns all records.
	List(ctx context.Context, capabilityPrefix string) ([]Record, error)
}
