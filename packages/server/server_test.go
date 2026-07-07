package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/sujmishra/meridian/packages/identity"
	"github.com/sujmishra/meridian/packages/registry"
	"github.com/sujmishra/meridian/packages/server"
)

const (
	testAgentURI = "agent://acme.com/workflow/approval/agent_01h455vb4pex5vsknk084sn02q"
	testTrustRoot = "acme.com"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	reg := registry.NewMemRegistry(registry.NewMemStore(), nil)
	srv := server.New(reg, testTrustRoot, nil)
	return httptest.NewServer(srv)
}

func registerAgent(t *testing.T, ts *httptest.Server, agentURI string) {
	t.Helper()
	body := map[string]any{
		"agent_uri": agentURI,
		"protocols": []string{"a2a"},
		"endpoints": map[string]string{"a2a": "https://agent.acme.com/a2a"},
	}
	b, _ := json.Marshal(body)
	resp, err := ts.Client().Post(ts.URL+"/v1/agents", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST /v1/agents: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /v1/agents: status %d, want 201", resp.StatusCode)
	}
}

func TestRegister_Created(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	body := map[string]any{
		"agent_uri": testAgentURI,
		"protocols": []string{"a2a", "mcp"},
		"endpoints": map[string]string{
			"a2a": "https://agent.acme.com/a2a",
			"mcp": "https://agent.acme.com/mcp",
		},
	}
	b, _ := json.Marshal(body)
	resp, err := ts.Client().Post(ts.URL+"/v1/agents", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want 201", resp.StatusCode)
	}
	var record registry.Record
	if err := json.NewDecoder(resp.Body).Decode(&record); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if record.AgentURI.String() != testAgentURI {
		t.Errorf("AgentURI = %q, want %q", record.AgentURI, testAgentURI)
	}
	if record.SchemaVersion != "1.0" {
		t.Errorf("SchemaVersion = %q, want 1.0", record.SchemaVersion)
	}
}

func TestRegister_Duplicate(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	registerAgent(t, ts, testAgentURI)

	body := map[string]any{
		"agent_uri": testAgentURI,
		"protocols": []string{"a2a"},
		"endpoints": map[string]string{"a2a": "https://agent.acme.com/a2a"},
	}
	b, _ := json.Marshal(body)
	resp, err := ts.Client().Post(ts.URL+"/v1/agents", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want 409", resp.StatusCode)
	}
}

func TestRegister_InvalidURI(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	body := map[string]any{
		"agent_uri": "not-an-agent-uri",
		"protocols": []string{"a2a"},
	}
	b, _ := json.Marshal(body)
	resp, err := ts.Client().Post(ts.URL+"/v1/agents", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestGet_Found(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	registerAgent(t, ts, testAgentURI)

	u := ts.URL + "/v1/agents?uri=" + url.QueryEscape(testAgentURI)
	resp, err := ts.Client().Get(u)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	var record registry.Record
	if err := json.NewDecoder(resp.Body).Decode(&record); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if record.AgentURI.String() != testAgentURI {
		t.Errorf("AgentURI = %q, want %q", record.AgentURI, testAgentURI)
	}
}

func TestGet_NotFound(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	u := ts.URL + "/v1/agents?uri=" + url.QueryEscape(testAgentURI)
	resp, err := ts.Client().Get(u)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestUpdate_Health(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	registerAgent(t, ts, testAgentURI)

	patch := map[string]string{"health": "healthy"}
	b, _ := json.Marshal(patch)
	req, _ := http.NewRequest(http.MethodPatch,
		ts.URL+"/v1/agents?uri="+url.QueryEscape(testAgentURI),
		bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	var record registry.Record
	if err := json.NewDecoder(resp.Body).Decode(&record); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if record.Health != registry.HealthHealthy {
		t.Errorf("Health = %q, want healthy", record.Health)
	}
}

func TestDeregister(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	registerAgent(t, ts, testAgentURI)

	req, _ := http.NewRequest(http.MethodDelete,
		ts.URL+"/v1/agents?uri="+url.QueryEscape(testAgentURI), nil)
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("DELETE status = %d, want 204", resp.StatusCode)
	}

	// Confirm gone.
	get, _ := ts.Client().Get(ts.URL + "/v1/agents?uri=" + url.QueryEscape(testAgentURI))
	_ = get.Body.Close()
	if get.StatusCode != http.StatusNotFound {
		t.Errorf("Get after DELETE: status = %d, want 404", get.StatusCode)
	}
}

func TestDiscover(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	registerAgent(t, ts, testAgentURI)
	registerAgent(t, ts, "agent://acme.com/data/ingest/agent_01h455vb4pex5vsknk084sn02r")

	resp, err := ts.Client().Get(ts.URL + "/v1/discover?capability=/workflow")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	var result struct {
		Agents []registry.Record `json:"agents"`
		Count  int               `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Count != 1 {
		t.Errorf("count = %d, want 1 (only /workflow agent)", result.Count)
	}
}

func TestDiscover_AllAgents(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	registerAgent(t, ts, testAgentURI)
	registerAgent(t, ts, "agent://acme.com/data/ingest/agent_01h455vb4pex5vsknk084sn02r")

	resp, err := ts.Client().Get(ts.URL + "/v1/discover")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var result struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.Count != 2 {
		t.Errorf("count = %d, want 2", result.Count)
	}
}

func TestTrustRoot(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/.well-known/agent-trust")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	var result struct {
		TrustRoot string `json:"trust_root"`
		Algorithm string `json:"algorithm"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.TrustRoot != testTrustRoot {
		t.Errorf("trust_root = %q, want %q", result.TrustRoot, testTrustRoot)
	}
}

func TestHMACAttestation_RoundTrip(t *testing.T) {
	signer, err := identity.NewHMACSigner("acme.com", nil)
	if err != nil {
		t.Fatalf("NewHMACSigner: %v", err)
	}
	verifier := identity.NewHMACVerifier(map[string][]byte{"acme.com": signer.Key()})

	uri, _ := identity.Parse(testAgentURI)
	att, err := signer.Sign(context.Background(), uri, uri.CapabilityPath, 5*60*1000000000)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	decoded, err := verifier.Verify(context.Background(), att.Token, "acme.com")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if decoded.AgentURI.String() != testAgentURI {
		t.Errorf("AgentURI = %q, want %q", decoded.AgentURI, testAgentURI)
	}
	if decoded.Issuer != "acme.com" {
		t.Errorf("Issuer = %q, want acme.com", decoded.Issuer)
	}
}
