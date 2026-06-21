// Example: routing an MCP tool call to an A2A agent through the protocol gateway.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/sujmishra/meridian/packages/gateway"
	"github.com/sujmishra/meridian/packages/registry"
)

func main() {
	ctx := context.Background()

	// A LangChain agent speaks MCP; the target invoice-approval agent speaks A2A.
	// The gateway translates transparently — the caller doesn't need to know.
	payload, err := json.Marshal(map[string]any{
		"tool":   "approve_invoice",
		"params": map[string]any{"invoice_id": "INV-2026-0042", "amount": 1500.00},
	})
	if err != nil {
		log.Fatalf("marshal payload: %v", err)
	}

	req := gateway.Request{
		CapabilityPath: "/workflow/approval",
		SourceProtocol: registry.ProtocolMCP,
		// No TargetProtocol — gateway picks the best supported by the target agent.
		Payload: payload,
	}

	_ = ctx
	_ = req

	// In production: resp, err := gw.Route(ctx, req)
	fmt.Printf("Routing MCP tool call → A2A agent at /workflow/approval\n")
	fmt.Printf("Gateway will translate: MCP tool call → A2A task delegation\n")
}
