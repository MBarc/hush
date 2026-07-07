package crypto

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{7}, KeySize)
	for _, plaintext := range []string{"", "p@ssw0rd", "long value with unicode: sshh 🤫"} {
		blob, err := Encrypt(key, []byte(plaintext))
		if err != nil {
			t.Fatalf("encrypt: %v", err)
		}
		got, err := Decrypt(key, blob)
		if err != nil {
			t.Fatalf("decrypt: %v", err)
		}
		if string(got) != plaintext {
			t.Fatalf("round trip mismatch: %q != %q", got, plaintext)
		}
	}
}

func TestUniqueCiphertexts(t *testing.T) {
	key := bytes.Repeat([]byte{7}, KeySize)
	a, _ := Encrypt(key, []byte("same"))
	b, _ := Encrypt(key, []byte("same"))
	if bytes.Equal(a, b) {
		t.Fatal("two encryptions of the same value must differ (fresh DEK + nonce)")
	}
}

func TestTamperDetection(t *testing.T) {
	key := bytes.Repeat([]byte{7}, KeySize)
	blob, _ := Encrypt(key, []byte("secret"))
	blob[len(blob)-1] ^= 0xFF
	if _, err := Decrypt(key, blob); err == nil {
		t.Fatal("tampered blob must not decrypt")
	}
}

func TestWrongKeyFails(t *testing.T) {
	key := bytes.Repeat([]byte{7}, KeySize)
	other := bytes.Repeat([]byte{8}, KeySize)
	blob, _ := Encrypt(key, []byte("secret"))
	if _, err := Decrypt(other, blob); err == nil {
		t.Fatal("wrong master key must not decrypt")
	}
}

func TestLoadOrCreateMasterKey(t *testing.T) {
	dir := t.TempDir()
	k1, err := LoadOrCreateMasterKey(dir)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	k2, err := LoadOrCreateMasterKey(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !bytes.Equal(k1, k2) {
		t.Fatal("second load must return the same key")
	}
	info, err := os.Stat(filepath.Join(dir, MasterKeyFile))
	if err != nil || info.Size() != KeySize {
		t.Fatalf("key file wrong: %v size=%d", err, info.Size())
	}
}
