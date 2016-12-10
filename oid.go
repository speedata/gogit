package gogit

import (
	"encoding/hex"
	"errors"
	"fmt"
)

// Oid is the representation of a sha1-string
type Oid struct {
	Bytes SHA1
}

// Create a new Oid from a Sha1 string of length 40.
// In performance-sensitive paths, use NewOidFromByteString.
func NewOidFromString(sha1 string) (*Oid, error) {
	return NewOidFromByteString([]byte(sha1))
}

// Create a new Oid from a 20 byte slice.
func NewOid(b []byte) (*Oid, error) {
	if len(b) != 20 {
		return nil, errors.New("Length must be 20")
	}
	var o Oid
	copy(o.Bytes[:], b)
	return &o, nil
}

// Create a new Oid from a 40 byte hex-encoded slice.
func NewOidFromByteString(b []byte) (*Oid, error) {
	if len(b) != 40 {
		return nil, fmt.Errorf("bad hex-encoded sha1 length %d want 40", len(b))
	}
	var o Oid
	_, err := hex.Decode(o.Bytes[:], b)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// Create a new Oid from a 20 byte array
func NewOidFromArray(a SHA1) *Oid {
	return &Oid{a}
}

// Return string (hex) representation of the Oid
func (o *Oid) String() string {
	return hex.EncodeToString(o.Bytes[:])
}

// Equal reports whether o and oid2 have the same sha1.
func (o *Oid) Equal(oid2 *Oid) bool {
	return o.Bytes == oid2.Bytes
}
