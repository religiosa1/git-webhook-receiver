package whreceiver

import (
	"crypto/hmac"
	"crypto/sha256"
)

func GetPayloadSignature(secret string, payload []byte) []byte {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	payloadSignature := h.Sum(nil)

	return payloadSignature
}
