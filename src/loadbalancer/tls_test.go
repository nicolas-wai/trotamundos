package loadbalancer

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tlsServer(t *testing.T, m TLSMaterial, h http.Handler) *httptest.Server {
	t.Helper()
	s := httptest.NewUnstartedServer(h)
	s.TLS = m.ServerConfig()
	s.StartTLS()
	t.Cleanup(s.Close)
	return s
}

func okHandler(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, name) })
}

// [ADR-12] Una petición http:// al puerto del balanceador no se sirve; una https:// con el certificado propio responde 200.
func TestBalancerOnlySpeaksHTTPS(t *testing.T) {
	m, err := NewSelfSignedTLS()
	if err != nil {
		t.Fatal(err)
	}
	backend := tlsServer(t, m, okHandler("a"))
	bu, _ := url.Parse(backend.URL)
	lb := tlsServer(t, m, NewLoadBalancer([]Backend{{ID: "a", URL: bu}}, WithBackendTLS(m)))

	resp, err := http.Get(strings.Replace(lb.URL, "https://", "http://", 1) + "/")
	if err == nil {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusOK || string(body) == "a" {
			t.Fatalf("el balanceador sirvió por http: %d %q", resp.StatusCode, body)
		}
	}

	resp, err = m.HTTPClient().Get(lb.URL + "/")
	if err != nil {
		t.Fatalf("https con el certificado propio debía funcionar: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "a" {
		t.Fatalf("respuesta %d %q", resp.StatusCode, body)
	}
	if got := resp.Header.Values("Strict-Transport-Security"); len(got) != 1 || got[0] != HSTSHeader {
		t.Fatalf("HSTS = %v, se esperaba una sola cabecera %q", got, HSTSHeader)
	}
	// Un cliente que no confía en el certificado propio (sin pool) lo rechaza.
	if _, err := http.Get(lb.URL + "/"); err == nil {
		t.Fatal("un cliente sin el certificado propio debía rechazar la conexión")
	}
}

// [ADR-12] El balanceador rechaza a una réplica que presenta un certificado distinto del propio.
func TestBalancerRejectsReplicaWithDifferentCertificate(t *testing.T) {
	own, _ := NewSelfSignedTLS()
	other, _ := NewSelfSignedTLS()
	backend := tlsServer(t, other, okHandler("impostor"))
	bu, _ := url.Parse(backend.URL)
	lbh := NewLoadBalancer([]Backend{{ID: "a", URL: bu}}, WithBackendTLS(own))
	lb := tlsServer(t, own, lbh)

	resp, err := own.HTTPClient().Get(lb.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusServiceUnavailable || strings.Contains(string(body), "impostor") {
		t.Fatalf("la réplica con otro certificado debía rechazarse: %d %q", resp.StatusCode, body)
	}
}

func TestLoadOrCreateTLSReusesFilesOnDisk(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tls")
	first, err := LoadOrCreateTLS(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreateTLS(dir)
	if err != nil {
		t.Fatal(err)
	}
	if string(first.Certificate.Certificate[0]) != string(second.Certificate.Certificate[0]) {
		t.Fatal("debía reutilizar el certificado guardado")
	}
	if info, err := os.Stat(filepath.Join(dir, "key.pem")); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("la clave privada debía quedar con permisos 0600: %v %v", info, err)
	}
}
