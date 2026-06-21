# Architecture

Meridian implements the Unified Agent Registry (UAR) — a five-layer infrastructure stack
enabling AI agents to register, discover, and interoperate across heterogeneous frameworks,
cloud providers, and organizations.

## The Five Layers

```
Layer 4 ── MEK (Cognitive)       Memory → Extraction → Knowledge sharing
Layer 3 ── HAI (Interaction)     SSE streaming, lifecycle control
Layer 2 ── UAP Gateway           Protocol bridge: MCP ↔ A2A ↔ ACP ↔ REST
Layer 1 ── UAP Registry Center   Capability index, health, governance
Layer 0 ── Identity              agent:// URI, DHT resolution, PASETO attestation
```

## Layer 0: Identity (`packages/identity`)

Every agent is assigned a stable `agent://` URI at registration time:

```
agent://trust-root/capability-path/agent-id
agent://acme.com/workflow/approval/agent_01h455vb4pex5vsknk084sn02q
```

The URI **does not change** when the agent migrates, scales, or changes protocol.
Only the DHT mapping from URI → network endpoint is updated.

- **TypeID**: type-prefixed UUIDv7 (`agent_<base32>`) — globally unique, lexicographically sortable
- **Trust root**: DNS hostname publishing a PASETO signing key at `/.well-known/agent-trust`
- **PASETO v4**: signs capability claims; verifiers cache the public key — no central authority at query time

## Layer 1: Registry Center (`packages/registry`)

Authoritative write store for agent records. Each record maps a stable `agent://` URI to:
- Supported protocols and current endpoints
- PASETO attestation from the trust root
- Health status and governance metadata

Write operations require a valid attestation. Reads can be served by the registry or any DHT node.

## Layer 2: Protocol Gateway (`packages/gateway`)

Translates between protocols so callers need not know the target's native protocol.
Routes by **capability path**, not by address — enabling transparent load balancing and failover.

| Conversion | Input | Output |
|------------|-------|--------|
| MCP → A2A | tool call | A2A task delegation |
| A2A → MCP | task message | MCP tool invocation |
| REST → A2A | HTTP POST | A2A task with streaming result |
| ACP → MCP | multi-part message | MCP resource + tool combo |

## Layer 3: HAI (`packages/hai`)

Standardizes human-agent interaction:
- Token-by-token streaming via SSE
- Full lifecycle control: start / pause / resume / stop
- Registry-aware events: `agent_discovery`, `agent_migration`, `attestation`

Trust root display names are always abstracted — raw DHT routing paths are never exposed.

## Layer 4: MEK (`packages/mek`)

Enables collective intelligence across agents:

```
Memory (raw) → Extractor → KnowledgeItem (shareable)
```

KnowledgeItems are registered in the UAR under `knowledge/` namespace with content-addressed URIs:

```
agent://acme.com/knowledge/invoice-approval-patterns/ki_01j4mn...
```

## Discovery Model

**Two-level discovery** anchored to stable `agent://` URIs:

1. **Coarse** — DHT prefix-match on capability path → candidate set
2. **Fine** — filter by protocol, health, trust root

```go
registry.Query{
    CapabilityPrefix: "/workflow/approval",
    Protocols:        []Protocol{ProtocolA2A},
    Health:           HealthHealthy,
    TrustRoots:       []string{"acme.com"},
}
```

## Trust Model

- Each organization is its own trust root; no implicit transitivity between orgs
- PASETO-signed capability claims; verifiers cache the public key
- DHT for decentralized reads; Registry Center for authoritative writes
- All cross-org queries are explicit (`TrustRoots` parameter required)

## References

- Co-TAP: [arXiv:2510.08263](https://arxiv.org/abs/2510.08263)
- Agent Identity URI: [arXiv:2601.14567](https://arxiv.org/abs/2601.14567)
- AGNTCY ADS: [arXiv:2509.18787](https://arxiv.org/abs/2509.18787)
