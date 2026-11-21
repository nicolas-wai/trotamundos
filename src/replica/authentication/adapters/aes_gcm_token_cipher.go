package adapters

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/nicolas-wai/trotamundos/src/ports"
)

// AESGCMTokenCipher [ADR-15][QA-08] Implementa ports.TokenCipherPort con AES-256-GCM (biblioteca estándar): los tokens
// OAuth del correo nunca se guardan en texto plano. La salida es nonce || texto cifrado y autenticado.
// [Prototipo] La clave se deriva (SHA-256) de la configuración; en producción vendría de un KMS.
type AESGCMTokenCipher struct{ aead cipher.AEAD }

var _ ports.TokenCipherPort = (*AESGCMTokenCipher)(nil)

// NewAESGCMTokenCipher crea el cifrador a partir de un secreto de configuración.
func NewAESGCMTokenCipher(secret string) (*AESGCMTokenCipher, error) {
	if secret == "" {
		return nil, errors.New("falta la clave de cifrado de tokens")
	}
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &AESGCMTokenCipher{aead: aead}, nil
}

// Encrypt cifra el texto con un nonce aleatorio nuevo.
func (c *AESGCMTokenCipher) Encrypt(plain string) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return c.aead.Seal(nonce, nonce, []byte(plain), nil), nil
}

// Decrypt descifra y verifica la autenticidad; falla si el dato fue alterado o la clave no coincide.
func (c *AESGCMTokenCipher) Decrypt(data []byte) (string, error) {
	n := c.aead.NonceSize()
	if len(data) < n {
		return "", errors.New("texto cifrado demasiado corto")
	}
	plain, err := c.aead.Open(nil, data[:n], data[n:], nil)
	if err != nil {
		return "", fmt.Errorf("descifrando token: %w", err)
	}
	return string(plain), nil
}
