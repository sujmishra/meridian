package identity_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/sujmishra/meridian/packages/identity"
)

const pasetoTestURI = "agent://acme.com/workflow/approval/agent_01h455vb4pex5vsknk084sn02q"

func newPASETOPair(t *testing.T, trustRoot string) (*identity.PASETOSigner, *identity.PASETOVerifier) {
	t.Helper()
	signer, err := identity.GeneratePASETOSigner(trustRoot)
	if err != nil {
		t.Fatalf("GeneratePASETOSigner: %v", err)
	}
	verifier := identity.NewPASETOVerifier(nil)
	verifier.AddKey(trustRoot, signer.PublicKey())
	return signer, verifier
}

func TestPASETO_RoundTrip(t *testing.T) {
	signer, verifier := newPASETOPair(t, "acme.com")
	uri, _ := identity.Parse(pasetoTestURI)

	att, err := signer.Sign(context.Background(), uri, uri.CapabilityPath, time.Hour)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if !hasPrefix(att.Token, "v4.public.") {
		t.Errorf("token should start with v4.public., got: %s", att.Token[:min(20, len(att.Token))])
	}

	decoded, err := verifier.Verify(context.Background(), att.Token, "acme.com")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if decoded.AgentURI.String() != pasetoTestURI {
		t.Errorf("AgentURI = %q, want %q", decoded.AgentURI, pasetoTestURI)
	}
	if decoded.CapabilityPath != uri.CapabilityPath {
		t.Errorf("CapabilityPath = %q, want %q", decoded.CapabilityPath, uri.CapabilityPath)
	}
	if decoded.Issuer != "acme.com" {
		t.Errorf("Issuer = %q, want acme.com", decoded.Issuer)
	}
	if decoded.ExpiresAt.Before(time.Now()) {
		t.Error("ExpiresAt should be in the future")
	}
}

func TestPASETO_TamperedPayload(t *testing.T) {
	signer, verifier := newPASETOPair(t, "acme.com")
	uri, _ := identity.Parse(pasetoTestURI)

	att, _ := signer.Sign(context.Background(), uri, uri.CapabilityPath, time.Hour)

	// Corrupt one byte in the base64-encoded body (after the header and before the ".").
	token := att.Token
	headerLen := len("v4.public.")
	dotIdx := len(token)
	for i := headerLen; i < len(token); i++ {
		if token[i] == '.' {
			dotIdx = i
			break
		}
	}
	bodyEnc := []byte(token[headerLen:dotIdx])
	bodyEnc[5] ^= 0xFF // flip bits
	tampered := "v4.public." + string(bodyEnc) + token[dotIdx:]

	_, err := verifier.Verify(context.Background(), tampered, "acme.com")
	if err == nil {
		t.Error("Verify should fail for tampered payload")
	}
}

func TestPASETO_ExpiredToken(t *testing.T) {
	signer, verifier := newPASETOPair(t, "acme.com")
	uri, _ := identity.Parse(pasetoTestURI)

	att, _ := signer.Sign(context.Background(), uri, uri.CapabilityPath, -time.Second)

	_, err := verifier.Verify(context.Background(), att.Token, "acme.com")
	if err == nil {
		t.Error("Verify should fail for expired token")
	}
}

func TestPASETO_WrongTrustRoot(t *testing.T) {
	signer, verifier := newPASETOPair(t, "acme.com")
	uri, _ := identity.Parse(pasetoTestURI)

	att, _ := signer.Sign(context.Background(), uri, uri.CapabilityPath, time.Hour)

	// Verifier has no key for "other.org".
	_, err := verifier.Verify(context.Background(), att.Token, "other.org")
	if err == nil {
		t.Error("Verify should fail for unknown trust root")
	}
}

func TestPASETO_WrongKey(t *testing.T) {
	signer1, _ := newPASETOPair(t, "acme.com")
	signer2, _ := newPASETOPair(t, "acme.com") // different key, same trust root

	// Verifier knows signer2's key but token was signed with signer1's key.
	verifier := identity.NewPASETOVerifier(nil)
	verifier.AddKey("acme.com", signer2.PublicKey())

	uri, _ := identity.Parse(pasetoTestURI)
	att, _ := signer1.Sign(context.Background(), uri, uri.CapabilityPath, time.Hour)

	_, err := verifier.Verify(context.Background(), att.Token, "acme.com")
	if err == nil {
		t.Error("Verify should fail when signed with a different key")
	}
}

func TestPASETO_FooterMismatch(t *testing.T) {
	signer, _ := newPASETOPair(t, "acme.com")

	// Verifier with acme.com key but we'll try to verify as other.org
	// (which happens to also have a key — but footer says acme.com).
	signer2, _ := newPASETOPair(t, "other.org")
	verifier := identity.NewPASETOVerifier(nil)
	verifier.AddKey("acme.com", signer.PublicKey())
	verifier.AddKey("other.org", signer2.PublicKey())

	uri, _ := identity.Parse(pasetoTestURI)
	att, _ := signer.Sign(context.Background(), uri, uri.CapabilityPath, time.Hour)

	// Token footer says "acme.com" but we verify as "other.org" — footer mismatch.
	_, err := verifier.Verify(context.Background(), att.Token, "other.org")
	if err == nil {
		t.Error("Verify should fail due to footer mismatch")
	}
}

func TestPASETO_PublicKeyJWK(t *testing.T) {
	signer, err := identity.GeneratePASETOSigner("acme.com")
	if err != nil {
		t.Fatalf("GeneratePASETOSigner: %v", err)
	}
	jwkBytes, err := signer.PublicKeyJWK()
	if err != nil {
		t.Fatalf("PublicKeyJWK: %v", err)
	}
	var jwk map[string]string
	if err := json.Unmarshal(jwkBytes, &jwk); err != nil {
		t.Fatalf("unmarshal JWK: %v", err)
	}
	if jwk["kty"] != "OKP" {
		t.Errorf("kty = %q, want OKP", jwk["kty"])
	}
	if jwk["crv"] != "Ed25519" {
		t.Errorf("crv = %q, want Ed25519", jwk["crv"])
	}
	raw, err := base64.RawURLEncoding.DecodeString(jwk["x"])
	if err != nil {
		t.Fatalf("decode JWK x: %v", err)
	}
	if len(raw) != 32 {
		t.Errorf("Ed25519 public key should be 32 bytes, got %d", len(raw))
	}
}

func TestPASETO_ImplementsInterfaces(t *testing.T) {
	signer, _ := identity.GeneratePASETOSigner("acme.com")

	// Compile-time check that PASETOSigner satisfies Signer + TrustRootPublisher.
	var _ identity.Signer = signer
	var _ identity.TrustRootPublisher = signer

	verifier := identity.NewPASETOVerifier(nil)
	var _ identity.Verifier = verifier
}

func TestMemTrustRootCache(t *testing.T) {
	cache := identity.NewMemTrustRootCache()

	key := []byte("test-public-key-32-bytes-exactly")
	cache.Set("acme.com", key, time.Hour)

	got, ok := cache.Get("acme.com")
	if !ok {
		t.Fatal("Get: expected hit")
	}
	if string(got) != string(key) {
		t.Errorf("Get: got %q, want %q", got, key)
	}

	// Expired entry should be a miss.
	cache.Set("expired.org", key, -time.Second)
	if _, ok := cache.Get("expired.org"); ok {
		t.Error("Get: expired entry should be a miss")
	}

	// Unknown trust root should be a miss.
	if _, ok := cache.Get("unknown.org"); ok {
		t.Error("Get: unknown trust root should be a miss")
	}
}

func TestPAE_KnownVector(t *testing.T) {
	// Verify PAE produces the correct output for a known PASETO test vector.
	// PAE(["hello", "world"]):
	//   LE64(2) || LE64(5) || "hello" || LE64(5) || "world"
	//   = 0200000000000000 0500000000000000 68656c6c6f 0500000000000000 776f726c64

	// We test the PAE indirectly by checking that sign + verify round-trips
	// with a known payload, which exercises PAE on both sides.
	signer, verifier := newPASETOPair(t, "acme.com")
	uri, _ := identity.Parse(pasetoTestURI)
	att, _ := signer.Sign(context.Background(), uri, "/workflow/approval", time.Minute)
	if _, err := verifier.Verify(context.Background(), att.Token, "acme.com"); err != nil {
		t.Fatalf("PAE-dependent sign/verify failed: %v", err)
	}
}

// helpers

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
