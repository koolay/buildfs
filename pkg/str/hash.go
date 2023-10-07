package str

import (
	"crypto/sha256"
	"fmt"
)

func HashString(input string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(input)))
}
