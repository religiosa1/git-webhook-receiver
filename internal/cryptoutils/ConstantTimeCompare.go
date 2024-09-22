package cryptoutils

import (
	"crypto/sha256"
	"crypto/subtle"
)

// constantTimeComparer of string or []bytes values, with hashing of
// provided values, so we're comparing against the same values length
//
// @see https://github.com/golang/go/issues/18936
type constantTimeComparer struct {
	targetValueHash [32]byte
}

func NewConstantTimeComparerBytes(targetValue []byte) constantTimeComparer {
	return constantTimeComparer{sha256.Sum256(targetValue)}
}
func NewConstantTimeComparer(targetValue string) constantTimeComparer {
	return NewConstantTimeComparerBytes([]byte(targetValue))
}

func (c constantTimeComparer) EqBytes(value []byte) bool {
	valueHash := sha256.Sum256(value)
	return subtle.ConstantTimeCompare(c.targetValueHash[:], valueHash[:]) == 1
}

func (c constantTimeComparer) Eq(value string) bool {
	return c.EqBytes([]byte(value))
}
