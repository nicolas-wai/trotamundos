package adapters

import (
	"bytes"
	"testing"
)

func TestAESGCMTokenCipherRoundTripAndTamper(t *testing.T) {
	c, err := NewAESGCMTokenCipher("clave-de-prueba")
	if err != nil {
		t.Fatal(err)
	}
	enc, err := c.Encrypt("mock-access-gmail-ana")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(enc, []byte("mock-access")) {
		t.Fatal("el texto cifrado no debe contener el token en claro")
	}
	if got, err := c.Decrypt(enc); err != nil || got != "mock-access-gmail-ana" {
		t.Fatalf("%q %v", got, err)
	}
	// Nonce aleatorio: dos cifrados del mismo texto difieren.
	if enc2, _ := c.Encrypt("mock-access-gmail-ana"); bytes.Equal(enc, enc2) {
		t.Fatal("el nonce debe ser aleatorio")
	}
	enc[len(enc)-1] ^= 0xff
	if _, err := c.Decrypt(enc); err == nil {
		t.Fatal("un texto alterado debe rechazarse")
	}
	other, _ := NewAESGCMTokenCipher("otra-clave")
	good, _ := c.Encrypt("x")
	if _, err := other.Decrypt(good); err == nil {
		t.Fatal("otra clave no debe poder descifrar")
	}
	if _, err := NewAESGCMTokenCipher(""); err == nil {
		t.Fatal("clave vacía")
	}
}
