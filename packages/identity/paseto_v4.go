package identity

// PASETO v4.public implementation using Ed25519 signatures (stdlib only).
//
// Token format:
//
//	v4.public.<base64url(payload || sig64)>[.<base64url(footer)>]
//
// The signature covers PAE("v4.public.", payload, footer, implicit_assertion=""),
// where PAE is the Pre-Authentication Encoding defined in the PASETO spec:
//
//	PAE(pieces) = LE64(len(pieces)) || for each piece: LE64(len(piece)) || piece
//
// The trust root hostname is stored as the token footer so the verifier can
// select the right public key before checking the signature.

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	pasetoV4PublicHeader = "v4.public."
	ed25519SigLen        = 64
)

// pasetoClaims is the JSON payload embedded in a PASETO v4.public token.
// Field names follow PASETO registered claim names where applicable.
type pasetoClaims struct {
	Subject        string `json:"sub"` // agent:// URI string
	CapabilityPath string `json:"cap"` // capability path (UAR extension)
	Issuer         string `json:"iss"` // trust root hostname
	IssuedAt       string `json:"iat"` // RFC 3339
	ExpiresAt      string `json:"exp"` // RFC 3339
}

// PASETOSigner issues PASETO v4.public tokens using an Ed25519 private key.
// It also implements TrustRootPublisher so the well-known endpoint can expose
// the corresponding public key for cross-organization verification.
type PASETOSigner struct {
	privateKey ed25519.PrivateKey
	trustRoot  string
}

// GeneratePASETOSigner creates a PASETOSigner with a freshly generated Ed25519 key pair.
func GeneratePASETOSigner(trustRoot string) (*PASETOSigner, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("identity: failed to generate Ed25519 key pair: %w", err)
	}
	return &PASETOSigner{privateKey: priv, trustRoot: trustRoot}, nil
}

// NewPASETOSigner creates a PASETOSigner from an existing Ed25519 private key.
func NewPASETOSigner(trustRoot string, privateKey ed25519.PrivateKey) *PASETOSigner {
	return &PASETOSigner{privateKey: privateKey, trustRoot: trustRoot}
}

// PublicKey returns the Ed25519 public key for this signer.
// Use this to pre-load a paired PASETOVerifier without an HTTP round-trip.
func (s *PASETOSigner) PublicKey() ed25519.PublicKey {
	return s.privateKey.Public().(ed25519.PublicKey)
}

// PublicKeyJWK returns the JWK-encoded Ed25519 public key (RFC 8037 OKP format).
// Implements TrustRootPublisher — mount at GET /.well-known/agent-trust so that
// remote verifiers can fetch and cache this key.
func (s *PASETOSigner) PublicKeyJWK() ([]byte, error) {
	pub := s.privateKey.Public().(ed25519.PublicKey)
	return json.Marshal(map[string]string{
		"kty": "OKP",
		"crv": "Ed25519",
		"x":   base64.RawURLEncoding.EncodeToString(pub),
	})
}

// Sign issues a PASETO v4.public attestation token for the given agent URI and capability path.
// The trust root hostname is embedded as the token footer for key selection at verification time.
func (s *PASETOSigner) Sign(_ context.Context, agentURI URI, capabilityPath string, ttl time.Duration) (Attestation, error) {
	now := time.Now().UTC()
	exp := now.Add(ttl)
	claims := pasetoClaims{
		Subject:        agentURI.String(),
		CapabilityPath: capabilityPath,
		Issuer:         s.trustRoot,
		IssuedAt:       now.Format(time.RFC3339),
		ExpiresAt:      exp.Format(time.RFC3339),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return Attestation{}, fmt.Errorf("identity: failed to marshal PASETO claims: %w", err)
	}
	token, err := pasetoV4Sign(s.privateKey, payload, []byte(s.trustRoot))
	if err != nil {
		return Attestation{}, err
	}
	return Attestation{
		AgentURI:       agentURI,
		CapabilityPath: capabilityPath,
		Issuer:         s.trustRoot,
		IssuedAt:       now,
		ExpiresAt:      exp,
		Token:          token,
	}, nil
}

// PASETOVerifier validates PASETO v4.public tokens using Ed25519 public keys.
// It holds a per-trust-root key map; keys are pre-loaded or added at runtime.
//
// For production multi-org deployments, extend this to fetch unknown trust root
// public keys from their /.well-known/agent-trust endpoint and cache them.
type PASETOVerifier struct {
	mu   sync.RWMutex
	keys map[string]ed25519.PublicKey // trust root → Ed25519 public key
}

// NewPASETOVerifier creates a verifier pre-loaded with trust-root public keys.
func NewPASETOVerifier(keys map[string]ed25519.PublicKey) *PASETOVerifier {
	if keys == nil {
		keys = make(map[string]ed25519.PublicKey)
	}
	return &PASETOVerifier{keys: keys}
}

// AddKey registers an Ed25519 public key for a trust root.
func (v *PASETOVerifier) AddKey(trustRoot string, key ed25519.PublicKey) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.keys[trustRoot] = key
}

// Verify validates a PASETO v4.public token and returns the decoded Attestation.
func (v *PASETOVerifier) Verify(_ context.Context, token string, trustRoot string) (Attestation, error) {
	v.mu.RLock()
	key, ok := v.keys[trustRoot]
	v.mu.RUnlock()
	if !ok {
		return Attestation{}, fmt.Errorf("identity: no registered public key for trust root %q", trustRoot)
	}
	payload, err := pasetoV4Verify(key, token, []byte(trustRoot))
	if err != nil {
		return Attestation{}, err
	}
	var claims pasetoClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Attestation{}, fmt.Errorf("identity: failed to unmarshal PASETO claims: %w", err)
	}
	if claims.Issuer != trustRoot {
		return Attestation{}, fmt.Errorf("identity: token issuer %q does not match expected trust root %q", claims.Issuer, trustRoot)
	}
	exp, err := time.Parse(time.RFC3339, claims.ExpiresAt)
	if err != nil {
		return Attestation{}, fmt.Errorf("identity: invalid expiry in token: %w", err)
	}
	if time.Now().UTC().After(exp) {
		return Attestation{}, errors.New("identity: PASETO token has expired")
	}
	iat, err := time.Parse(time.RFC3339, claims.IssuedAt)
	if err != nil {
		return Attestation{}, fmt.Errorf("identity: invalid issued-at in token: %w", err)
	}
	agentURI, err := Parse(claims.Subject)
	if err != nil {
		return Attestation{}, fmt.Errorf("identity: invalid agent URI in token subject: %w", err)
	}
	return Attestation{
		AgentURI:       agentURI,
		CapabilityPath: claims.CapabilityPath,
		Issuer:         claims.Issuer,
		IssuedAt:       iat,
		ExpiresAt:      exp,
		Token:          token,
	}, nil
}

// MemTrustRootCache is an in-memory TrustRootCache with TTL expiry.
// Useful for caching public keys fetched from remote trust root well-known endpoints.
type MemTrustRootCache struct {
	mu    sync.RWMutex
	cache map[string]trustCacheEntry
}

type trustCacheEntry struct {
	key       []byte
	expiresAt time.Time // zero means no expiry
}

// NewMemTrustRootCache creates an empty in-memory trust root cache.
func NewMemTrustRootCache() *MemTrustRootCache {
	return &MemTrustRootCache{cache: make(map[string]trustCacheEntry)}
}

func (c *MemTrustRootCache) Get(trustRoot string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.cache[trustRoot]
	if !ok || (!e.expiresAt.IsZero() && time.Now().After(e.expiresAt)) {
		return nil, false
	}
	out := make([]byte, len(e.key))
	copy(out, e.key)
	return out, true
}

func (c *MemTrustRootCache) Set(trustRoot string, publicKey []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var exp time.Time
	if ttl != 0 {
		// Positive TTL: expires in the future. Negative TTL: already expired (test helper).
		// Zero TTL: no expiry (permanent cache entry).
		exp = time.Now().Add(ttl)
	}
	c.cache[trustRoot] = trustCacheEntry{key: publicKey, expiresAt: exp}
}

// --- PASETO v4.public low-level ---

// pae implements Pre-Authentication Encoding as required by the PASETO spec.
// Reference: https://github.com/paseto-standard/paseto-spec/blob/master/docs/01-Protocol-Versions/Common.md
func pae(pieces ...[]byte) []byte {
	var buf bytes.Buffer
	le64 := func(n int) {
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], uint64(n))
		buf.Write(b[:])
	}
	le64(len(pieces))
	for _, p := range pieces {
		le64(len(p))
		buf.Write(p)
	}
	return buf.Bytes()
}

// pasetoV4Sign signs payload and returns a PASETO v4.public token string.
// footer is appended base64url-encoded after a "." separator; it is NOT encrypted.
func pasetoV4Sign(sk ed25519.PrivateKey, payload, footer []byte) (string, error) {
	// Signature covers PAE(header, payload, footer, implicit_assertion="").
	m2 := pae([]byte(pasetoV4PublicHeader), payload, footer, []byte{})
	sig := ed25519.Sign(sk, m2)

	// Token body = payload || sig (64 bytes).
	body := make([]byte, len(payload), len(payload)+ed25519SigLen)
	copy(body, payload)
	body = append(body, sig...)

	token := pasetoV4PublicHeader + base64.RawURLEncoding.EncodeToString(body)
	if len(footer) > 0 {
		token += "." + base64.RawURLEncoding.EncodeToString(footer)
	}
	return token, nil
}

// pasetoV4Verify verifies a PASETO v4.public token and returns the payload.
// expectedFooter must match the footer embedded in the token.
func pasetoV4Verify(pk ed25519.PublicKey, token string, expectedFooter []byte) ([]byte, error) {
	if !strings.HasPrefix(token, pasetoV4PublicHeader) {
		return nil, errors.New("identity: not a PASETO v4.public token")
	}
	rest := token[len(pasetoV4PublicHeader):]

	// Separate body and footer on the first "." (base64url never contains ".").
	var bodyEnc, footerEnc string
	if idx := strings.IndexByte(rest, '.'); idx >= 0 {
		bodyEnc, footerEnc = rest[:idx], rest[idx+1:]
	} else {
		bodyEnc = rest
	}

	// Decode and authenticate the footer.
	var footer []byte
	if footerEnc != "" {
		var err error
		footer, err = base64.RawURLEncoding.DecodeString(footerEnc)
		if err != nil {
			return nil, fmt.Errorf("identity: failed to decode token footer: %w", err)
		}
	}
	if !bytes.Equal(footer, expectedFooter) {
		return nil, errors.New("identity: token footer does not match expected trust root")
	}

	body, err := base64.RawURLEncoding.DecodeString(bodyEnc)
	if err != nil {
		return nil, fmt.Errorf("identity: failed to decode token body: %w", err)
	}
	if len(body) < ed25519SigLen {
		return nil, errors.New("identity: token body too short to contain a 64-byte Ed25519 signature")
	}

	payload := body[:len(body)-ed25519SigLen]
	sig := body[len(body)-ed25519SigLen:]

	m2 := pae([]byte(pasetoV4PublicHeader), payload, footer, []byte{})
	if !ed25519.Verify(pk, m2, sig) {
		return nil, errors.New("identity: PASETO v4.public signature verification failed")
	}
	return payload, nil
}
