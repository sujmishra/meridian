// Package dht implements decentralized read resolution for the Unified Agent Registry.
//
// While the registry package is the authoritative write store, the DHT allows any
// participating node to answer capability lookups in O(log N) hops — eliminating
// the single point of failure inherent in a centralized registry.
//
// Each organization maintains its own DHT partition scoped to its trust root.
// Cross-org queries require explicit federation; there is no implicit routing
// between partitions.
//
// The DHT stores (agent_uri → current_endpoints) mappings. When an agent migrates,
// the registry updates the DHT entry; all workflow references using the stable
// agent:// URI continue to resolve correctly.
package dht
