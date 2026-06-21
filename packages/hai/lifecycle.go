package hai

import "context"

// TaskState represents the current execution state of an agent task.
type TaskState string

const (
	TaskStateStarted   TaskState = "started"
	TaskStateRunning   TaskState = "running"
	TaskStatePaused    TaskState = "paused"
	TaskStateCompleted TaskState = "completed"
	TaskStateFailed    TaskState = "failed"
	TaskStateStopped   TaskState = "stopped"
)

// TaskController provides full lifecycle control over a running agent task.
type TaskController interface {
	// Pause suspends execution of the task. The task may be resumed later.
	Pause(ctx context.Context, taskID string) error

	// Resume continues a paused task.
	Resume(ctx context.Context, taskID string) error

	// Stop terminates the task immediately. State is not preserved.
	Stop(ctx context.Context, taskID string) error

	// State returns the current lifecycle state of the task.
	State(ctx context.Context, taskID string) (TaskState, error)
}

// TaskSnapshot is a full point-in-time state snapshot of a running task.
type TaskSnapshot struct {
	TaskID     string    `json:"task_id"`
	State      TaskState `json:"state"`
	OutputSoFar string   `json:"output_so_far"`
}
