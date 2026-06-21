package mek

import "context"

// Extractor distills one or more raw Memories into a KnowledgeItem.
// The extraction process is implementation-defined — a common approach is
// to use an LLM to summarize and generalize the experience.
type Extractor interface {
	// Extract processes the given memories and returns a distilled KnowledgeItem.
	// The caller is responsible for registering the item with the UAR.
	Extract(ctx context.Context, memories []Memory) (KnowledgeItem, error)
}

// ExtractionPolicy controls which memories are eligible for extraction.
type ExtractionPolicy struct {
	// MinMemories is the minimum number of memories required before extraction runs.
	MinMemories int

	// Tags restricts extraction to memories with at least one of these tags.
	Tags []string

	// MaxAgeSeconds ignores memories older than this many seconds.
	MaxAgeSeconds int64
}
