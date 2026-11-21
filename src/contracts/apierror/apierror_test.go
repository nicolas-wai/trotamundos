package apierror

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteNotImplemented(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteNotImplemented(rec)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("código %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type %q", ct)
	}
	var m map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["codigo"] != "NO_IMPLEMENTADO" || m["mensaje"] == "" || len(m) != 2 {
		t.Fatalf("cuerpo inesperado: %v", m)
	}
}

func TestWriteErrorUsesGivenStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, http.StatusUnauthorized, TokenExpirado, "token vencido")
	if rec.Code != 401 {
		t.Fatalf("código %d", rec.Code)
	}
}
