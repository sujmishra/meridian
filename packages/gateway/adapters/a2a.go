package adapters

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sujmishra/meridian/packages/registry"
)

// A2AToMCP translates A2A task messages into MCP tool invocations.
type A2AToMCP struct{}

func (A2AToMCP) From() registry.Protocol { return registry.ProtocolA2A }
func (A2AToMCP) To() registry.Protocol   { return registry.ProtocolMCP }

func (A2AToMCP) Translate(_ context.Context, payload []byte) ([]byte, error) {
	var task A2ATask
	if err := json.Unmarshal(payload, &task); err != nil {
		return nil, fmt.Errorf("a2a→mcp: failed to decode A2A task: %w", err)
	}
	call := MCPToolCall{
		Tool:   task.Message,
		Params: task.Params,
	}
	return json.Marshal(call)
}

func (A2AToMCP) TranslateResponse(_ context.Context, response []byte) ([]byte, error) {
	return response, nil
}
