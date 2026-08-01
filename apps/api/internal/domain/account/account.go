// Package account defines authenticated account ownership without depending
// on PostgreSQL, OAuth, HTTP, or another infrastructure concern.
package account

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	// ErrInvalidID reports a non-canonical account identifier.
	ErrInvalidID = errors.New("invalid account id")
	// ErrNotFound reports that an account does not exist or is not visible to
	// the authenticated owner.
	ErrNotFound = errors.New("account not found")
)

// ID is a canonical, opaque UUID used as an account ownership boundary.
type ID [16]byte

// NewID creates a random RFC 4122 version 4 identifier.
func NewID() (ID, error) {
	return newID(rand.Reader)
}

// ParseID accepts only the lower-case canonical UUID representation.
func ParseID(raw string) (ID, error) {
	var id ID
	if len(raw) != 36 ||
		raw[8] != '-' ||
		raw[13] != '-' ||
		raw[18] != '-' ||
		raw[23] != '-' ||
		raw != strings.ToLower(raw) {
		return ID{}, ErrInvalidID
	}
	compact := raw[0:8] + raw[9:13] + raw[14:18] + raw[19:23] + raw[24:36]
	decoded, err := hex.DecodeString(compact)
	if err != nil || len(decoded) != len(id) {
		return ID{}, ErrInvalidID
	}
	copy(id[:], decoded)
	if id == (ID{}) {
		return ID{}, ErrInvalidID
	}

	return id, nil
}

// String returns the canonical lower-case UUID representation.
func (id ID) String() string {
	encoded := hex.EncodeToString(id[:])
	return fmt.Sprintf(
		"%s-%s-%s-%s-%s",
		encoded[0:8],
		encoded[8:12],
		encoded[12:16],
		encoded[16:20],
		encoded[20:32],
	)
}

// OwnedDataSummary contains counts of data deleted with an account. It
// intentionally excludes content and identifiers.
type OwnedDataSummary struct {
	Bookmarks     int64
	Identities    int64
	Preferences   int64
	SavedSearches int64
	Sessions      int64
}

func newID(random io.Reader) (ID, error) {
	var id ID
	if _, err := io.ReadFull(random, id[:]); err != nil {
		return ID{}, fmt.Errorf("generate account id: %w", err)
	}
	id[6] = (id[6] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80

	return id, nil
}
