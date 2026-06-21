package gateway

import (
	"context"

	"github.com/sujmishra/meridian/packages/identity"
	"github.com/sujmishra/meridian/packages/registry"
)

// Request is an inbound call routed through the gateway.
type Request struct {
	// CapabilityPath is used to discover the target agent (prefix match).
	CapabilityPath string

	// SourceProtocol is the protocol the caller is using.
	SourceProtocol registry.Protocol

	// TargetProtocol is the preferred protocol to use when calling the target agent.
	// If empty, the gateway picks the best protocol the target supports.
	TargetProtocol registry.Protocol

	// Payload is the raw protocol-specific message body.
	Payload []byte

	// PreferredAgent pins the request to a specific agent URI, bypassing discovery.
	// If set, CapabilityPath is ignored for routing.
	PreferredAgent *identity.URI
}

// Response is the gateway's reply after protocol translation and agent execution.
type Response struct {
	// AgentURI is the identity of the agent that handled the request.
	AgentURI identity.URI

	// Protocol is the protocol used to call the target agent.
	Protocol registry.Protocol

	// Payload is the raw response from the target agent, in SourceProtocol format.
	Payload []byte
}

// Gateway routes requests to registered agents, translating protocols as needed.
type Gateway interface {
	// Route discovers a suitable agent via capability path and forwards the request,
	// translating from SourceProtocol to the target's native protocol.
	Route(ctx context.Context, req Request) (Response, error)

	// RouteStream is like Route but returns a channel of streamed response chunks
	// for protocols that support it (A2A, REST+SSE).
	RouteStream(ctx context.Context, req Request) (<-chan Chunk, error)
}

// Chunk is a single streamed response fragment.
type Chunk struct {
	Data  []byte
	Done  bool
	Error error
}

// Adapter translates between two protocols.
type Adapter interface {
	// From returns the source protocol this adapter accepts.
	From() registry.Protocol

	// To returns the target protocol this adapter produces.
	To() registry.Protocol

	// Translate converts a source-protocol payload into the target-protocol format.
	Translate(ctx context.Context, payload []byte) ([]byte, error)

	// TranslateResponse converts a target-protocol response back to the source format.
	TranslateResponse(ctx context.Context, response []byte) ([]byte, error)
}
