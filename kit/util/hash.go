package util

import (
	"crypto/sha256"
	"fmt"
)

func SHA256(str string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(str)))
}
