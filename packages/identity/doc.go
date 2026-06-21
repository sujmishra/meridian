// Package identity implements Layer 0 of the Unified Agent Registry stack.
//
// It provides:
//   - agent:// URI parsing, validation, and normalization (arXiv:2601.14567)
//   - TypeID generation (type-prefixed UUIDv7) for globally unique, stable agent IDs
//   - PASETO v4 token issuance and verification for capability attestation
//   - Trust root key publication and caching
//
// An agent:// URI has the form:
//
//	agent://trust-root/capability-path/agent-id
//	agent://acme.com/workflow/approval/agent_01h455vb4pex5vsknk084sn02q
//
// The agent ID is stable across migrations; only the DHT mapping from URI to
// network endpoint changes when an agent moves.
package identity
