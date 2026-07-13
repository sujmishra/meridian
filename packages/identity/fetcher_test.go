package identity_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sujmishra/meridian/packages/identity"
)

func TestFetchingPASETOVerifier_FetchesAndVerifies(t *testing.T) {
	signer, err := identity.GeneratePASETOSigner("acme.com")
	if err != nil {
		t.Fatalf("GeneratePASETOSigner: %v", err)
	}

	var fetchCount atomic.Int32
	ts := httptest.NewServer(wellKnownHandler("acme.com", signer, &fetchCount))
	defer ts.Close()

	verifier := identity.NewFetchingPASETOVerifier(
		&http.Client{Transport: redirectTransport(ts)},
		nil, 0,
	)

	uri, _ := identity.Parse(pasetoTestURI)
	att, _ := signer.Sign(context.Background(), uri, uri.CapabilityPath, time.Hour)

	got, err := verifier.Verify(context.Background(), att.Token, "acme.com")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.Issuer != "acme.com" {
		t.Errorf("Issuer = %q, want acme.com", got.Issuer)
	}
	if got.AgentURI.String() != pasetoTestURI {
		t.Errorf("AgentURI = %q, want %q", got.AgentURI, pasetoTestURI)
	}
	if fetchCount.Load() != 1 {
		t.Errorf("expected 1 fetch, got %d", fetchCount.Load())
	}
}

func TestFetchingPASETOVerifier_KeyFetchedOnce(t *testing.T) {
	signer, _ := identity.GeneratePASETOSigner("acme.com")

	var fetchCount atomic.Int32
	ts := httptest.NewServer(wellKnownHandler("acme.com", signer, &fetchCount))
	defer ts.Close()

	verifier := identity.NewFetchingPASETOVerifier(
		&http.Client{Transport: redirectTransport(ts)},
		nil, 0,
	)

	uri, _ := identity.Parse(pasetoTestURI)
	att, _ := signer.Sign(context.Background(), uri, uri.CapabilityPath, time.Hour)

	for range 5 {
		if _, err := verifier.Verify(context.Background(), att.Token, "acme.com"); err != nil {
			t.Fatalf("Verify: %v", err)
		}
	}

	if fetchCount.Load() != 1 {
		t.Errorf("expected key fetched exactly once, got %d fetches", fetchCount.Load())
	}
}

func TestFetchingPASETOVerifier_UsesCache(t *testing.T) {
	signer, _ := identity.GeneratePASETOSigner("acme.com")
	jwk, _ := signer.PublicKeyJWK()

	cache := identity.NewMemTrustRootCache()
	cache.Set("acme.com", jwk, time.Hour)

	// Server must not be contacted when the cache is warm.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("remote fetch should not occur when cache is warm")
		http.Error(w, "unexpected", http.StatusInternalServerError)
	}))
	defer ts.Close()

	verifier := identity.NewFetchingPASETOVerifier(
		&http.Client{Transport: redirectTransport(ts)},
		cache, time.Hour,
	)

	uri, _ := identity.Parse(pasetoTestURI)
	att, _ := signer.Sign(context.Background(), uri, uri.CapabilityPath, time.Hour)

	if _, err := verifier.Verify(context.Background(), att.Token, "acme.com"); err != nil {
		t.Fatalf("Verify with warm cache: %v", err)
	}
}

func TestFetchingPASETOVerifier_PopulatesCache(t *testing.T) {
	signer, _ := identity.GeneratePASETOSigner("acme.com")

	ts := httptest.NewServer(wellKnownHandler("acme.com", signer, nil))
	defer ts.Close()

	cache := identity.NewMemTrustRootCache()
	verifier := identity.NewFetchingPASETOVerifier(
		&http.Client{Transport: redirectTransport(ts)},
		cache, time.Hour,
	)

	uri, _ := identity.Parse(pasetoTestURI)
	att, _ := signer.Sign(context.Background(), uri, uri.CapabilityPath, time.Hour)

	if _, err := verifier.Verify(context.Background(), att.Token, "acme.com"); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	if _, ok := cache.Get("acme.com"); !ok {
		t.Error("expected JWK to be stored in cache after fetch")
	}
}

func TestFetchingPASETOVerifier_ExpiredCacheFallsThrough(t *testing.T) {
	signer, _ := identity.GeneratePASETOSigner("acme.com")
	jwk, _ := signer.PublicKeyJWK()

	cache := identity.NewMemTrustRootCache()
	cache.Set("acme.com", jwk, -time.Second) // already expired

	var fetchCount atomic.Int32
	ts := httptest.NewServer(wellKnownHandler("acme.com", signer, &fetchCount))
	defer ts.Close()

	verifier := identity.NewFetchingPASETOVerifier(
		&http.Client{Transport: redirectTransport(ts)},
		cache, time.Hour,
	)

	uri, _ := identity.Parse(pasetoTestURI)
	att, _ := signer.Sign(context.Background(), uri, uri.CapabilityPath, time.Hour)

	if _, err := verifier.Verify(context.Background(), att.Token, "acme.com"); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if fetchCount.Load() != 1 {
		t.Errorf("expected remote fetch on cache miss, got %d fetches", fetchCount.Load())
	}
}

func TestFetchingPASETOVerifier_ServerDown(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ts.Close() // shut down immediately

	verifier := identity.NewFetchingPASETOVerifier(
		&http.Client{Transport: redirectTransport(ts)},
		nil, 0,
	)

	uri, _ := identity.Parse(pasetoTestURI)
	signer, _ := identity.GeneratePASETOSigner("acme.com")
	att, _ := signer.Sign(context.Background(), uri, uri.CapabilityPath, time.Hour)

	_, err := verifier.Verify(context.Background(), att.Token, "acme.com")
	if err == nil {
		t.Error("Verify should fail when well-known server is unreachable")
	}
}

func TestFetchingPASETOVerifier_WrongAlgorithm(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"trust_root": "acme.com",
			"algorithm":  "hmac-sha256",
		})
	}))
	defer ts.Close()

	verifier := identity.NewFetchingPASETOVerifier(
		&http.Client{Transport: redirectTransport(ts)},
		nil, 0,
	)

	uri, _ := identity.Parse(pasetoTestURI)
	signer, _ := identity.GeneratePASETOSigner("acme.com")
	att, _ := signer.Sign(context.Background(), uri, uri.CapabilityPath, time.Hour)

	_, err := verifier.Verify(context.Background(), att.Token, "acme.com")
	if err == nil {
		t.Error("Verify should fail when trust root advertises non-PASETO algorithm")
	}
}

func TestFetchingPASETOVerifier_HTTP404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()

	verifier := identity.NewFetchingPASETOVerifier(
		&http.Client{Transport: redirectTransport(ts)},
		nil, 0,
	)

	uri, _ := identity.Parse(pasetoTestURI)
	signer, _ := identity.GeneratePASETOSigner("acme.com")
	att, _ := signer.Sign(context.Background(), uri, uri.CapabilityPath, time.Hour)

	_, err := verifier.Verify(context.Background(), att.Token, "acme.com")
	if err == nil {
		t.Error("Verify should fail on HTTP 404 from well-known endpoint")
	}
}

func TestFetchingPASETOVerifier_ImplementsVerifier(t *testing.T) {
	var _ identity.Verifier = identity.NewFetchingPASETOVerifier(nil, nil, 0)
}

// --- helpers ---

// wellKnownHandler serves a valid /.well-known/agent-trust response for trustRoot.
// count is incremented on each request; pass nil to skip counting.
func wellKnownHandler(trustRoot string, signer *identity.PASETOSigner, count *atomic.Int32) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if count != nil {
			count.Add(1)
		}
		jwk, err := signer.PublicKeyJWK()
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"trust_root": trustRoot,
			"algorithm":  "paseto-v4-public",
			"public_key": json.RawMessage(jwk),
		})
	})
}

// redirectTransport returns a RoundTripper that rewrites all requests to ts,
// preserving the path and query. Used to intercept HTTPS well-known fetches.
func redirectTransport(ts *httptest.Server) http.RoundTripper {
	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		r := req.Clone(req.Context())
		r.URL.Scheme = "http"
		r.URL.Host = ts.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(r)
	})
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
