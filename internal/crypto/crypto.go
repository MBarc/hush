// Package crypto implements Hush's envelope encryption. Every plaintext is
// sealed with a fresh random data key (AES-256-GCM), and the data key is
// sealed with the vault master key. Rotating the master key only requires
// rewrapping data keys, never re-encrypting secret values.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// KeySize is the master and data key size in bytes (AES-256).
const KeySize = 32

const blobVersion = 1

// MasterKeyFile is the file name of the master key inside the data dir.
const MasterKeyFile = "master.key"

// LoadOrCreateMasterKey returns the master key from dataDir, generating and
// persisting a new one (0600) on first boot.
func LoadOrCreateMasterKey(dataDir string) ([]byte, error) {
	p := filepath.Join(dataDir, MasterKeyFile)
	b, err := os.ReadFile(p)
	if err == nil {
		if len(b) != KeySize {
			return nil, fmt.Errorf("master key at %s is %d bytes, want %d", p, len(b), KeySize)
		}
		return b, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(p, key, 0o600); err != nil {
		return nil, fmt.Errorf("writing master key: %w", err)
	}
	return key, nil
}

// Encrypt seals plaintext with a fresh data key and wraps that key with the
// master key. Blob layout: version | wdekLen | wrappedDEK | sealedPayload.
func Encrypt(masterKey, plaintext []byte) ([]byte, error) {
	dek := make([]byte, KeySize)
	if _, err := rand.Read(dek); err != nil {
		return nil, err
	}
	wrapped, err := seal(masterKey, dek)
	if err != nil {
		return nil, err
	}
	payload, err := seal(dek, plaintext)
	if err != nil {
		return nil, err
	}
	if len(wrapped) > 255 {
		return nil, errors.New("wrapped key unexpectedly large")
	}
	blob := make([]byte, 0, 2+len(wrapped)+len(payload))
	blob = append(blob, blobVersion, byte(len(wrapped)))
	blob = append(blob, wrapped...)
	blob = append(blob, payload...)
	return blob, nil
}

// Decrypt reverses Encrypt.
func Decrypt(masterKey, blob []byte) ([]byte, error) {
	if len(blob) < 2 || blob[0] != blobVersion {
		return nil, errors.New("unrecognized ciphertext blob")
	}
	wdekLen := int(blob[1])
	if len(blob) < 2+wdekLen {
		return nil, errors.New("truncated ciphertext blob")
	}
	dek, err := open(masterKey, blob[2:2+wdekLen])
	if err != nil {
		return nil, fmt.Errorf("unwrapping data key: %w", err)
	}
	return open(dek, blob[2+wdekLen:])
}

func seal(key, plaintext []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func open(key, sealed []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, errors.New("ciphertext shorter than nonce")
	}
	return gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], nil)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("key is %d bytes, want %d", len(key), KeySize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
