// Package auth hashes passwords and generates session tokens.
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Iterations is the PBKDF2 work factor for new hashes. It follows the
// OWASP recommendation for PBKDF2-HMAC-SHA256.
var Iterations = 600_000

const (
	saltSize = 16
	keySize  = 32
	scheme   = "pbkdf2-sha256"
)

// MinPasswordLength is the shortest password accepted for an account.
const MinPasswordLength = 8

// ErrMismatch is returned when a password does not match its hash.
var ErrMismatch = errors.New("password does not match")

// HashPassword returns an encoded hash of the form
// pbkdf2-sha256$<iterations>$<salt>$<key>.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, Iterations, keySize)
	if err != nil {
		return "", err
	}
	enc := base64.RawStdEncoding
	return fmt.Sprintf("%s$%d$%s$%s", scheme, Iterations, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// VerifyPassword checks password against an encoded hash.
func VerifyPassword(encoded, password string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != scheme {
		return errors.New("unsupported password hash")
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter <= 0 {
		return errors.New("invalid password hash")
	}
	enc := base64.RawStdEncoding
	salt, err := enc.DecodeString(parts[2])
	if err != nil {
		return errors.New("invalid password hash")
	}
	want, err := enc.DecodeString(parts[3])
	if err != nil {
		return errors.New("invalid password hash")
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iter, len(want))
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrMismatch
	}
	return nil
}
