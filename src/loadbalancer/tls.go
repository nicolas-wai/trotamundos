package loadbalancer

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// HSTSHeader [ADR-12] Valor de Strict-Transport-Security: el navegador solo vuelve a usar https.
const HSTSHeader = "max-age=31536000"

// TLSMaterial [ADR-12] Certificado autofirmado de la región (válido para localhost y 127.0.0.1) y el
// x509.CertPool que lo confía. El balanceador confía solo en este certificado para hablar con las réplicas.
type TLSMaterial struct {
	Certificate tls.Certificate
	Pool        *x509.CertPool
}

// ServerConfig devuelve la configuración TLS para servir (balanceador y réplicas).
func (m TLSMaterial) ServerConfig() *tls.Config {
	return &tls.Config{Certificates: []tls.Certificate{m.Certificate}, MinVersion: tls.VersionTLS12}
}

// ClientConfig devuelve la configuración TLS para conectarse confiando solo en el certificado propio
// (sin InsecureSkipVerify).
func (m TLSMaterial) ClientConfig() *tls.Config {
	return &tls.Config{RootCAs: m.Pool, MinVersion: tls.VersionTLS12}
}

// HTTPClient arma un cliente https que confía solo en el certificado propio (pruebas, generador de carga).
func (m TLSMaterial) HTTPClient() *http.Client {
	return &http.Client{Transport: &http.Transport{TLSClientConfig: m.ClientConfig(), MaxIdleConnsPerHost: 256}}
}

// LoadOrCreateTLS [ADR-12] Reutiliza cert.pem y key.pem de dir si existen y siguen vigentes; si no, genera un
// certificado autofirmado ECDSA P-256 con la biblioteca estándar y lo guarda ahí.
func LoadOrCreateTLS(dir string) (TLSMaterial, error) {
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if m, err := loadTLS(certPath, keyPath); err == nil {
		return m, nil
	}
	certPEM, keyPEM, err := generateSelfSigned(time.Now())
	if err != nil {
		return TLSMaterial{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return TLSMaterial{}, err
	}
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return TLSMaterial{}, err
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return TLSMaterial{}, err
	}
	return newTLSMaterial(certPEM, keyPEM)
}

// NewSelfSignedTLS genera un certificado autofirmado sin tocar el disco (pruebas).
func NewSelfSignedTLS() (TLSMaterial, error) {
	certPEM, keyPEM, err := generateSelfSigned(time.Now())
	if err != nil {
		return TLSMaterial{}, err
	}
	return newTLSMaterial(certPEM, keyPEM)
}

func loadTLS(certPath, keyPath string) (TLSMaterial, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return TLSMaterial{}, err
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return TLSMaterial{}, err
	}
	m, err := newTLSMaterial(certPEM, keyPEM)
	if err != nil {
		return TLSMaterial{}, err
	}
	leaf, err := x509.ParseCertificate(m.Certificate.Certificate[0])
	if err != nil {
		return TLSMaterial{}, err
	}
	if time.Now().After(leaf.NotAfter.Add(-24 * time.Hour)) {
		return TLSMaterial{}, errors.New("certificado por vencer")
	}
	return m, nil
}

func newTLSMaterial(certPEM, keyPEM []byte) (TLSMaterial, error) {
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return TLSMaterial{}, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(certPEM) {
		return TLSMaterial{}, errors.New("certificado propio inválido")
	}
	return TLSMaterial{Certificate: pair, Pool: pool}, nil
}

func generateSelfSigned(now time.Time) (certPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 62))
	if err != nil {
		return nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{Organization: []string{"Trotamundos (prototipo)"}, CommonName: "localhost"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true, // autofirmado: se confía como su propia raíz
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("generando certificado: %w", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM, nil
}

// WithHSTS agrega Strict-Transport-Security a todas las respuestas del handler. [ADR-12]
func WithHSTS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", HSTSHeader)
		next.ServeHTTP(w, r)
	})
}
