package identity

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const scheme = "agent"

// URI is a parsed, validated agent:// URI.
// The URI decouples agent identity from its network location — the same URI
// remains valid across cloud migrations, protocol changes, and restarts.
type URI struct {
	// TrustRoot is the DNS hostname of the organization vouching for the agent.
	// It must be a valid hostname (e.g. "acme.com").
	TrustRoot string

	// CapabilityPath is the hierarchical description of what the agent does.
	// It supports prefix-match discovery (e.g. "/workflow/approval").
	CapabilityPath string

	// AgentID is the TypeID uniquely identifying this agent instance.
	AgentID TypeID

	// RawQuery and Fragment are optional per RFC 3986.
	RawQuery string
	Fragment string
}

var (
	ErrInvalidScheme  = errors.New("identity: URI scheme must be 'agent'")
	ErrEmptyTrustRoot = errors.New("identity: trust root must not be empty")
	ErrEmptyAgentID   = errors.New("identity: agent ID must not be empty")
	ErrInvalidPath    = errors.New("identity: capability path must have at least one segment followed by agent ID")
)

// Parse parses a raw agent:// URI string into a URI.
// Returns an error if the URI is malformed or missing required components.
func Parse(raw string) (URI, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return URI{}, fmt.Errorf("identity: failed to parse URI: %w", err)
	}

	if u.Scheme != scheme {
		return URI{}, ErrInvalidScheme
	}
	if u.Host == "" {
		return URI{}, ErrEmptyTrustRoot
	}

	// Path format: /capability/.../segments/agent-id
	// The last path segment is the agent ID; everything before it is the capability path.
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return URI{}, ErrInvalidPath
	}

	rawID := parts[len(parts)-1]
	if rawID == "" {
		return URI{}, ErrEmptyAgentID
	}

	agentID, err := ParseTypeID(rawID)
	if err != nil {
		return URI{}, fmt.Errorf("identity: invalid agent ID: %w", err)
	}

	capPath := "/" + strings.Join(parts[:len(parts)-1], "/")

	return URI{
		TrustRoot:      u.Host,
		CapabilityPath: capPath,
		AgentID:        agentID,
		RawQuery:       u.RawQuery,
		Fragment:       u.Fragment,
	}, nil
}

// String returns the canonical string representation of the URI.
func (u URI) String() string {
	s := fmt.Sprintf("agent://%s%s/%s", u.TrustRoot, u.CapabilityPath, u.AgentID)
	if u.RawQuery != "" {
		s += "?" + u.RawQuery
	}
	if u.Fragment != "" {
		s += "#" + u.Fragment
	}
	return s
}

// WellKnownTrustURL returns the HTTPS URL at which the trust root publishes
// its PASETO signing key.
//
//	e.g. https://acme.com/.well-known/agent-trust
func (u URI) WellKnownTrustURL() string {
	return fmt.Sprintf("https://%s/.well-known/agent-trust", u.TrustRoot)
}

// MatchesCapabilityPrefix reports whether this URI's capability path starts
// with the given prefix (e.g. "/workflow" matches "/workflow/approval").
func (u URI) MatchesCapabilityPrefix(prefix string) bool {
	return strings.HasPrefix(u.CapabilityPath, strings.TrimRight(prefix, "/"))
}

// MarshalJSON serializes the URI as its canonical string form.
func (u URI) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

// UnmarshalJSON parses the URI from its canonical string form.
func (u *URI) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := Parse(s)
	if err != nil {
		return err
	}
	*u = parsed
	return nil
}
