package mek

import (
	"time"

	"github.com/sujmishra/meridian/packages/identity"
)

// Memory is a raw, unstructured experience record stored by an agent.
// Memories are the input to the Extraction stage and are not shared directly.
type Memory struct {
	// ID uniquely identifies this memory within the agent's local store.
	ID identity.TypeID

	// AgentURI is the agent that created this memory.
	AgentURI identity.URI

	// Content is the raw, unstructured experience (e.g. task transcript, tool output).
	Content []byte

	// Tags are optional labels for coarse categorization before extraction.
	Tags []string

	// CreatedAt is when the memory was recorded.
	CreatedAt time.Time
}

// MemoryStore is the local per-agent storage for raw memories.
type MemoryStore interface {
	// Save persists a new memory.
	Save(m Memory) error

	// List returns memories for the given agent, optionally filtered by tags.
	List(agentURI identity.URI, tags []string) ([]Memory, error)

	// Delete removes a memory after it has been successfully extracted.
	Delete(id identity.TypeID) error
}
