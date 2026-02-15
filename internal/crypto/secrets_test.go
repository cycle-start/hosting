package crypto

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	plaintext := []byte("super-secret-value-123")
	encrypted, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	decrypted, err := Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("round-trip failed: got %q, want %q", decrypted, plaintext)
	}
}

func TestEmptyPlaintext(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	encrypted, err := Encrypt([]byte(""), key)
	if err != nil {
		t.Fatalf("Encrypt empty: %v", err)
	}

	decrypted, err := Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt empty: %v", err)
	}

	if len(decrypted) != 0 {
		t.Fatalf("expected empty plaintext, got %q", decrypted)
	}
}

func TestWrongKeyRejected(t *testing.T) {
	key1, _ := GenerateKey()
	key2, _ := GenerateKey()

	encrypted, err := Encrypt([]byte("secret"), key1)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	_, err = Decrypt(encrypted, key2)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

func TestTamperedCiphertextRejected(t *testing.T) {
	key, _ := GenerateKey()

	encrypted, err := Encrypt([]byte("secret"), key)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	data, _ := base64.StdEncoding.DecodeString(encrypted)
	// Flip a byte in the ciphertext portion.
	data[len(data)-1] ^= 0xff
	tampered := base64.StdEncoding.EncodeToString(data)

	_, err = Decrypt(tampered, key)
	if err == nil {
		t.Fatal("expected error decrypting tampered ciphertext")
	}
}

func TestDifferentCiphertextsForSamePlaintext(t *testing.T) {
	key, _ := GenerateKey()
	plaintext := []byte("same-value")

	enc1, _ := Encrypt(plaintext, key)
	enc2, _ := Encrypt(plaintext, key)

	if enc1 == enc2 {
		t.Fatal("expected different ciphertexts due to random nonce")
	}
}

func TestGenerateKeyLength(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("expected 32-byte key, got %d bytes", len(key))
	}
}
