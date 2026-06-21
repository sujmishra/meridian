// Command meridian is the Unified Agent Registry server.
//
// Usage:
//
//	meridian [--config=path/to/config.yaml] [--addr=:8080]
//
// The server exposes:
//   - Registry API:  POST /agents, GET /agents/{uri}, DELETE /agents/{uri}
//   - Discovery API: GET /discover?capability=/workflow/approval&protocol=a2a
//   - Gateway API:   POST /gateway/route
//   - HAI stream:    GET /tasks/{taskID}/stream  (SSE)
//   - Trust root:    GET /.well-known/agent-trust
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	addr := flag.String("addr", ":8080", "address to listen on")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("meridian starting", "addr", *addr)

	// TODO: wire up registry, gateway, HAI, and MEK components and start HTTP server.
	// See packages/registry, packages/gateway, packages/hai, packages/mek.

	<-ctx.Done()
	slog.Info("meridian shutting down")
}
