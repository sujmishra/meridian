package registry

import (
	"context"
	"time"

	"github.com/sujmishra/meridian/packages/identity"
)

// MemRegistry is a single-node, in-memory Registry implementation.
// It is safe for concurrent use and delegates persistence to a Store.
// Pass a nil Verifier to skip attestation checking (dev/test mode only).
type MemRegistry struct {
	store    Store
	verifier identity.Verifier
}

// NewMemRegistry constructs a MemRegistry backed by the given store.
func NewMemRegistry(store Store, verifier identity.Verifier) *MemRegistry {
	return &MemRegistry{store: store, verifier: verifier}
}

func (r *MemRegistry) Register(ctx context.Context, record Record) error {
	if err := validateRecord(record); err != nil {
		return err
	}
	if r.verifier != nil && record.Attestation != "" {
		if _, err := r.verifier.Verify(ctx, record.Attestation, record.TrustRoot); err != nil {
			return ErrAttestationFail
		}
	}
	if _, err := r.store.Get(ctx, record.AgentURI); err == nil {
		return ErrAlreadyExists
	}
	now := time.Now().UTC()
	record.RegisteredAt = now
	record.UpdatedAt = now
	if record.SchemaVersion == "" {
		record.SchemaVersion = "1.0"
	}
	if record.Health == "" {
		record.Health = HealthUnknown
	}
	return r.store.Put(ctx, record)
}

func (r *MemRegistry) Update(ctx context.Context, agentURI identity.URI, patch RecordPatch) error {
	existing, err := r.store.Get(ctx, agentURI)
	if err != nil {
		return err
	}
	if r.verifier != nil && patch.Attestation != "" {
		if _, err := r.verifier.Verify(ctx, patch.Attestation, existing.TrustRoot); err != nil {
			return ErrAttestationFail
		}
	}
	if patch.Endpoints != nil {
		existing.Endpoints = patch.Endpoints
		existing.Protocols = make([]Protocol, 0, len(patch.Endpoints))
		for p := range patch.Endpoints {
			existing.Protocols = append(existing.Protocols, p)
		}
	}
	if patch.Health != "" {
		existing.Health = patch.Health
	}
	if patch.Attestation != "" {
		existing.Attestation = patch.Attestation
	}
	existing.UpdatedAt = time.Now().UTC()
	return r.store.Put(ctx, existing)
}

func (r *MemRegistry) Deregister(ctx context.Context, agentURI identity.URI) error {
	return r.store.Delete(ctx, agentURI)
}

func (r *MemRegistry) Get(ctx context.Context, agentURI identity.URI) (Record, error) {
	return r.store.Get(ctx, agentURI)
}

func (r *MemRegistry) Discover(ctx context.Context, q Query) ([]Record, error) {
	candidates, err := r.store.List(ctx, q.CapabilityPrefix)
	if err != nil {
		return nil, err
	}
	var results []Record
	for _, rec := range candidates {
		if q.Matches(rec) {
			results = append(results, rec)
			if q.Limit > 0 && len(results) >= q.Limit {
				break
			}
		}
	}
	return results, nil
}

func validateRecord(r Record) error {
	if r.AgentURI.TrustRoot == "" {
		return ErrInvalidRecord
	}
	if r.TrustRoot == "" || r.CapabilityPath == "" {
		return ErrInvalidRecord
	}
	if len(r.Protocols) == 0 {
		return ErrInvalidRecord
	}
	return nil
}
