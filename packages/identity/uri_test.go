package identity_test

import (
	"testing"

	"github.com/sujmishra/meridian/packages/identity"
)

func TestParseURI(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
		check   func(t *testing.T, u identity.URI)
	}{
		{
			name: "valid URI",
			raw:  "agent://acme.com/workflow/approval/agent_01h455vb4pex5vsknk084sn02q",
			check: func(t *testing.T, u identity.URI) {
				if u.TrustRoot != "acme.com" {
					t.Errorf("TrustRoot = %q, want %q", u.TrustRoot, "acme.com")
				}
				if u.CapabilityPath != "/workflow/approval" {
					t.Errorf("CapabilityPath = %q, want %q", u.CapabilityPath, "/workflow/approval")
				}
			},
		},
		{
			name:    "wrong scheme",
			raw:     "https://acme.com/workflow/approval/agent_01h455vb4pex5vsknk084sn02q",
			wantErr: true,
		},
		{
			name:    "missing trust root",
			raw:     "agent:///workflow/approval/agent_01h455vb4pex5vsknk084sn02q",
			wantErr: true,
		},
		{
			name:    "path too short",
			raw:     "agent://acme.com/agent_01h455vb4pex5vsknk084sn02q",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u, err := identity.Parse(tc.raw)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Parse(%q) error = %v, wantErr %v", tc.raw, err, tc.wantErr)
			}
			if tc.check != nil {
				tc.check(t, u)
			}
		})
	}
}

func TestURIRoundTrip(t *testing.T) {
	raw := "agent://acme.com/workflow/approval/agent_01h455vb4pex5vsknk084sn02q"
	u, err := identity.Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := u.String(); got != raw {
		t.Errorf("String() = %q, want %q", got, raw)
	}
}

func TestMatchesCapabilityPrefix(t *testing.T) {
	u, _ := identity.Parse("agent://acme.com/workflow/approval/agent_01h455vb4pex5vsknk084sn02q")

	if !u.MatchesCapabilityPrefix("/workflow") {
		t.Error("expected /workflow/approval to match prefix /workflow")
	}
	if !u.MatchesCapabilityPrefix("/workflow/approval") {
		t.Error("expected /workflow/approval to match itself")
	}
	if u.MatchesCapabilityPrefix("/data") {
		t.Error("expected /workflow/approval not to match /data")
	}
}
