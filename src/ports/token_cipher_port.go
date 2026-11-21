package ports

// TokenCipherPort [ADR-15][QA-08] Puerto de cifrado en reposo de los tokens OAuth del correo.
// El núcleo nunca conoce el algoritmo: el adapter (AES-GCM) vive en authentication/adapters.
type TokenCipherPort interface {
	Encrypt(plain string) ([]byte, error)
	Decrypt(cipherText []byte) (string, error)
}
