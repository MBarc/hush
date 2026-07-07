package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

// TokenPrefix marks Hush API tokens.
const TokenPrefix = "hush_"

// NewToken returns a fresh API token and the hash to persist. The plaintext
// token is shown exactly once at creation time.
func NewToken() (token, hash string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", "", err
	}
	token = TokenPrefix + base64.RawURLEncoding.EncodeToString(raw)
	return token, HashToken(token), nil
}

// HashToken returns the storable hash of a token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// LooksLikeToken reports whether s has the Hush token shape.
func LooksLikeToken(s string) bool {
	return strings.HasPrefix(s, TokenPrefix) && len(s) > len(TokenPrefix)+20
}

// NewSessionID returns a random session identifier.
func NewSessionID() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

// MatchScope reports whether path is covered by the scope pattern.
// Patterns: "*" (everything), "infra/dns/*" (that subtree), or an exact
// secret path.
func MatchScope(pattern, path string) bool {
	if pattern == "*" {
		return true
	}
	if sub, ok := strings.CutSuffix(pattern, "/*"); ok {
		return path == sub || strings.HasPrefix(path, sub+"/")
	}
	return pattern == path
}

// MatchAnyScope reports whether any pattern covers path.
func MatchAnyScope(patterns []string, path string) bool {
	for _, p := range patterns {
		if MatchScope(p, path) {
			return true
		}
	}
	return false
}
