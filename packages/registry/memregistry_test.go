package registry_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

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
	return registry.NewMemRegistry(registry.NewMemStore(), nil, nil)
}

// stubDHT records DHT calls for assertion in unit tests.
type stubDHT struct {
	mu         sync.Mutex
	puts       []identity.URI
	deletes    []identity.URI
	findResult []identity.URI
	findErr    error
}

func (d *stubDHT) Put(_ context.Context, uri identity.URI, _ map[registry.Protocol]string, _ time.Duration) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.puts = append(d.puts, uri)
	return nil
}

func (d *stubDHT) Delete(_ context.Context, uri identity.URI) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.deletes = append(d.deletes, uri)
	return nil
}

func (d *stubDHT) FindCapability(_ context.Context, _ string) ([]identity.URI, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.findResult, d.findErr
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

// --- DHT wiring tests ---

func TestRegister_PopulatesDHT(t *testing.T) {
	stub := &stubDHT{}
	reg := registry.NewMemRegistry(registry.NewMemStore(), nil, stub)
	uri := mustParseURI(t, testURI1)

	if err := reg.Register(context.Background(), makeRecord(t, testURI1)); err != nil {
		t.Fatalf("Register: %v", err)
	}

	stub.mu.Lock()
	puts := stub.puts
	stub.mu.Unlock()
	if len(puts) != 1 || puts[0] != uri {
		t.Errorf("DHT.Put calls = %v, want [%v]", puts, uri)
	}
}

func TestDeregister_RemovesFromDHT(t *testing.T) {
	stub := &stubDHT{}
	reg := registry.NewMemRegistry(registry.NewMemStore(), nil, stub)
	uri := mustParseURI(t, testURI1)

	_ = reg.Register(context.Background(), makeRecord(t, testURI1))
	if err := reg.Deregister(context.Background(), uri); err != nil {
		t.Fatalf("Deregister: %v", err)
	}

	stub.mu.Lock()
	deletes := stub.deletes
	stub.mu.Unlock()
	if len(deletes) != 1 || deletes[0] != uri {
		t.Errorf("DHT.Delete calls = %v, want [%v]", deletes, uri)
	}
}

func TestUpdate_EndpointChange_RefreshesDHT(t *testing.T) {
	stub := &stubDHT{}
	reg := registry.NewMemRegistry(registry.NewMemStore(), nil, stub)
	uri := mustParseURI(t, testURI1)

	_ = reg.Register(context.Background(), makeRecord(t, testURI1))
	patch := registry.RecordPatch{
		Endpoints: map[registry.Protocol]string{registry.ProtocolA2A: "https://new.acme.com/a2a"},
	}
	if err := reg.Update(context.Background(), uri, patch); err != nil {
		t.Fatalf("Update: %v", err)
	}

	stub.mu.Lock()
	putCount := len(stub.puts)
	stub.mu.Unlock()
	if putCount != 2 { // once for Register, once for Update
		t.Errorf("DHT.Put called %d times, want 2 (register + endpoint update)", putCount)
	}
}

func TestUpdate_HealthOnly_DoesNotTouchDHT(t *testing.T) {
	stub := &stubDHT{}
	reg := registry.NewMemRegistry(registry.NewMemStore(), nil, stub)
	uri := mustParseURI(t, testURI1)

	_ = reg.Register(context.Background(), makeRecord(t, testURI1))
	if err := reg.Update(context.Background(), uri, registry.RecordPatch{Health: registry.HealthHealthy}); err != nil {
		t.Fatalf("Update health: %v", err)
	}

	stub.mu.Lock()
	putCount := len(stub.puts)
	stub.mu.Unlock()
	if putCount != 1 { // only from Register, not from health update
		t.Errorf("DHT.Put called %d times after health-only update, want 1", putCount)
	}
}

// --- Two-level discovery tests ---

func TestDiscover_TwoLevel_UsesDHT(t *testing.T) {
	stub := &stubDHT{}
	reg := registry.NewMemRegistry(registry.NewMemStore(), nil, stub)
	ctx := context.Background()

	for _, raw := range []string{testURI1, testURI2, testURI3} {
		if err := reg.Register(ctx, makeRecord(t, raw)); err != nil {
			t.Fatalf("Register %s: %v", raw, err)
		}
	}
	// Stub FindCapability to return only the two /workflow agents.
	uri1, uri2 := mustParseURI(t, testURI1), mustParseURI(t, testURI2)
	stub.mu.Lock()
	stub.findResult = []identity.URI{uri1, uri2}
	stub.mu.Unlock()

	results, err := reg.Discover(ctx, registry.Query{CapabilityPrefix: "/workflow"})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Discover via DHT: got %d results, want 2", len(results))
	}
}

func TestDiscover_DHT_Error_FallsBackToStore(t *testing.T) {
	stub := &stubDHT{findErr: errors.New("network timeout")}
	reg := registry.NewMemRegistry(registry.NewMemStore(), nil, stub)
	ctx := context.Background()

	for _, raw := range []string{testURI1, testURI2, testURI3} {
		if err := reg.Register(ctx, makeRecord(t, raw)); err != nil {
			t.Fatalf("Register %s: %v", raw, err)
		}
	}
	// DHT fails → should fall back to store.List and still return the /workflow agents.
	results, err := reg.Discover(ctx, registry.Query{CapabilityPrefix: "/workflow"})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Discover (store fallback): got %d results, want 2", len(results))
	}
}

// --- Attestation enforcement tests ---

func newRegWithVerifier(t *testing.T, trustRoot string) (*registry.MemRegistry, *identity.PASETOSigner) {
	t.Helper()
	signer, err := identity.GeneratePASETOSigner(trustRoot)
	if err != nil {
		t.Fatalf("GeneratePASETOSigner: %v", err)
	}
	verifier := identity.NewPASETOVerifier(nil)
	verifier.AddKey(trustRoot, signer.PublicKey())
	return registry.NewMemRegistry(registry.NewMemStore(), verifier, nil), signer
}

func signToken(t *testing.T, signer *identity.PASETOSigner, uri identity.URI) string {
	t.Helper()
	att, err := signer.Sign(context.Background(), uri, uri.CapabilityPath, time.Hour)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	return att.Token
}

func TestRegister_Verifier_RequiresAttestation(t *testing.T) {
	reg, _ := newRegWithVerifier(t, "acme.com")

	rec := makeRecord(t, testURI1) // no Attestation field
	if err := reg.Register(context.Background(), rec); err != registry.ErrAttestationFail {
		t.Errorf("Register without attestation: got %v, want ErrAttestationFail", err)
	}
}

func TestRegister_Verifier_RejectsInvalidAttestation(t *testing.T) {
	reg, _ := newRegWithVerifier(t, "acme.com")

	rec := makeRecord(t, testURI1)
	rec.Attestation = "v4.public.notavalidtoken"
	if err := reg.Register(context.Background(), rec); err != registry.ErrAttestationFail {
		t.Errorf("Register with bad token: got %v, want ErrAttestationFail", err)
	}
}

func TestRegister_Verifier_AcceptsValidAttestation(t *testing.T) {
	reg, signer := newRegWithVerifier(t, "acme.com")
	uri := mustParseURI(t, testURI1)

	rec := makeRecord(t, testURI1)
	rec.Attestation = signToken(t, signer, uri)
	if err := reg.Register(context.Background(), rec); err != nil {
		t.Errorf("Register with valid attestation: %v", err)
	}
}

func TestUpdate_Verifier_EndpointChange_RequiresAttestation(t *testing.T) {
	reg, signer := newRegWithVerifier(t, "acme.com")
	ctx := context.Background()

	uri := mustParseURI(t, testURI1)
	rec := makeRecord(t, testURI1)
	rec.Attestation = signToken(t, signer, uri)
	if err := reg.Register(ctx, rec); err != nil {
		t.Fatalf("Register: %v", err)
	}

	patch := registry.RecordPatch{
		Endpoints: map[registry.Protocol]string{registry.ProtocolA2A: "https://new.acme.com/a2a"},
		// no Attestation
	}
	if err := reg.Update(ctx, uri, patch); err != registry.ErrAttestationFail {
		t.Errorf("Update endpoints without attestation: got %v, want ErrAttestationFail", err)
	}
}

func TestUpdate_Verifier_EndpointChange_AcceptsAttestation(t *testing.T) {
	reg, signer := newRegWithVerifier(t, "acme.com")
	ctx := context.Background()

	uri := mustParseURI(t, testURI1)
	rec := makeRecord(t, testURI1)
	rec.Attestation = signToken(t, signer, uri)
	if err := reg.Register(ctx, rec); err != nil {
		t.Fatalf("Register: %v", err)
	}

	patch := registry.RecordPatch{
		Endpoints:   map[registry.Protocol]string{registry.ProtocolA2A: "https://new.acme.com/a2a"},
		Attestation: signToken(t, signer, uri),
	}
	if err := reg.Update(ctx, uri, patch); err != nil {
		t.Errorf("Update endpoints with valid attestation: %v", err)
	}
}

func TestUpdate_Verifier_HealthOnly_NoAttestationRequired(t *testing.T) {
	reg, signer := newRegWithVerifier(t, "acme.com")
	ctx := context.Background()

	uri := mustParseURI(t, testURI1)
	rec := makeRecord(t, testURI1)
	rec.Attestation = signToken(t, signer, uri)
	if err := reg.Register(ctx, rec); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Health-only update must succeed without attestation.
	if err := reg.Update(ctx, uri, registry.RecordPatch{Health: registry.HealthHealthy}); err != nil {
		t.Errorf("Update health without attestation: %v", err)
	}
}
