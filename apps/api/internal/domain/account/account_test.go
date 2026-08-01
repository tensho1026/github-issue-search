package account

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestNewIDCreatesCanonicalVersionFourUUID(t *testing.T) {
	id, err := newID(bytes.NewReader(bytes.Repeat([]byte{0xff}, 16)))
	if err != nil {
		t.Fatalf("newID() error = %v", err)
	}
	if got := id.String(); got != "ffffffff-ffff-4fff-bfff-ffffffffffff" {
		t.Fatalf("newID().String() = %q", got)
	}
	parsed, err := ParseID(id.String())
	if err != nil {
		t.Fatalf("ParseID() error = %v", err)
	}
	if parsed != id {
		t.Fatalf("ParseID() = %v, want %v", parsed, id)
	}
}

func TestParseIDRejectsNonCanonicalValues(t *testing.T) {
	for _, raw := range []string{
		"",
		"00000000-0000-0000-0000-000000000000",
		"FFFFFFFF-FFFF-4FFF-BFFF-FFFFFFFFFFFF",
		"ffffffffffff4fffbfffffffffffffff",
		"not-a-uuid",
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := ParseID(raw); !errors.Is(err, ErrInvalidID) {
				t.Fatalf("ParseID(%q) error = %v", raw, err)
			}
		})
	}
}

func TestNewIDPropagatesRandomnessFailure(t *testing.T) {
	_, err := newID(strings.NewReader(""))
	if err == nil {
		t.Fatal("newID() unexpectedly succeeded")
	}
}
