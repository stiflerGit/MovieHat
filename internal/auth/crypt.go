package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

func generateCryptoSecureRandomString() (string, error) {
	buf := make([]byte, 32)
	_, err := rand.Read(buf)
	if err != nil {
		return "", fmt.Errorf("rand.Read: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func tokenHash(token, secret string) (string, error) {
	hmacHash := hmac.New(sha256.New, []byte(secret))
	_, err := hmacHash.Write([]byte(token))
	if err != nil {
		return "", fmt.Errorf("hmacHash.Write: %w", err)
	}

	tokenHash := hmacHash.Sum([]byte{})
	return base64.RawStdEncoding.EncodeToString(tokenHash), nil
}
