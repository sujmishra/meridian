// Package registry implements Layer 1 of the Unified Agent Registry stack:
// the UAP Registry Center.
//
// The Registry Center is the authoritative write store for agent records.
// Each record maps a stable agent:// URI to the agent's current capabilities,
// supported protocols, active endpoints, health status, and PASETO attestation.
//
// Key design decisions:
//   - Writes require a valid PASETO attestation signed by the agent's trust root.
//   - Reads are served from the Registry Center OR from any DHT node (see package dht).
//   - Discovery uses a two-level model: coarse prefix-match → fine attribute filter.
//
// Usage:
//
//	r := registry.New(store, verifier)
//	if err := r.Register(ctx, record); err != nil { ... }
//	results, err := r.Discover(ctx, registry.Query{CapabilityPrefix: "/workflow/approval"})
package registry
