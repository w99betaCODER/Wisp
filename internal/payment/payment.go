// Package payment provides helpers for verifying payment-gateway webhooks.
//
// A gateway signs each webhook with a shared secret (HMAC-SHA256 over the raw
// body); the panel recomputes the signature and compares it in constant time,
// so only the gateway can mark an order paid.
package payment

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// SignHMAC returns the hex-encoded HMAC-SHA256 of body keyed by secret.
func SignHMAC(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyHMAC reports whether sigHex is a valid HMAC-SHA256 of body under
// secret. The comparison is constant-time to avoid leaking the signature.
func VerifyHMAC(secret string, body []byte, sigHex string) bool {
	want, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(want, mac.Sum(nil))
}
