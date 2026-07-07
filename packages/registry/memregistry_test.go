package registry_test

import (
	"context"
	"testing"

	"github.com/sujmishra/meridian/packages/identity"
	"github.com/sujmishra/meridian/packages/registry"
)

const (
	testURI1 = "agent://acme.com/workflow/approval/agent_01h455vb4pex5vsknk084sn02q"
	testURI2 = "agent://acme.com/workflow/summary/agent_01h455vb4pex5vsknk084sn02r"
	testURI3 = "agent://other.org/data/ingest/agent_01h455vb4pex5vsknk084sn02s"
)

func newReg(t *testing.T) registry.Registry {
	t.Helper()
	return registry.NewMemRegistry(registry.NewMemStore(), nil)
}

func mustParseURI(t *testing.T, raw string) identity.URI {
	t.Helper()
	u, err := identity.Parse(raw)
	if err != nil {
		t.Fatalf("identity.Parse(%q): %v", raw, err)
	}
	return u
}

func makeRecord(t *testing.T, raw string) registry.Record {
	t.Helper()
	uri := mustParseURI(t, raw)
	return registry.Record{
		AgentURI:       uri,
		TrustRoot:      uri.TrustRoot,
		CapabilityPath: uri.CapabilityPath,
		Protocols:      []registry.Protocol{registry.ProtocolA2A},
		Endpoints:      map[registry.Protocol]string{registry.ProtocolA2A: "https://agent.acme.com/a2a"},
	}
}

func TestRegister_Success(t *testing.T) {
	reg := newReg(t)
	ctx := context.Background()

	if err := reg.Register(ctx, makeRecord(t, testURI1)); err != nil {
		t.Fatalf("Register: %v", err)
	}
	r, err := reg.Get(ctx, mustParseURI(t, testURI1))
	if err != nil {
		t.Fatalf("Get after Register: %v", err)
	}
	if r.AgentURI.String() != testURI1 {
		t.Errorf("AgentURI = %q, want %q", r.AgentURI, testURI1)
	}
	if r.SchemaVersion != "1.0" {
		t.Errorf("SchemaVersion = %q, want 1.0", r.SchemaVersion)
	}
	if r.Health != registry.HealthUnknown {
		t.Errorf("Health = %q, want %q", r.Health, registry.HealthUnknown)
	}
	if r.RegisteredAt.IsZero() {
		t.Error("RegisteredAt should be set")
	}
}

func TestRegister_Duplicate(t *testing.T) {
	reg := newReg(t)
	ctx := context.Background()
	rec := makeRecord(t, testURI1)

	if err := reg.Register(ctx, rec); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	if err := reg.Register(ctx, rec); err != registry.ErrAlreadyExists {
		t.Errorf("second Register: got %v, want ErrAlreadyExists", err)
	}
}

func TestRegister_InvalidRecord(t *testing.T) {
	reg := newReg(t)
	ctx := context.Background()

	// Record with no protocols — should be rejected.
	uri := mustParseURI(t, testURI1)
	bad := registry.Record{
		AgentURI:       uri,
		TrustRoot:      uri.TrustRoot,
		CapabilityPath: uri.CapabilityPath,
		// Protocols is empty
	}
	if err := reg.Register(ctx, bad); err != registry.ErrInvalidRecord {
		t.Errorf("Register invalid: got %v, want ErrInvalidRecord", err)
	}
}

func TestGet_NotFound(t *testing.T) {
	reg := newReg(t)
	_, err := reg.Get(context.Background(), mustParseURI(t, testURI1))
	if err != registry.ErrNotFound {
		t.Errorf("Get unknown: got %v, want ErrNotFound", err)
	}
}

func TestUpdate_Endpoints(t *testing.T) {
	reg := newReg(t)
	ctx := context.Background()

	if err := reg.Register(ctx, makeRecord(t, testURI1)); err != nil {
		t.Fatalf("Register: %v", err)
	}
	uri := mustParseURI(t, testURI1)
	newEndpoints := map[registry.Protocol]string{
		registry.ProtocolA2A: "https://new.acme.com/a2a",
		registry.ProtocolMCP: "https://new.acme.com/mcp",
	}
	if err := reg.Update(ctx, uri, registry.RecordPatch{Endpoints: newEndpoints}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	updated, _ := reg.Get(ctx, uri)
	if updated.EndpointFor(registry.ProtocolA2A) != "https://new.acme.com/a2a" {
		t.Errorf("A2A endpoint = %q after update", updated.EndpointFor(registry.ProtocolA2A))
	}
	if !updated.SupportsProtocol(registry.ProtocolMCP) {
		t.Error("expected MCP to be added to protocols after update")
	}
}

func TestUpdate_Health(t *testing.T) {
	reg := newReg(t)
	ctx := context.Background()

	if err := reg.Register(ctx, makeRecord(t, testURI1)); err != nil {
		t.Fatalf("Register: %v", err)
	}
	uri := mustParseURI(t, testURI1)
	if err := reg.Update(ctx, uri, registry.RecordPatch{Health: registry.HealthHealthy}); err != nil {
		t.Fatalf("Update health: %v", err)
	}
	r, _ := reg.Get(ctx, uri)
	if r.Health != registry.HealthHealthy {
		t.Errorf("Health = %q, want healthy", r.Health)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	reg := newReg(t)
	err := reg.Update(context.Background(), mustParseURI(t, testURI1), registry.RecordPatch{Health: registry.HealthHealthy})
	if err != registry.ErrNotFound {
		t.Errorf("Update unknown: got %v, want ErrNotFound", err)
	}
}

func TestDeregister(t *testing.T) {
	reg := newReg(t)
	ctx := context.Background()

	if err := reg.Register(ctx, makeRecord(t, testURI1)); err != nil {
		t.Fatalf("Register: %v", err)
	}
	uri := mustParseURI(t, testURI1)
	if err := reg.Deregister(ctx, uri); err != nil {
		t.Fatalf("Deregister: %v", err)
	}
	if _, err := reg.Get(ctx, uri); err != registry.ErrNotFound {
		t.Errorf("Get after Deregister: got %v, want ErrNotFound", err)
	}
}

func TestDeregister_NotFound(t *testing.T) {
	reg := newReg(t)
	err := reg.Deregister(context.Background(), mustParseURI(t, testURI1))
	if err != registry.ErrNotFound {
		t.Errorf("Deregister unknown: got %v, want ErrNotFound", err)
	}
}

func TestDiscover_ByCapabilityPrefix(t *testing.T) {
	reg := newReg(t)
	ctx := context.Background()

	for _, raw := range []string{testURI1, testURI2, testURI3} {
		if err := reg.Register(ctx, makeRecord(t, raw)); err != nil {
			t.Fatalf("Register %s: %v", raw, err)
		}
	}

	results, err := reg.Discover(ctx, registry.Query{CapabilityPrefix: "/workflow"})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Discover /workflow: got %d results, want 2", len(results))
	}
}

func TestDiscover_ByProtocol(t *testing.T) {
	reg := newReg(t)
	ctx := context.Background()

	rec1 := makeRecord(t, testURI1) // has A2A only
	rec2 := makeRecord(t, testURI2)
	rec2.Protocols = []registry.Protocol{registry.ProtocolMCP}
	rec2.Endpoints = map[registry.Protocol]string{registry.ProtocolMCP: "https://agent.acme.com/mcp"}

	if err := reg.Register(ctx, rec1); err != nil {
		t.Fatalf("Register rec1: %v", err)
	}
	if err := reg.Register(ctx, rec2); err != nil {
		t.Fatalf("Register rec2: %v", err)
	}

	results, err := reg.Discover(ctx, registry.Query{
		CapabilityPrefix: "/workflow",
		Protocols:        []registry.Protocol{registry.ProtocolA2A},
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(results) != 1 || results[0].AgentURI.String() != testURI1 {
		t.Errorf("Discover A2A: got %d results, want 1 (testURI1)", len(results))
	}
}

func TestDiscover_ByHealth(t *testing.T) {
	reg := newReg(t)
	ctx := context.Background()

	if err := reg.Register(ctx, makeRecord(t, testURI1)); err != nil {
		t.Fatalf("Register: %v", err)
	}
	uri := mustParseURI(t, testURI1)
	if err := reg.Update(ctx, uri, registry.RecordPatch{Health: registry.HealthHealthy}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if err := reg.Register(ctx, makeRecord(t, testURI2)); err != nil {
		t.Fatalf("Register: %v", err)
	}

	results, err := reg.Discover(ctx, registry.Query{
		CapabilityPrefix: "/workflow",
		Health:           registry.HealthHealthy,
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(results) != 1 || results[0].AgentURI.String() != testURI1 {
		t.Errorf("Discover healthy: got %d results, want 1", len(results))
	}
}

func TestDiscover_ByTrustRoot(t *testing.T) {
	reg := newReg(t)
	ctx := context.Background()

	for _, raw := range []string{testURI1, testURI3} {
		if err := reg.Register(ctx, makeRecord(t, raw)); err != nil {
			t.Fatalf("Register %s: %v", raw, err)
		}
	}

	results, err := reg.Discover(ctx, registry.Query{
		CapabilityPrefix: "/",
		TrustRoots:       []string{"acme.com"},
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(results) != 1 || results[0].TrustRoot != "acme.com" {
		t.Errorf("Discover by trust root: got %d results, want 1 from acme.com", len(results))
	}
}

func TestDiscover_Limit(t *testing.T) {
	reg := newReg(t)
	ctx := context.Background()

	for _, raw := range []string{testURI1, testURI2, testURI3} {
		if err := reg.Register(ctx, makeRecord(t, raw)); err != nil {
			t.Fatalf("Register %s: %v", raw, err)
		}
	}

	results, err := reg.Discover(ctx, registry.Query{
		CapabilityPrefix: "/",
		Limit:            2,
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Discover limit=2: got %d results, want 2", len(results))
	}
}

func TestDiscover_EmptyResult(t *testing.T) {
	reg := newReg(t)
	results, err := reg.Discover(context.Background(), registry.Query{CapabilityPrefix: "/unknown"})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Discover unknown prefix: got %d results, want 0", len(results))
	}
}
