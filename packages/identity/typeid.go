package identity

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const agentTypePrefix = "agent"

// TypeID is a type-prefixed UUIDv7 identifier for an agent.
// Format: <prefix>_<crockford-base32-uuidv7>
// Example: agent_01h455vb4pex5vsknk084sn02q
//
// TypeIDs are globally unique, lexicographically sortable by creation time,
// and stable across agent migrations.
type TypeID struct {
	prefix string
	suffix string // 26-character Crockford base32-encoded UUIDv7
}

// NewTypeID generates a new TypeID with the "agent" prefix and a fresh UUIDv7.
func NewTypeID() (TypeID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return TypeID{}, fmt.Errorf("identity: failed to generate UUIDv7: %w", err)
	}
	return TypeID{prefix: agentTypePrefix, suffix: crockfordEncode([16]byte(id))}, nil
}

// NewKnowledgeTypeID generates a TypeID with the "ki" (knowledge item) prefix.
func NewKnowledgeTypeID() (TypeID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return TypeID{}, fmt.Errorf("identity: failed to generate UUIDv7: %w", err)
	}
	return TypeID{prefix: "ki", suffix: crockfordEncode([16]byte(id))}, nil
}

// ParseTypeID parses a TypeID from its string representation.
// Returns an error if the suffix is not a valid 26-character Crockford base32 string.
func ParseTypeID(s string) (TypeID, error) {
	parts := strings.SplitN(s, "_", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return TypeID{}, fmt.Errorf("identity: invalid TypeID format: %q", s)
	}
	if _, err := crockfordDecode(parts[1]); err != nil {
		return TypeID{}, fmt.Errorf("identity: invalid TypeID suffix in %q: %w", s, err)
	}
	return TypeID{prefix: parts[0], suffix: parts[1]}, nil
}

// String returns the canonical string representation: prefix_suffix.
func (t TypeID) String() string {
	return t.prefix + "_" + t.suffix
}

// Prefix returns the type prefix (e.g. "agent", "ki").
func (t TypeID) Prefix() string { return t.prefix }

// Time returns the timestamp embedded in the UUIDv7 suffix.
func (t TypeID) Time() time.Time {
	b, err := crockfordDecode(t.suffix)
	if err != nil {
		return time.Time{}
	}
	sec, nsec := uuid.UUID(b).Time().UnixTime()
	return time.Unix(sec, nsec).UTC()
}

// MarshalJSON serializes the TypeID as its string form "prefix_suffix".
func (t TypeID) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

// UnmarshalJSON parses the TypeID from its string form "prefix_suffix".
func (t *TypeID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := ParseTypeID(s)
	if err != nil {
		return err
	}
	*t = parsed
	return nil
}
