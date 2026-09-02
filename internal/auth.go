package internal

import (
	"crypto/subtle"
	"encoding/base64"

	"golang.org/x/crypto/argon2"
)

func HashPassword(password string) (string, error) {
	hash := argon2.IDKey([]byte(password), nil, 1, 64*1024, 4, 32)
	return base64.RawStdEncoding.EncodeToString(hash), nil
}

func CheckPasswordHash(password, hash string) bool {
	hashBytes, err := base64.RawStdEncoding.DecodeString(hash)
	if err != nil {
		return false
	}

	computed := argon2.IDKey([]byte(password), nil, 1, 64*1024, 4, 32)
	return subtle.ConstantTimeCompare(computed, hashBytes) == 1
}
