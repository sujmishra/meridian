// Example: discovering agents by capability using the two-level discovery model.
package main

import (
	"fmt"

	"github.com/sujmishra/meridian/packages/registry"
)

func main() {
	// Level 1: coarse prefix match via DHT.
	// Level 2: fine attribute filter on the candidate set.
	q := registry.Query{
		CapabilityPrefix: "/workflow/approval",
		Protocols:        []registry.Protocol{registry.ProtocolA2A},
		Health:           registry.HealthHealthy,
		TrustRoots:       []string{"acme.com"},
	}

	fmt.Printf("Discovery query:\n")
	fmt.Printf("  Capability prefix : %s\n", q.CapabilityPrefix)
	fmt.Printf("  Required protocol : %v\n", q.Protocols)
	fmt.Printf("  Health filter     : %s\n", q.Health)
	fmt.Printf("  Trust roots       : %v\n", q.TrustRoots)

	// In production, pass q to registry.Registry.Discover(ctx, q)
	// The registry returns a []registry.Record; pick one via the gateway router.
}
