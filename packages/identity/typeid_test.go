package identity

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// crockfordCharSet is the set of valid Crockford base32 characters for validation in tests.
const crockfordCharSet = "0123456789abcdefghjkmnpqrstvwxyz"

// --- Crockford codec ---

func TestCrockfordEncode_ZeroUUID(t *testing.T) {
	got := crockfordEncode([16]byte{})
	const want = "00000000000000000000000000"
	if got != want {
		t.Errorf("crockfordEncode zero: got %q, want %q", got, want)
	}
}

func TestCrockfordEncode_MaxUUID(t *testing.T) {
	var all0xFF [16]byte
	for i := range all0xFF {
		all0xFF[i] = 0xFF
	}
	got := crockfordEncode(all0xFF)
	const want = "7zzzzzzzzzzzzzzzzzzzzzzzzz"
	if got != want {
		t.Errorf("crockfordEncode 0xFF×16: got %q, want %q", got, want)
	}
}

func TestCrockfordRoundTrip(t *testing.T) {
	cases := [][16]byte{
		{},
		{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
		{0x01, 0x89, 0x0A, 0x5B, 0xC3, 0x47, 0x70, 0x00, 0xB1, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF},
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
	}
	for _, orig := range cases {
		s := crockfordEncode(orig)
		if len(s) != 26 {
			t.Errorf("encode produced %d chars, want 26 (input %v)", len(s), orig)
			continue
		}
		got, err := crockfordDecode(s)
		if err != nil {
			t.Errorf("decode(%q) error: %v", s, err)
			continue
		}
		if got != orig {
			t.Errorf("round-trip mismatch: encode→decode of %v gave %v", orig, got)
		}
	}
}

func TestCrockfordDecode_CaseInsensitive(t *testing.T) {
	lower := "01h455vb4pex5vsknk084sn02q"
	upper := strings.ToUpper(lower)
	a, err := crockfordDecode(lower)
	if err != nil {
		t.Fatalf("decode lowercase: %v", err)
	}
	b, err := crockfordDecode(upper)
	if err != nil {
		t.Fatalf("decode uppercase: %v", err)
	}
	if a != b {
		t.Error("case-insensitive decode produced different results")
	}
}

func TestCrockfordDecode_InvalidLength(t *testing.T) {
	_, err := crockfordDecode("short")
	if err == nil {
		t.Error("expected error for too-short suffix")
	}
	_, err = crockfordDecode(strings.Repeat("0", 27))
	if err == nil {
		t.Error("expected error for too-long suffix")
	}
}

func TestCrockfordDecode_InvalidChar(t *testing.T) {
	// 'i', 'l', 'o', 'u' are excluded from the Crockford alphabet.
	for _, bad := range []byte{'i', 'l', 'o', 'u'} {
		s := strings.Repeat("0", 25) + string(bad)
		if _, err := crockfordDecode(s); err == nil {
			t.Errorf("expected error for excluded char %q", bad)
		}
	}
}

func TestCrockfordDecode_Overflow(t *testing.T) {
	// First character > '7' would overflow 128 bits.
	for _, c := range []byte{'8', '9', 'a', 'z'} {
		s := string(c) + strings.Repeat("0", 25)
		if _, err := crockfordDecode(s); err == nil {
			t.Errorf("expected overflow error for first char %q", c)
		}
	}
}

// --- TypeID generation ---

func TestNewTypeID_Format(t *testing.T) {
	id, err := NewTypeID()
	if err != nil {
		t.Fatalf("NewTypeID: %v", err)
	}
	s := id.String()

	// Must have exactly one underscore separating prefix and suffix.
	parts := strings.SplitN(s, "_", 2)
	if len(parts) != 2 {
		t.Fatalf("no underscore in TypeID %q", s)
	}
	if parts[0] != "agent" {
		t.Errorf("prefix = %q, want %q", parts[0], "agent")
	}
	suffix := parts[1]
	if len(suffix) != 26 {
		t.Errorf("suffix length = %d, want 26 (got %q)", len(suffix), suffix)
	}
	if suffix[0] > '7' {
		t.Errorf("first suffix char %q exceeds '7' (overflow)", suffix[0])
	}
	for i, c := range suffix {
		if !strings.ContainsRune(crockfordCharSet, c) {
			t.Errorf("suffix char %q at index %d is not in Crockford alphabet", c, i)
		}
	}
}

func TestNewTypeID_Unique(t *testing.T) {
	a, _ := NewTypeID()
	b, _ := NewTypeID()
	if a.String() == b.String() {
		t.Error("two consecutive NewTypeID calls returned the same value")
	}
}

func TestNewTypeID_LexicographicOrder(t *testing.T) {
	// UUIDv7 timestamps ensure later IDs sort after earlier ones.
	a, _ := NewTypeID()
	b, _ := NewTypeID()
	if a.suffix > b.suffix {
		t.Errorf("earlier ID %q sorts after later ID %q", a, b)
	}
}

func TestNewKnowledgeTypeID_Prefix(t *testing.T) {
	id, err := NewKnowledgeTypeID()
	if err != nil {
		t.Fatalf("NewKnowledgeTypeID: %v", err)
	}
	if id.Prefix() != "ki" {
		t.Errorf("prefix = %q, want %q", id.Prefix(), "ki")
	}
	if len(id.suffix) != 26 {
		t.Errorf("suffix length = %d, want 26", len(id.suffix))
	}
}

// --- ParseTypeID ---

func TestParseTypeID_Valid(t *testing.T) {
	const s = "agent_01h455vb4pex5vsknk084sn02q"
	id, err := ParseTypeID(s)
	if err != nil {
		t.Fatalf("ParseTypeID(%q): %v", s, err)
	}
	if id.String() != s {
		t.Errorf("round-trip: got %q, want %q", id.String(), s)
	}
	if id.Prefix() != "agent" {
		t.Errorf("prefix = %q, want %q", id.Prefix(), "agent")
	}
}

func TestParseTypeID_MissingUnderscore(t *testing.T) {
	if _, err := ParseTypeID("nounderscore"); err == nil {
		t.Error("expected error for missing underscore")
	}
}

func TestParseTypeID_InvalidSuffix_WrongLength(t *testing.T) {
	if _, err := ParseTypeID("agent_tooshort"); err == nil {
		t.Error("expected error for too-short suffix")
	}
}

func TestParseTypeID_InvalidSuffix_BadChar(t *testing.T) {
	// 'o' is excluded from the Crockford alphabet.
	s := "agent_0" + strings.Repeat("o", 25)
	if _, err := ParseTypeID(s); err == nil {
		t.Errorf("expected error for bad Crockford char in %q", s)
	}
}

func TestParseTypeID_InvalidSuffix_Overflow(t *testing.T) {
	// First char 'a' (value 10) would overflow 128 bits.
	s := "agent_a" + strings.Repeat("0", 25)
	if _, err := ParseTypeID(s); err == nil {
		t.Errorf("expected overflow error for %q", s)
	}
}

// --- TypeID.Time ---

func TestTypeID_Time(t *testing.T) {
	before := time.Now().UTC().Truncate(time.Millisecond)
	id, err := NewTypeID()
	if err != nil {
		t.Fatalf("NewTypeID: %v", err)
	}
	after := time.Now().UTC().Add(time.Millisecond)

	got := id.Time()
	if got.IsZero() {
		t.Fatal("Time() returned zero time")
	}
	if got.Before(before) || got.After(after) {
		t.Errorf("Time() %v is outside [%v, %v]", got, before, after)
	}
}

func TestTypeID_Time_KnownSuffix(t *testing.T) {
	// Parsed TypeID with hardcoded Crockford suffix must not return zero time.
	id, err := ParseTypeID("agent_01h455vb4pex5vsknk084sn02q")
	if err != nil {
		t.Fatalf("ParseTypeID: %v", err)
	}
	got := id.Time()
	if got.IsZero() {
		t.Error("Time() returned zero time for hardcoded TypeID")
	}
}

// --- JSON ---

func TestTypeID_JSONRoundTrip(t *testing.T) {
	orig, _ := NewTypeID()

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	var got TypeID
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	if got.String() != orig.String() {
		t.Errorf("JSON round-trip: got %q, want %q", got, orig)
	}
}

func TestTypeID_JSONUnmarshal_Invalid(t *testing.T) {
	var id TypeID
	// UUID hex format (old, non-Crockford) must now be rejected.
	bad := `"agent_0196d4a1-3f2e-7c4b-8f0a-1b2c3d4e5f60"`
	if err := json.Unmarshal([]byte(bad), &id); err == nil {
		t.Error("expected error unmarshaling UUID-hex TypeID")
	}
}
