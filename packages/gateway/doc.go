// Package gateway implements Layer 2 of the Unified Agent Registry stack:
// the UAP Protocol Gateway.
//
// The gateway translates between heterogeneous agent communication protocols
// (MCP, A2A, ACP, REST) so that callers need not know the target agent's
// native protocol. Routing is by capability path, not by network address —
// a caller requesting "/workflow/approval" is routed to any healthy registered
// agent matching that prefix, enabling transparent load balancing and failover.
//
// Supported conversions:
//
//	MCP  → A2A   tool call           → A2A task delegation
//	A2A  → MCP   task message        → MCP tool invocation
//	REST → A2A   HTTP POST           → A2A task with streaming result
//	ACP  → MCP   multi-part message  → MCP resource + tool combo
//
// Usage:
//
//	gw := gateway.New(registry, adapters...)
//	result, err := gw.Route(ctx, gateway.Request{
//	    CapabilityPath: "/workflow/approval",
//	    Protocol:       registry.ProtocolA2A,
//	    Payload:        taskJSON,
//	})
package gateway
