package adapters

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sujmishra/meridian/packages/registry"
)

// ACPToMCP translates ACP multi-part messages into MCP resource + tool combos.
type ACPToMCP struct{}

func (ACPToMCP) From() registry.Protocol { return registry.ProtocolACP }
func (ACPToMCP) To() registry.Protocol   { return registry.ProtocolMCP }

// ACPMessage is a simplified ACP multi-part message envelope.
type ACPMessage struct {
	Parts []ACPPart `json:"parts"`
}

// ACPPart is a single part within an ACP message.
type ACPPart struct {
	ContentType string `json:"content_type"`
	Content     any    `json:"content"`
}

func (ACPToMCP) Translate(_ context.Context, payload []byte) ([]byte, error) {
	var msg ACPMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, fmt.Errorf("acp→mcp: failed to decode ACP message: %w", err)
	}
	// Merge all parts into a single MCP tool call params map.
	params := make(map[string]any, len(msg.Parts))
	for i, part := range msg.Parts {
		params[fmt.Sprintf("part_%d", i)] = part.Content
	}
	call := MCPToolCall{
		Tool:   "acp_message",
		Params: params,
	}
	return json.Marshal(call)
}

func (ACPToMCP) TranslateResponse(_ context.Context, response []byte) ([]byte, error) {
	return response, nil
}
