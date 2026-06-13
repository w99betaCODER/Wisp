// Package util holds small shared helpers with no project dependencies.
package util

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// NewUUID returns a random RFC 4122 version 4 UUID string.
// This is used as the VLESS client identifier for a user.
func NewUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// A machine without a working CSPRNG cannot safely run a VPN tool.
		panic(err)
	}
	// Set the version (4) and variant (RFC 4122) bits per the spec.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// NewID returns a short random hex identifier, used as a record's primary key.
func NewID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
