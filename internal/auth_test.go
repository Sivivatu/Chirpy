package internal

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHashPassword_returnsEncodedHash(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash == "" {
		t.Fatal("HashPassword() returned empty hash")
	}

	decoded, err := base64.RawStdEncoding.DecodeString(hash)
	if err != nil {
		t.Fatalf("hash is not valid base64: %v", err)
	}
	if len(decoded) != 32 {
		t.Fatalf("expected 32-byte hash, got %d bytes", len(decoded))
	}
}

func TestHashPassword_isDeterministicWithNilSalt(t *testing.T) {
	password := "same-password"

	first, err := HashPassword(password)
	if err != nil {
		t.Fatalf("first HashPassword() error = %v", err)
	}

	second, err := HashPassword(password)
	if err != nil {
		t.Fatalf("second HashPassword() error = %v", err)
	}

	if first != second {
		t.Fatalf("expected identical hashes for same password, got %q and %q", first, second)
	}
}

func TestHashPassword_differentPasswordsProduceDifferentHashes(t *testing.T) {
	first, err := HashPassword("password-one")
	if err != nil {
		t.Fatalf("first HashPassword() error = %v", err)
	}

	second, err := HashPassword("password-two")
	if err != nil {
		t.Fatalf("second HashPassword() error = %v", err)
	}

	if first == second {
		t.Fatal("expected different hashes for different passwords")
	}
}

func TestCheckPasswordHash_acceptsMatchingPassword(t *testing.T) {
	password := "my-secret-password"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if !CheckPasswordHash(password, hash) {
		t.Fatal("CheckPasswordHash() = false, want true for matching password")
	}
}

func TestCheckPasswordHash_rejectsWrongPassword(t *testing.T) {
	hash, err := HashPassword("real-password")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if CheckPasswordHash("wrong-password", hash) {
		t.Fatal("CheckPasswordHash() = true, want false for wrong password")
	}
}

func TestCheckPasswordHash_rejectsInvalidHash(t *testing.T) {
	if CheckPasswordHash("password", "not-valid-base64!!!") {
		t.Fatal("CheckPasswordHash() = true, want false for invalid hash")
	}
}

func TestCheckPasswordHash_rejectsEmptyHash(t *testing.T) {
	if CheckPasswordHash("password", "") {
		t.Fatal("CheckPasswordHash() = true, want false for empty hash")
	}
}

func TestCheckPasswordHash_isCaseSensitive(t *testing.T) {
	password := "CaseSensitive"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if CheckPasswordHash(strings.ToLower(password), hash) {
		t.Fatal("CheckPasswordHash() = true, want false for different casing")
	}
}

func TestValidateJWT_acceptsValidToken(t *testing.T) {
	userID := uuid.New()
	secret := "test-secret"
	token, err := MakeJWT(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("MakeJWT() error = %v", err)
	}

	gotUserID, err := ValidateJWT(token, secret)
	if err != nil {
		t.Fatalf("ValidateJWT() error = %v", err)
	}
	if gotUserID != userID {
		t.Fatalf("ValidateJWT() user ID = %v, want %v", gotUserID, userID)
	}
}

func TestValidateJWT_rejectsWrongSecret(t *testing.T) {
	token, err := MakeJWT(uuid.New(), "correct-secret", time.Hour)
	if err != nil {
		t.Fatalf("MakeJWT() error = %v", err)
	}

	gotUserID, err := ValidateJWT(token, "wrong-secret")
	if err == nil {
		t.Fatal("ValidateJWT() error = nil, want an error for wrong secret")
	}
	if gotUserID != uuid.Nil {
		t.Fatalf("ValidateJWT() user ID = %v, want uuid.Nil", gotUserID)
	}
}

func TestValidateJWT_rejectsExpiredToken(t *testing.T) {
	token, err := MakeJWT(uuid.New(), "test-secret", -time.Minute)
	if err != nil {
		t.Fatalf("MakeJWT() error = %v", err)
	}

	gotUserID, err := ValidateJWT(token, "test-secret")
	if err == nil {
		t.Fatal("ValidateJWT() error = nil, want an error for expired token")
	}
	if gotUserID != uuid.Nil {
		t.Fatalf("ValidateJWT() user ID = %v, want uuid.Nil", gotUserID)
	}
}
