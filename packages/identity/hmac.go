package identity

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// hmacClaims is the JSON payload embedded in an HMAC attestation token.
type hmacClaims struct {
	AgentURI       string `json:"aud"`
	CapabilityPath string `json:"cap"`
	Issuer         string `json:"iss"`
	IssuedAt       int64  `json:"iat"`
	ExpiresAt      int64  `json:"exp"`
}

const hmacTokenHeader = "hmacv1."

// HMACSigner issues HMAC-SHA256 attestation tokens on behalf of a trust root.
// Token format: "hmacv1.<base64url(json-claims)>.<base64url(hmac-sha256)>"
//
// This is a baseline implementation for development. Replace with PASETO v4 for
// production deployments that require cross-organization attestation.
type HMACSigner struct {
	key       []byte
	trustRoot string
}

// NewHMACSigner creates an HMACSigner for the given trust root.
// If key is nil, a cryptographically random 32-byte key is generated.
func NewHMACSigner(trustRoot string, key []byte) (*HMACSigner, error) {
	if len(key) == 0 {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("identity: failed to generate HMAC key: %w", err)
		}
	}
	return &HMACSigner{key: key, trustRoot: trustRoot}, nil
}

// Key returns a copy of the signing key, for sharing with a paired HMACVerifier.
func (s *HMACSigner) Key() []byte {
	out := make([]byte, len(s.key))
	copy(out, s.key)
	return out
}

// Sign issues an HMAC attestation for the given agent URI and capability path.
func (s *HMACSigner) Sign(_ context.Context, agentURI URI, capabilityPath string, ttl time.Duration) (Attestation, error) {
	now := time.Now().UTC()
	exp := now.Add(ttl)
	claims := hmacClaims{
		AgentURI:       agentURI.String(),
		CapabilityPath: capabilityPath,
		Issuer:         s.trustRoot,
		IssuedAt:       now.Unix(),
		ExpiresAt:      exp.Unix(),
	}
	token, err := encodeHMAC(s.key, claims)
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

// HMACVerifier validates HMAC-SHA256 attestation tokens.
// It holds a per-trust-root key map so it can verify tokens from multiple orgs.
type HMACVerifier struct {
	keys map[string][]byte // trust root → HMAC key
}

// NewHMACVerifier creates a verifier pre-loaded with a trust-root → key map.
func NewHMACVerifier(keys map[string][]byte) *HMACVerifier {
	if keys == nil {
		keys = make(map[string][]byte)
	}
	return &HMACVerifier{keys: keys}
}

// AddKey registers an HMAC key for a trust root.
func (v *HMACVerifier) AddKey(trustRoot string, key []byte) {
	v.keys[trustRoot] = key
}

// Verify validates the token and returns the decoded Attestation.
func (v *HMACVerifier) Verify(_ context.Context, token string, trustRoot string) (Attestation, error) {
	key, ok := v.keys[trustRoot]
	if !ok {
		return Attestation{}, fmt.Errorf("identity: no registered key for trust root %q", trustRoot)
	}
	claims, err := decodeHMAC(key, token)
	if err != nil {
		return Attestation{}, err
	}
	if claims.Issuer != trustRoot {
		return Attestation{}, fmt.Errorf("identity: token issuer %q does not match expected trust root %q", claims.Issuer, trustRoot)
	}
	if time.Now().Unix() > claims.ExpiresAt {
		return Attestation{}, errors.New("identity: attestation token has expired")
	}
	agentURI, err := Parse(claims.AgentURI)
	if err != nil {
		return Attestation{}, fmt.Errorf("identity: invalid agent URI in token claims: %w", err)
	}
	return Attestation{
		AgentURI:       agentURI,
		CapabilityPath: claims.CapabilityPath,
		Issuer:         claims.Issuer,
		IssuedAt:       time.Unix(claims.IssuedAt, 0).UTC(),
		ExpiresAt:      time.Unix(claims.ExpiresAt, 0).UTC(),
		Token:          token,
	}, nil
}

func encodeHMAC(key []byte, claims hmacClaims) (string, error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("identity: failed to encode attestation claims: %w", err)
	}
	enc := base64.RawURLEncoding.EncodeToString(payload)
	msg := hmacTokenHeader + enc
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msg))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return msg + "." + sig, nil
}

func decodeHMAC(key []byte, token string) (hmacClaims, error) {
	if len(token) <= len(hmacTokenHeader) || token[:len(hmacTokenHeader)] != hmacTokenHeader {
		return hmacClaims{}, errors.New("identity: token missing hmacv1 header")
	}
	// Split on the last "." to separate message from signature.
	lastDot := -1
	for i := len(token) - 1; i >= len(hmacTokenHeader); i-- {
		if token[i] == '.' {
			lastDot = i
			break
		}
	}
	if lastDot < 0 {
		return hmacClaims{}, errors.New("identity: malformed token: missing signature")
	}
	msg, sigEnc := token[:lastDot], token[lastDot+1:]

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msg))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sigEnc), []byte(expected)) {
		return hmacClaims{}, errors.New("identity: token signature verification failed")
	}

	payloadEnc := msg[len(hmacTokenHeader):]
	payload, err := base64.RawURLEncoding.DecodeString(payloadEnc)
	if err != nil {
		return hmacClaims{}, fmt.Errorf("identity: failed to decode token payload: %w", err)
	}
	var claims hmacClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return hmacClaims{}, fmt.Errorf("identity: failed to unmarshal token claims: %w", err)
	}
	return claims, nil
}
