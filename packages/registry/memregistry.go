package registry

import (
	"context"
	"log/slog"
	"time"

	"github.com/sujmishra/meridian/packages/identity"
)

// MemRegistry is a single-node, in-memory Registry implementation.
// It is safe for concurrent use and delegates persistence to a Store.
//
// Pass a non-nil DHT to enable write-through to the DHT and two-level
// discovery. Pass nil verifier to skip attestation checking (dev/test only).
type MemRegistry struct {
	store    Store
	verifier identity.Verifier
	dht      DHT // nil disables DHT write-through and forces store-scan discovery
}

// dhtTTL is the time-to-live for DHT endpoint entries.
// Zero means no expiry in MemNode. Replace with a lease-derived value once
// TTL/lease renewal is implemented.
const dhtTTL = 0

// NewMemRegistry constructs a MemRegistry backed by the given store.
// verifier may be nil (disables attestation checks — dev/test only).
// dht may be nil (disables DHT integration; discovery falls back to store scan).
func NewMemRegistry(store Store, verifier identity.Verifier, dht DHT) *MemRegistry {
	return &MemRegistry{store: store, verifier: verifier, dht: dht}
}

func (r *MemRegistry) Register(ctx context.Context, record Record) error {
	if err := validateRecord(record); err != nil {
		return err
	}
	// When a verifier is configured a signed attestation is always required.
	if r.verifier != nil {
		if record.Attestation == "" {
			return ErrAttestationFail
		}
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
	if err := r.store.Put(ctx, record); err != nil {
		return err
	}
	if r.dht != nil {
		if err := r.dht.Put(ctx, record.AgentURI, record.Endpoints, dhtTTL); err != nil {
			slog.Warn("dht: failed to publish agent after registration",
				"uri", record.AgentURI.String(), "err", err)
		}
	}
	return nil
}

func (r *MemRegistry) Update(ctx context.Context, agentURI identity.URI, patch RecordPatch) error {
	existing, err := r.store.Get(ctx, agentURI)
	if err != nil {
		return err
	}
	// Endpoint changes are location claims — require attestation when verifier is set.
	if r.verifier != nil && patch.Endpoints != nil {
		if patch.Attestation == "" {
			return ErrAttestationFail
		}
	}
	// Verify any presented attestation token regardless of what else changed.
	if r.verifier != nil && patch.Attestation != "" {
		if _, err := r.verifier.Verify(ctx, patch.Attestation, existing.TrustRoot); err != nil {
			return ErrAttestationFail
		}
	}
	endpointsChanged := patch.Endpoints != nil
	if endpointsChanged {
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
	if err := r.store.Put(ctx, existing); err != nil {
		return err
	}
	if r.dht != nil && endpointsChanged {
		if err := r.dht.Put(ctx, existing.AgentURI, existing.Endpoints, dhtTTL); err != nil {
			slog.Warn("dht: failed to refresh agent after endpoint update",
				"uri", existing.AgentURI.String(), "err", err)
		}
	}
	return nil
}

func (r *MemRegistry) Deregister(ctx context.Context, agentURI identity.URI) error {
	if err := r.store.Delete(ctx, agentURI); err != nil {
		return err
	}
	if r.dht != nil {
		if err := r.dht.Delete(ctx, agentURI); err != nil {
			slog.Warn("dht: failed to remove agent after deregistration",
				"uri", agentURI.String(), "err", err)
		}
	}
	return nil
}

func (r *MemRegistry) Get(ctx context.Context, agentURI identity.URI) (Record, error) {
	return r.store.Get(ctx, agentURI)
}

func (r *MemRegistry) Discover(ctx context.Context, q Query) ([]Record, error) {
	candidates, err := r.discoverCandidates(ctx, q.CapabilityPrefix)
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

// discoverCandidates returns the Level-1 candidate set for the given prefix.
// With a DHT it calls FindCapability for O(log N) network-wide prefix lookup,
// then resolves each URI from the authoritative store (Level-2 resolution).
// Without a DHT, or when the DHT call fails, it falls back to a full store scan.
func (r *MemRegistry) discoverCandidates(ctx context.Context, capabilityPrefix string) ([]Record, error) {
	if r.dht == nil {
		return r.store.List(ctx, capabilityPrefix)
	}
	uris, err := r.dht.FindCapability(ctx, capabilityPrefix)
	if err != nil {
		slog.Warn("dht: FindCapability failed, falling back to store scan",
			"prefix", capabilityPrefix, "err", err)
		return r.store.List(ctx, capabilityPrefix)
	}
	records := make([]Record, 0, len(uris))
	for _, uri := range uris {
		rec, storeErr := r.store.Get(ctx, uri)
		if storeErr != nil {
			continue // DHT entry ahead of store (transient race); skip silently
		}
		records = append(records, rec)
	}
	return records, nil
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
