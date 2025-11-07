package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

func ComputeHMAC(message []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(message)
	return hex.EncodeToString(mac.Sum(nil))
}

func VerifyHMAC(message []byte, secret string, signature string) bool {
	expected := ComputeHMAC(message, secret)
	return hmac.Equal([]byte(expected), []byte(signature))
}

func GenerateSecret(bytesLen int) (string, error) {
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

