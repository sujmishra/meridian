package hai

import (
	"time"

	"github.com/sujmishra/meridian/packages/identity"
	"github.com/sujmishra/meridian/packages/registry"
)

// EventType identifies the kind of HAI event.
type EventType string

const (
	// EventToken is emitted for each streamed output token from an agent.
	EventToken EventType = "token"

	// EventAgentDiscovery is emitted when a capability query resolves to an agent URI.
	EventAgentDiscovery EventType = "agent_discovery"

	// EventAgentMigration is emitted when an agent's endpoint changes.
	// UI may display a continuity notice.
	EventAgentMigration EventType = "agent_migration"

	// EventAttestation is emitted when a PASETO capability claim is verified.
	// Visible in the audit trail.
	EventAttestation EventType = "attestation"

	// EventLifecycle is emitted on task state transitions (started, paused, completed, failed).
	EventLifecycle EventType = "lifecycle"

	// EventError signals a non-fatal error during task execution.
	EventError EventType = "error"
)

// Event is a single HAI event emitted over the SSE stream.
type Event struct {
	Type      EventType `json:"type"`
	TaskID    string    `json:"task_id"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data"`
}

// TokenData carries a streamed output token.
type TokenData struct {
	Token string `json:"token"`
	Index int    `json:"index"`
}

// DiscoveryData carries the result of a capability resolution.
type DiscoveryData struct {
	CapabilityQuery string       `json:"capability_query"`
	ResolvedURI     identity.URI `json:"resolved_uri"`
	// DisplayName abstracts the trust root — raw DHT paths are never exposed.
	DisplayName string `json:"display_name"`
}

// MigrationData carries endpoint change information.
type MigrationData struct {
	AgentURI    identity.URI               `json:"agent_uri"`
	OldEndpoint map[registry.Protocol]string `json:"old_endpoint"`
	NewEndpoint map[registry.Protocol]string `json:"new_endpoint"`
}

// AttestationData carries capability verification results for the audit trail.
type AttestationData struct {
	AgentURI       identity.URI `json:"agent_uri"`
	CapabilityPath string       `json:"capability_path"`
	Issuer         string       `json:"issuer"`
	Valid           bool         `json:"valid"`
}

// LifecycleData carries a task state transition.
type LifecycleData struct {
	State     TaskState `json:"state"`
	PrevState TaskState `json:"prev_state"`
}
