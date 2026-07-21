package storage

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateName produces a name from a prefix by appending a random 5-character hex suffix.
func GenerateName(prefix string) string {
	b := make([]byte, 5)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)[:5]
}
