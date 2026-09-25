package utils

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"testing"
)

func TestEncryptUsesRandomNonceAndDecrypts(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	plaintext := []byte("sensitive value")

	first, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt first: %v", err)
	}
	second, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt second: %v", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("NewGCM: %v", err)
	}
	nonceSize := aead.NonceSize()
	if len(first) < nonceSize || len(second) < nonceSize {
		t.Fatalf("ciphertexts shorter than nonce size %d", nonceSize)
	}
	if bytes.Equal(first[:nonceSize], second[:nonceSize]) {
		t.Fatal("Encrypt reused the same nonce")
	}

	for i, ciphertext := range [][]byte{first, second} {
		got, err := Decrypt(ciphertext, key)
		if err != nil {
			t.Fatalf("Decrypt ciphertext %d: %v", i, err)
		}
		if !bytes.Equal(got, plaintext) {
			t.Errorf("Decrypt ciphertext %d = %q, want %q", i, got, plaintext)
		}
	}
}

func TestDecryptAcceptsExistingNoncePrefixFormat(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	plaintext := []byte("ciphertext written with the nonce prefix format")
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("NewGCM: %v", err)
	}

	// Prior Encrypt output used an all-zero nonce but the same [nonce|ciphertext]
	// envelope. Keep that data readable while new writes use random nonces.
	nonce := make([]byte, aead.NonceSize())
	legacyCiphertext := aead.Seal(nonce, nonce, plaintext, nil)

	got, err := Decrypt(legacyCiphertext, key)
	if err != nil {
		t.Fatalf("Decrypt legacy ciphertext: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("Decrypt legacy ciphertext = %q, want %q", got, plaintext)
	}
}
