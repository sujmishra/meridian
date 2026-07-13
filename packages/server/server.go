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

	"github.com/gin-gonic/gin"
	"github.com/sujmishra/meridian/packages/identity"
	"github.com/sujmishra/meridian/packages/registry"
)

// Server is the HTTP API server for the Unified Agent Registry.
type Server struct {
	reg       registry.Registry
	trustRoot string
	signer    identity.Signer // may be nil in dev mode
	engine    *gin.Engine
}

// New constructs a Server wired to the given registry.
// trustRoot is the DNS hostname this server represents (e.g. "acme.com").
// signer may be nil; in that case the /.well-known/agent-trust endpoint omits key material.
func New(reg registry.Registry, trustRoot string, signer identity.Signer) *Server {
	s := &Server{
		reg:       reg,
		trustRoot: trustRoot,
		signer:    signer,
		engine:    gin.New(),
	}
	s.engine.Use(gin.Recovery())
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.engine.ServeHTTP(w, r)
}

func (s *Server) routes() {
	v1 := s.engine.Group("/v1")
	v1.POST("/agents", s.handleRegister)
	v1.GET("/agents", s.handleGet)
	v1.PATCH("/agents", s.handleUpdate)
	v1.DELETE("/agents", s.handleDeregister)
	v1.GET("/discover", s.handleDiscover)
	s.engine.GET("/.well-known/agent-trust", s.handleTrustRoot)
}

// registerRequest is the JSON body for POST /v1/agents.
type registerRequest struct {
	AgentURI    string                       `json:"agent_uri"`
	Protocols   []registry.Protocol          `json:"protocols"`
	Endpoints   map[registry.Protocol]string `json:"endpoints"`
	Attestation string                       `json:"attestation,omitempty"`
}

func (s *Server) handleRegister(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}
	uri, err := identity.Parse(req.AgentURI)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent_uri: " + err.Error()})
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
	if err := s.reg.Register(c.Request.Context(), record); err != nil {
		switch err {
		case registry.ErrAlreadyExists:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case registry.ErrInvalidRecord:
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case registry.ErrAttestationFail:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			slog.Error("register failed", "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	registered, _ := s.reg.Get(c.Request.Context(), uri)
	c.JSON(http.StatusCreated, registered)
}

func (s *Server) handleGet(c *gin.Context) {
	uri, err := uriQueryParam(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	record, err := s.reg.Get(c.Request.Context(), uri)
	if err == registry.ErrNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, record)
}

// updateRequest is the JSON body for PATCH /v1/agents.
type updateRequest struct {
	Endpoints   map[registry.Protocol]string `json:"endpoints,omitempty"`
	Health      registry.HealthStatus        `json:"health,omitempty"`
	Attestation string                       `json:"attestation,omitempty"`
}

func (s *Server) handleUpdate(c *gin.Context) {
	uri, err := uriQueryParam(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}
	patch := registry.RecordPatch{
		Endpoints:   req.Endpoints,
		Health:      req.Health,
		Attestation: req.Attestation,
	}
	if err := s.reg.Update(c.Request.Context(), uri, patch); err != nil {
		switch err {
		case registry.ErrNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case registry.ErrAttestationFail:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	updated, _ := s.reg.Get(c.Request.Context(), uri)
	c.JSON(http.StatusOK, updated)
}

func (s *Server) handleDeregister(c *gin.Context) {
	uri, err := uriQueryParam(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.reg.Deregister(c.Request.Context(), uri); err == registry.ErrNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) handleDiscover(c *gin.Context) {
	query := registry.Query{
		CapabilityPrefix: c.Query("capability"),
		Health:           registry.HealthStatus(c.Query("health")),
	}
	if proto := c.Query("protocol"); proto != "" {
		query.Protocols = []registry.Protocol{registry.Protocol(proto)}
	}
	if tr := c.Query("trust_root"); tr != "" {
		query.TrustRoots = []string{tr}
	}
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			query.Limit = n
		}
	}
	if query.CapabilityPrefix == "" {
		query.CapabilityPrefix = "/"
	}
	results, err := s.reg.Discover(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if results == nil {
		results = []registry.Record{}
	}
	c.JSON(http.StatusOK, gin.H{
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

func (s *Server) handleTrustRoot(c *gin.Context) {
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
	c.JSON(http.StatusOK, resp)
}

// uriQueryParam reads and parses the "uri" query parameter as an agent:// URI.
func uriQueryParam(c *gin.Context) (identity.URI, error) {
	raw := c.Query("uri")
	if raw == "" {
		return identity.URI{}, fmt.Errorf("missing required query parameter 'uri'")
	}
	return identity.Parse(raw)
}
