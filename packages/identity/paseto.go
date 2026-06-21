package identity

import (
	"context"
	"time"
)

// Attestation is a signed capability claim issued by a trust root.
// It binds an agent URI to a capability path for a fixed validity window.
type Attestation struct {
	AgentURI       URI
	CapabilityPath string
	Issuer         string // trust root hostname
	IssuedAt       time.Time
	ExpiresAt      time.Time
	Token          string // raw PASETO v4 local or public token
}

// Signer issues PASETO attestation tokens on behalf of a trust root.
type Signer interface {
	// Sign issues an attestation for the given agent URI and capability path.
	// The returned Attestation carries the signed token.
	Sign(ctx context.Context, agentURI URI, capabilityPath string, ttl time.Duration) (Attestation, error)
}

// Verifier validates PASETO attestation tokens without contacting a central authority.
// Public keys are fetched from the trust root's well-known endpoint and cached.
type Verifier interface {
	// Verify validates the attestation token and returns the decoded Attestation.
	// Returns an error if the token is expired, tampered with, or issued by an
	// untrusted root.
	Verify(ctx context.Context, token string, trustRoot string) (Attestation, error)
}

// TrustRootPublisher serves the trust root's public key at the well-known endpoint.
// Mount at: GET /.well-known/agent-trust
type TrustRootPublisher interface {
	// PublicKeyJWK returns the JWK-encoded public key for this trust root.
	PublicKeyJWK() ([]byte, error)
}

// TrustRootCache caches fetched trust root public keys to avoid repeated HTTP fetches.
type TrustRootCache interface {
	// Get returns the cached public key bytes for the given trust root hostname,
	// or false if not cached.
	Get(trustRoot string) (publicKey []byte, ok bool)
	// Set stores a public key for the given trust root.
	Set(trustRoot string, publicKey []byte, ttl time.Duration)
}
