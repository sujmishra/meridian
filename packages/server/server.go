// Package server provides the HTTP API for the Unified Agent Registry.
//
// Endpoints:
//
//	POST   /v1/agents                                     Register an agent
//	GET    /v1/agents?uri=agent://...                     Get an agent by URI
//	PATCH  /v1/agents?uri=agent://...                     Update endpoints/health
//	DELETE /v1/agents?uri=agent://...                     Deregister an agent
//	GET    /v1/discover?capability=...&protocol=...       Discover agents by capability
//	GET    /.well-known/agent-trust                       Trust root metadata
package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/sujmishra/meridian/packages/identity"
	"github.com/sujmishra/meridian/packages/registry"
)

// Server is the HTTP API server for the Unified Agent Registry.
type Server struct {
	reg       registry.Registry
	trustRoot string
	signer    identity.Signer // may be nil in dev mode
	mux       *http.ServeMux
}

// New constructs a Server wired to the given registry.
// trustRoot is the DNS hostname this server represents (e.g. "acme.com").
// signer may be nil; in that case the /.well-known/agent-trust endpoint omits key material.
func New(reg registry.Registry, trustRoot string, signer identity.Signer) *Server {
	s := &Server{
		reg:       reg,
		trustRoot: trustRoot,
		signer:    signer,
		mux:       http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("POST /v1/agents", s.handleRegister)
	s.mux.HandleFunc("GET /v1/agents", s.handleGet)
	s.mux.HandleFunc("PATCH /v1/agents", s.handleUpdate)
	s.mux.HandleFunc("DELETE /v1/agents", s.handleDeregister)
	s.mux.HandleFunc("GET /v1/discover", s.handleDiscover)
	s.mux.HandleFunc("GET /.well-known/agent-trust", s.handleTrustRoot)
}

// registerRequest is the JSON body for POST /v1/agents.
type registerRequest struct {
	AgentURI    string                       `json:"agent_uri"`
	Protocols   []registry.Protocol          `json:"protocols"`
	Endpoints   map[registry.Protocol]string `json:"endpoints"`
	Attestation string                       `json:"attestation,omitempty"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	uri, err := identity.Parse(req.AgentURI)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent_uri: "+err.Error())
		return
	}
	record := registry.Record{
		AgentURI:       uri,
		TrustRoot:      uri.TrustRoot,
		CapabilityPath: uri.CapabilityPath,
		Protocols:      req.Protocols,
		Endpoints:      req.Endpoints,
		Attestation:    req.Attestation,
	}
	if err := s.reg.Register(r.Context(), record); err != nil {
		switch err {
		case registry.ErrAlreadyExists:
			writeError(w, http.StatusConflict, err.Error())
		case registry.ErrInvalidRecord:
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		case registry.ErrAttestationFail:
			writeError(w, http.StatusForbidden, err.Error())
		default:
			slog.Error("register failed", "err", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}
	registered, _ := s.reg.Get(r.Context(), uri)
	writeJSON(w, http.StatusCreated, registered)
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	uri, err := uriQueryParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	record, err := s.reg.Get(r.Context(), uri)
	if err == registry.ErrNotFound {
		writeError(w, http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, record)
}

// updateRequest is the JSON body for PATCH /v1/agents.
type updateRequest struct {
	Endpoints   map[registry.Protocol]string `json:"endpoints,omitempty"`
	Health      registry.HealthStatus        `json:"health,omitempty"`
	Attestation string                       `json:"attestation,omitempty"`
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	uri, err := uriQueryParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	patch := registry.RecordPatch{
		Endpoints:   req.Endpoints,
		Health:      req.Health,
		Attestation: req.Attestation,
	}
	if err := s.reg.Update(r.Context(), uri, patch); err != nil {
		switch err {
		case registry.ErrNotFound:
			writeError(w, http.StatusNotFound, err.Error())
		case registry.ErrAttestationFail:
			writeError(w, http.StatusForbidden, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}
	updated, _ := s.reg.Get(r.Context(), uri)
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeregister(w http.ResponseWriter, r *http.Request) {
	uri, err := uriQueryParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.reg.Deregister(r.Context(), uri); err == registry.ErrNotFound {
		writeError(w, http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDiscover(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	query := registry.Query{
		CapabilityPrefix: q.Get("capability"),
		Health:           registry.HealthStatus(q.Get("health")),
	}
	if proto := q.Get("protocol"); proto != "" {
		query.Protocols = []registry.Protocol{registry.Protocol(proto)}
	}
	if tr := q.Get("trust_root"); tr != "" {
		query.TrustRoots = []string{tr}
	}
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			query.Limit = n
		}
	}
	if query.CapabilityPrefix == "" {
		query.CapabilityPrefix = "/"
	}
	results, err := s.reg.Discover(r.Context(), query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if results == nil {
		results = []registry.Record{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"agents": results,
		"count":  len(results),
	})
}

// trustRootResponse is the /.well-known/agent-trust response.
// When the signer implements TrustRootPublisher (PASETO v4.public), the response
// includes the Ed25519 public key as a JWK so remote verifiers can fetch and
// cache it without a shared secret.
type trustRootResponse struct {
	TrustRoot string          `json:"trust_root"`
	Algorithm string          `json:"algorithm"`
	PublicKey json.RawMessage `json:"public_key,omitempty"` // JWK, present for PASETO signers
}

func (s *Server) handleTrustRoot(w http.ResponseWriter, r *http.Request) {
	resp := trustRootResponse{
		TrustRoot: s.trustRoot,
		Algorithm: "hmac-sha256",
	}
	if pub, ok := s.signer.(identity.TrustRootPublisher); ok {
		if jwk, err := pub.PublicKeyJWK(); err == nil {
			resp.Algorithm = "paseto-v4-public"
			resp.PublicKey = json.RawMessage(jwk)
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// uriQueryParam reads and parses the "uri" query parameter as an agent:// URI.
func uriQueryParam(r *http.Request) (identity.URI, error) {
	raw := r.URL.Query().Get("uri")
	if raw == "" {
		return identity.URI{}, fmt.Errorf("missing required query parameter 'uri'")
	}
	return identity.Parse(raw)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
