package execution

import (
	"crypto/sha256"
	"fmt"
)

func DeterministicID(value string) string {
	b := sha256.Sum256([]byte(value))
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
