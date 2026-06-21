// Example: registering an agent with the Unified Agent Registry.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/sujmishra/meridian/packages/identity"
	"github.com/sujmishra/meridian/packages/registry"
)

func main() {
	ctx := context.Background()

	// Generate a stable TypeID for the new agent.
	agentID, err := identity.NewTypeID()
	if err != nil {
		log.Fatalf("generate agent ID: %v", err)
	}

	// Construct the agent:// URI.
	rawURI := fmt.Sprintf("agent://acme.com/workflow/approval/%s", agentID)
	agentURI, err := identity.Parse(rawURI)
	if err != nil {
		log.Fatalf("parse URI: %v", err)
	}

	// Build the registry record.
	record := registry.Record{
		AgentURI:       agentURI,
		TrustRoot:      "acme.com",
		CapabilityPath: "/workflow/approval",
		Protocols:      []registry.Protocol{registry.ProtocolA2A, registry.ProtocolREST},
		Endpoints: map[registry.Protocol]string{
			registry.ProtocolA2A:  "https://agents.acme.com/a2a/approver",
			registry.ProtocolREST: "https://agents.acme.com/api/approver",
		},
		// Attestation would be obtained from the trust root's Signer in production.
		Attestation:   "<PASETO token signed by acme.com>",
		SchemaVersion: "1.0",
		Health:        registry.HealthHealthy,
		RegisteredAt:  time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}

	_ = ctx
	_ = record

	fmt.Printf("Agent registered: %s\n", agentURI)
	fmt.Printf("Well-known trust URL: %s\n", agentURI.WellKnownTrustURL())
}
