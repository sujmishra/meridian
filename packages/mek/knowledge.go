package mek

import (
	"context"
	"time"

	"github.com/sujmishra/meridian/packages/identity"
)

// KnowledgeItem is a distilled, reusable insight extracted from agent memories.
// It is a first-class citizen in the UAR: registered with a stable agent:// URI
// under the "knowledge/" capability namespace so any agent can discover and consume it.
//
// KnowledgeItems are content-addressed: the URI suffix is derived from the content
// hash to avoid versioning ambiguity — identical content always resolves to the same URI.
type KnowledgeItem struct {
	// URI is the stable, content-addressed agent:// URI for this knowledge item.
	// e.g. agent://acme.com/knowledge/invoice-approval-patterns/ki_01j4mn...
	URI identity.URI

	// Title is a short human-readable name for this knowledge item.
	Title string

	// Content is the distilled, reusable insight in a structured format.
	Content []byte

	// ContentType is the MIME type of Content (e.g. "application/json", "text/plain").
	ContentType string

	// SourceAgents lists the agent URIs whose memories contributed to this item.
	SourceAgents []identity.URI

	// CreatedAt is when this knowledge item was first registered.
	CreatedAt time.Time
}

// KnowledgeStore manages registration and retrieval of KnowledgeItems in the UAR.
type KnowledgeStore interface {
	// Register adds a KnowledgeItem to the registry under the knowledge/ namespace.
	// If an item with the same content hash already exists, it returns the existing URI.
	Register(ctx context.Context, item KnowledgeItem) (identity.URI, error)

	// Get returns a KnowledgeItem by its URI.
	Get(ctx context.Context, uri identity.URI) (KnowledgeItem, error)

	// Query finds KnowledgeItems whose capability path matches the given prefix.
	// e.g. Query("/knowledge/invoice-approval-patterns") returns all items under that namespace.
	Query(ctx context.Context, capabilityPrefix string) ([]KnowledgeItem, error)
}
