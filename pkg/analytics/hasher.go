package analytics

import (
	"crypto/sha256"
	"encoding/hex"
)

func hashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}
