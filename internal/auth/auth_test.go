package auth

import (
	"strings"
	"testing"
)

func TestPasswordHashVerify(t *testing.T) {
	h, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$") {
		t.Fatalf("unexpected encoding: %s", h)
	}
	if !VerifyPassword("correct horse battery staple", h) {
		t.Fatal("correct password rejected")
	}
	if VerifyPassword("wrong", h) {
		t.Fatal("wrong password accepted")
	}
	if VerifyPassword("anything", "$garbage$") {
		t.Fatal("garbage hash accepted")
	}
}

func TestTokens(t *testing.T) {
	tok, hash, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if !LooksLikeToken(tok) {
		t.Fatalf("token shape wrong: %s", tok)
	}
	if HashToken(tok) != hash {
		t.Fatal("hash mismatch")
	}
	tok2, _, _ := NewToken()
	if tok == tok2 {
		t.Fatal("tokens must be unique")
	}
}

func TestGeneratePassword(t *testing.T) {
	p1, err := GeneratePassword(24)
	if err != nil || len(p1) != 24 {
		t.Fatalf("p1=%q err=%v", p1, err)
	}
	p2, _ := GeneratePassword(24)
	if p1 == p2 {
		t.Fatal("passwords must be random")
	}
	digits, err := GenerateFromCharset(6, "0123456789")
	if err != nil || len(digits) != 6 || strings.Trim(digits, "0123456789") != "" {
		t.Fatalf("digits=%q err=%v", digits, err)
	}
}

func TestMatchScope(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"*", "anything/at/all", true},
		{"infra/dns/*", "infra/dns/cloudflare", true},
		{"infra/dns/*", "infra/dns/deep/nested", true},
		{"infra/dns/*", "infra/dnsmasq/key", false},
		{"infra/dns/*", "media/jellyfin/admin", false},
		{"infra/dns/cloudflare", "infra/dns/cloudflare", true},
		{"infra/dns/cloudflare", "infra/dns/cloudflare2", false},
	}
	for _, c := range cases {
		if got := MatchScope(c.pattern, c.path); got != c.want {
			t.Errorf("MatchScope(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}
