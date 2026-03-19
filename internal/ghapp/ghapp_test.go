package ghapp

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
)

func generateTestKey(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}

func TestNew_ValidKey(t *testing.T) {
	pemKey := generateTestKey(t)
	client, err := New(12345, pemKey)
	if err != nil {
		t.Fatal(err)
	}
	if client.appID != 12345 {
		t.Errorf("appID = %d, want 12345", client.appID)
	}
}

func TestNew_InvalidPEM(t *testing.T) {
	_, err := New(1, []byte("not a pem"))
	if err == nil {
		t.Error("expected error for invalid PEM")
	}
}

func TestCreateJWT(t *testing.T) {
	pemKey := generateTestKey(t)
	client, _ := New(12345, pemKey)

	token, err := client.createJWT()
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Error("JWT should not be empty")
	}
	// JWT has 3 parts separated by dots
	parts := 0
	for _, c := range token {
		if c == '.' {
			parts++
		}
	}
	if parts != 2 {
		t.Errorf("JWT should have 3 parts (2 dots), got %d dots", parts)
	}
}
