package registry

import (
	"time"

	"github.com/sujmishra/meridian/packages/identity"
)

// Protocol identifies an agent communication protocol.
type Protocol string

const (
	ProtocolA2A  Protocol = "a2a"
	ProtocolMCP  Protocol = "mcp"
	ProtocolACP  Protocol = "acp"
	ProtocolREST Protocol = "rest"
)

// HealthStatus reports the last known liveness of a registered agent.
type HealthStatus string

const (
	HealthHealthy  HealthStatus = "healthy"
	HealthDegraded HealthStatus = "degraded"
	HealthUnknown  HealthStatus = "unknown"
)

// Record is the authoritative registry entry for a single agent.
// It is serialized to JSON for storage and transport.
type Record struct {
	// AgentURI is the stable, topology-independent identity of this agent.
	AgentURI identity.URI `json:"agent_uri"`

	// TrustRoot is the DNS hostname of the organization that attested this agent.
	TrustRoot string `json:"trust_root"`

	// CapabilityPath mirrors AgentURI.CapabilityPath for indexed queries.
	CapabilityPath string `json:"capability_path"`

	// Protocols lists the communication protocols this agent speaks.
	Protocols []Protocol `json:"protocols"`

	// Endpoints maps each supported protocol to its current network address.
	// These change on migration; the AgentURI does not.
	Endpoints map[Protocol]string `json:"endpoints"`

	// Attestation is the raw PASETO token binding this record to the trust root.
	Attestation string `json:"attestation"`

	// SchemaVersion is the UAR record schema version (currently "1.0").
	SchemaVersion string `json:"schema_version"`

	// Health is the most recently observed health of this agent.
	Health HealthStatus `json:"health"`

	// RegisteredAt is when this record was first written.
	RegisteredAt time.Time `json:"registered_at"`

	// UpdatedAt is when this record was last modified.
	UpdatedAt time.Time `json:"updated_at"`
}

// SupportsProtocol reports whether the agent supports the given protocol.
func (r Record) SupportsProtocol(p Protocol) bool {
	for _, proto := range r.Protocols {
		if proto == p {
			return true
		}
	}
	return false
}

// EndpointFor returns the network address for the given protocol, or "" if unsupported.
func (r Record) EndpointFor(p Protocol) string {
	return r.Endpoints[p]
}
