package adapters

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sujmishra/meridian/packages/registry"
)

// RESTToA2A wraps an HTTP POST body as an A2A task with a streaming result.
type RESTToA2A struct{}

func (RESTToA2A) From() registry.Protocol { return registry.ProtocolREST }
func (RESTToA2A) To() registry.Protocol   { return registry.ProtocolA2A }

func (RESTToA2A) Translate(_ context.Context, payload []byte) ([]byte, error) {
	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		return nil, fmt.Errorf("rest→a2a: failed to decode request body: %w", err)
	}
	task := A2ATask{
		Message: "REST request",
		Params:  body,
	}
	return json.Marshal(task)
}

func (RESTToA2A) TranslateResponse(_ context.Context, response []byte) ([]byte, error) {
	return response, nil
}
