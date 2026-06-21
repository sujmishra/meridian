package registry

// Query describes a two-level capability discovery request.
//
// Level 1 (coarse): CapabilityPrefix filters by the hierarchical capability path.
// Level 2 (fine):   remaining fields further narrow the candidate set.
//
// Example:
//
//	Query{
//	    CapabilityPrefix: "/workflow/approval",
//	    Protocols:        []Protocol{ProtocolA2A},
//	    Health:           HealthHealthy,
//	    TrustRoots:       []string{"acme.com"},
//	}
type Query struct {
	// CapabilityPrefix is matched as a path prefix against record.CapabilityPath.
	// Required. A prefix of "/" matches all agents.
	CapabilityPrefix string

	// Protocols filters to agents that support ALL listed protocols.
	// Empty means no protocol constraint.
	Protocols []Protocol

	// Health filters to agents with the given health status.
	// Zero value means no health constraint.
	Health HealthStatus

	// TrustRoots restricts results to agents attested by the listed trust roots.
	// Cross-org federation is ALWAYS explicit — no implicit transitivity.
	// Empty defaults to the registry's own trust root.
	TrustRoots []string

	// Limit caps the number of results returned. 0 means no limit.
	Limit int
}

// Matches reports whether the record satisfies all constraints in the query.
func (q Query) Matches(r Record) bool {
	if q.CapabilityPrefix != "" && !r.AgentURI.MatchesCapabilityPrefix(q.CapabilityPrefix) {
		return false
	}
	if q.Health != "" && r.Health != q.Health {
		return false
	}
	if len(q.TrustRoots) > 0 {
		found := false
		for _, root := range q.TrustRoots {
			if r.TrustRoot == root {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for _, p := range q.Protocols {
		if !r.SupportsProtocol(p) {
			return false
		}
	}
	return true
}
