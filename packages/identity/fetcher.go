package identity

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// wellKnownResponse is the JSON shape served at /.well-known/agent-trust.
type wellKnownResponse struct {
	TrustRoot string          `json:"trust_root"`
	Algorithm string          `json:"algorithm"`
	PublicKey json.RawMessage `json:"public_key,omitempty"`
}

// edJWK is the OKP JWK encoding of an Ed25519 public key (RFC 8037).
type edJWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"` // base64url-encoded 32-byte raw public key
}

// FetchingPASETOVerifier is a Verifier that auto-fetches unknown trust root
// public keys from their /.well-known/agent-trust endpoint, caches them, and
// delegates signature verification to an underlying PASETOVerifier.
//
// Each trust root key is fetched at most once per process lifetime; subsequent
// calls for the same trust root use the already-loaded key. If a TrustRootCache
// is provided, the JWK is also persisted there so it survives across restarts.
type FetchingPASETOVerifier struct {
	mu       sync.Mutex
	verifier *PASETOVerifier
	cache    TrustRootCache // optional; may be nil
	client   *http.Client
	cacheTTL time.Duration
}

// NewFetchingPASETOVerifier creates a FetchingPASETOVerifier.
//
// client is the HTTP client used for well-known fetches; pass nil to use
// http.DefaultClient.
// cache is an optional TrustRootCache; if non-nil, fetched JWKs are stored
// with cacheTTL. Pass nil and 0 to disable persistent caching.
func NewFetchingPASETOVerifier(client *http.Client, cache TrustRootCache, cacheTTL time.Duration) *FetchingPASETOVerifier {
	if client == nil {
		client = http.DefaultClient
	}
	return &FetchingPASETOVerifier{
		verifier: NewPASETOVerifier(nil),
		cache:    cache,
		client:   client,
		cacheTTL: cacheTTL,
	}
}

// Verify validates a PASETO v4.public token issued by trustRoot.
// If the trust root public key is not yet known, it is fetched from
// https://<trustRoot>/.well-known/agent-trust before verification proceeds.
func (f *FetchingPASETOVerifier) Verify(ctx context.Context, token string, trustRoot string) (Attestation, error) {
	if err := f.ensureKey(ctx, trustRoot); err != nil {
		return Attestation{}, err
	}
	return f.verifier.Verify(ctx, token, trustRoot)
}

// ensureKey guarantees the public key for trustRoot is loaded into f.verifier.
// It uses double-checked locking to avoid duplicate fetches under concurrency.
func (f *FetchingPASETOVerifier) ensureKey(ctx context.Context, trustRoot string) error {
	if f.verifier.HasKey(trustRoot) {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.verifier.HasKey(trustRoot) {
		return nil
	}
	return f.loadKey(ctx, trustRoot)
}

// loadKey resolves the Ed25519 public key for trustRoot — from cache first,
// then from the remote well-known endpoint — and registers it in f.verifier.
// Must be called with f.mu held.
func (f *FetchingPASETOVerifier) loadKey(ctx context.Context, trustRoot string) error {
	if f.cache != nil {
		if jwkBytes, ok := f.cache.Get(trustRoot); ok {
			key, err := parseEd25519JWK(jwkBytes)
			if err == nil {
				f.verifier.AddKey(trustRoot, key)
				return nil
			}
			// Cache entry is malformed; fall through to fetch.
		}
	}

	jwkBytes, err := f.fetchJWK(ctx, trustRoot)
	if err != nil {
		return err
	}

	key, err := parseEd25519JWK(jwkBytes)
	if err != nil {
		return fmt.Errorf("identity: invalid JWK from trust root %q: %w", trustRoot, err)
	}

	f.verifier.AddKey(trustRoot, key)

	if f.cache != nil {
		f.cache.Set(trustRoot, jwkBytes, f.cacheTTL)
	}
	return nil
}

// fetchJWK GETs the well-known endpoint for trustRoot and returns the raw JWK bytes.
func (f *FetchingPASETOVerifier) fetchJWK(ctx context.Context, trustRoot string) ([]byte, error) {
	url := "https://" + trustRoot + "/.well-known/agent-trust"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("identity: failed to build request for trust root %q: %w", trustRoot, err)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("identity: failed to fetch trust root %q: %w", trustRoot, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("identity: trust root %q returned HTTP %d", trustRoot, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, fmt.Errorf("identity: failed to read response from trust root %q: %w", trustRoot, err)
	}
	var wk wellKnownResponse
	if err := json.Unmarshal(body, &wk); err != nil {
		return nil, fmt.Errorf("identity: malformed well-known response from %q: %w", trustRoot, err)
	}
	if wk.Algorithm != "paseto-v4-public" {
		return nil, fmt.Errorf("identity: trust root %q uses algorithm %q, expected paseto-v4-public", trustRoot, wk.Algorithm)
	}
	if len(wk.PublicKey) == 0 {
		return nil, fmt.Errorf("identity: trust root %q well-known response has no public_key", trustRoot)
	}
	return wk.PublicKey, nil
}

// parseEd25519JWK decodes an OKP JWK (RFC 8037) into an Ed25519 public key.
func parseEd25519JWK(data []byte) (ed25519.PublicKey, error) {
	var jwk edJWK
	if err := json.Unmarshal(data, &jwk); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JWK: %w", err)
	}
	if jwk.Kty != "OKP" || jwk.Crv != "Ed25519" {
		return nil, fmt.Errorf("expected OKP/Ed25519 JWK, got kty=%q crv=%q", jwk.Kty, jwk.Crv)
	}
	raw, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWK x field: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("Ed25519 public key must be %d bytes, got %d", ed25519.PublicKeySize, len(raw))
	}
	return ed25519.PublicKey(raw), nil
}
