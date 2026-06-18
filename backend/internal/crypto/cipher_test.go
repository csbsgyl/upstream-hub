package crypto

import (
	"strings"
	"testing"
)

func TestNewCipherRejectsPlaceholderSecret(t *testing.T) {
	_, err := NewCipher("please-change-me-to-a-long-random-secret-32bytes-min")
	if err == nil || !strings.Contains(err.Error(), "placeholder") {
		t.Fatalf("NewCipher error = %v, want placeholder rejection", err)
	}
}

func TestCipherRoundTrip(t *testing.T) {
	c, err := NewCipher("test-secret-that-is-not-a-placeholder")
	if err != nil {
		t.Fatalf("NewCipher error = %v", err)
	}
	enc, err := c.Encrypt("sensitive value")
	if err != nil {
		t.Fatalf("Encrypt error = %v", err)
	}
	got, err := c.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt error = %v", err)
	}
	if got != "sensitive value" {
		t.Fatalf("Decrypt = %q, want sensitive value", got)
	}
}
