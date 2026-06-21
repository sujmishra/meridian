# Getting Started

## Requirements

- Go 1.22 or later

## Build

```bash
git clone https://github.com/sujmishra/meridian
cd meridian
make build
```

## Run the server

```bash
make run
# or
./bin/meridian --addr=:8080
```

## Run tests

```bash
make test
```

## Examples

### Register an agent

```bash
go run examples/register-agent/main.go
```

### Discover agents by capability

```bash
go run examples/discover-agent/main.go
```

### Route a call through the protocol gateway

```bash
go run examples/protocol-bridge/main.go
```

## Package structure

| Package | Layer | Description |
|---------|-------|-------------|
| `packages/identity` | 0 | `agent://` URI, TypeID, PASETO |
| `packages/registry` | 1 | Agent records, capability discovery |
| `packages/dht` | 1/0 | Decentralized resolution |
| `packages/gateway` | 2 | Protocol bridge (MCP/A2A/ACP/REST) |
| `packages/hai` | 3 | SSE streaming, lifecycle control |
| `packages/mek` | 4 | Memory → Knowledge distillation |

## Import paths

```go
import "github.com/sujmishra/meridian/packages/identity"
import "github.com/sujmishra/meridian/packages/registry"
import "github.com/sujmishra/meridian/packages/gateway"
```
