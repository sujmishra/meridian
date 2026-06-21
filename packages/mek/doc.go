// Package mek implements Layer 4 of the Unified Agent Registry stack:
// Memory → Extraction → Knowledge (MEK).
//
// MEK enables agents to share learned experiences by distilling raw memories
// into reusable KnowledgeItems. Without MEK, each agent learns from scratch;
// with MEK, the ecosystem accumulates collective intelligence.
//
// The cognitive chain:
//
//	Memory (raw)  →  Extraction  →  KnowledgeItem (shareable)
//
// In the UAR, KnowledgeItems are first-class registry citizens: each receives
// a stable agent:// URI under the "knowledge/" capability namespace so any
// agent can discover and consume it via the standard registry query interface.
//
//	agent://acme.com/knowledge/invoice-approval-patterns/ki_01j4mn...
//
// Versioning: KnowledgeItems use content-addressed (hash-based) URIs to avoid
// versioning ambiguity — the same content always maps to the same URI.
package mek
