// Command meridian is the Unified Agent Registry server.
//
// Usage:
//
//	meridian [--addr=:8080] [--trust-root=localhost]
//
// The server exposes:
//   - Registry API:  POST /v1/agents, GET /v1/agents?uri=..., PATCH /v1/agents?uri=..., DELETE /v1/agents?uri=...
//   - Discovery API: GET /v1/discover?capability=/workflow/approval&protocol=a2a
//   - Trust root:    GET /.well-known/agent-trust
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sujmishra/meridian/packages/identity"
	"github.com/sujmishra/meridian/packages/registry"
	"github.com/sujmishra/meridian/packages/server"
)

func main() {
	addr := flag.String("addr", ":8080", "address to listen on")
	trustRoot := flag.String("trust-root", "localhost", "DNS hostname of this registry's trust root")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	signer, err := identity.GeneratePASETOSigner(*trustRoot)
	if err != nil {
		slog.Error("failed to generate Ed25519 key pair", "err", err)
		os.Exit(1)
	}

	verifier := identity.NewPASETOVerifier(nil)
	verifier.AddKey(*trustRoot, signer.PublicKey())

	store := registry.NewMemStore()
	reg := registry.NewMemRegistry(store, verifier)

	srv := server.New(reg, *trustRoot, signer)

	httpServer := &http.Server{
		Addr:         *addr,
		Handler:      srv,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
		BaseContext: func(l net.Listener) context.Context {
			return context.Background()
		},
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("meridian starting", "addr", *addr, "trust_root", *trustRoot)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("meridian shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
}
