// Package adapters provides protocol translation adapters for the UAP gateway.
package adapters

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sujmishra/meridian/packages/registry"
)

// MCPToA2A translates MCP tool calls into A2A task delegations.
type MCPToA2A struct{}

func (MCPToA2A) From() registry.Protocol { return registry.ProtocolMCP }
func (MCPToA2A) To() registry.Protocol   { return registry.ProtocolA2A }

// MCPToolCall is the MCP wire format for a tool invocation.
type MCPToolCall struct {
	Tool   string         `json:"tool"`
	Params map[string]any `json:"params"`
}

// A2ATask is the A2A wire format for a delegated task.
type A2ATask struct {
	ID      string         `json:"id"`
	Message string         `json:"message"`
	Params  map[string]any `json:"params,omitempty"`
}

func (MCPToA2A) Translate(_ context.Context, payload []byte) ([]byte, error) {
	var call MCPToolCall
	if err := json.Unmarshal(payload, &call); err != nil {
		return nil, fmt.Errorf("mcp→a2a: failed to decode MCP tool call: %w", err)
	}
	task := A2ATask{
		Message: fmt.Sprintf("Invoke tool: %s", call.Tool),
		Params:  call.Params,
	}
	return json.Marshal(task)
}

func (MCPToA2A) TranslateResponse(_ context.Context, response []byte) ([]byte, error) {
	// A2A task result → MCP tool result envelope.
	// Passthrough for now; full translation depends on A2A response schema.
	return response, nil
}
