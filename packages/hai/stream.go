package hai

import (
	"context"
	"net/http"
)

// Stream is the SSE event stream interface for a single agent task.
type Stream interface {
	// Events returns a channel of events for this task.
	// The channel is closed when the task completes or the context is cancelled.
	Events() <-chan Event

	// Snapshot returns the current full state of the task, useful for reconnecting clients.
	Snapshot() TaskSnapshot
}

// StreamHandler is an HTTP handler that serves an SSE stream for agent tasks.
// Mount at: GET /tasks/{taskID}/stream
type StreamHandler interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// Streamer manages active SSE streams for agent tasks.
type Streamer interface {
	// NewStream creates an SSE stream for the given task ID.
	NewStream(ctx context.Context, taskID string) (Stream, error)

	// Publish emits an event to all subscribers of a task stream.
	Publish(taskID string, event Event) error

	// Close terminates all streams for the given task and releases resources.
	Close(taskID string) error
}
