// Package hai implements Layer 3 of the Unified Agent Registry stack:
// Human-Agent Interaction (HAI).
//
// HAI standardizes how users and orchestrators observe and control agent tasks:
//   - Token-by-token streaming via Server-Sent Events (SSE)
//   - Full lifecycle control: start, pause, resume, stop
//   - State synchronization: full snapshot or incremental diff
//   - Registry-aware events: discovery, migration, attestation
//
// HAI events relevant to the registry:
//
//	EventAgentDiscovery   — emitted when a capability query resolves to an agent URI
//	EventAgentMigration   — emitted when an agent's endpoint changes
//	EventAttestation      — emitted when a PASETO claim is verified (audit trail)
//
// Trust root display names are always abstracted in UI events — raw DHT routing
// paths are never exposed to prevent organizational topology leakage.
package hai
