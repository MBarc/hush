// Package auth implements password hashing, API token generation, and the
// credential primitives shared by the server and CLI.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 1
	argonKeyLen  = 32
	saltLen      = 16
)

// HashPassword returns an encoded argon2id hash in the standard
// $argon2id$v=19$m=...,t=...,p=...$salt$hash format.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword reports whether password matches the encoded hash.
func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}
	var m uint32
	var t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// GeneratePassword returns a random password of length n drawn from a
// shell-safe charset.
func GeneratePassword(n int) (string, error) {
	return GenerateFromCharset(n, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.!@#%^*")
}

// GenerateFromCharset returns n random characters from charset.
func GenerateFromCharset(n int, charset string) (string, error) {
	if n <= 0 || len(charset) == 0 {
		return "", errors.New("bad generator parameters")
	}
	out := make([]byte, n)
	buf := make([]byte, 1)
	max := 256 - (256 % len(charset)) // rejection sampling for uniformity
	for i := 0; i < n; {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		if int(buf[0]) >= max {
			continue
		}
		out[i] = charset[int(buf[0])%len(charset)]
		i++
	}
	return string(out), nil
}
