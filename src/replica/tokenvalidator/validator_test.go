package tokenvalidator

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nicolas-wai/trotamundos/src/ports"
)

type tokens struct{ ports.TokenServicePort }

func (tokens) Validate(t string) (ports.Claims, error) {
	switch t {
	case "ok":
		return ports.Claims{TravelerID: "v1"}, nil
	case "vencido":
		return ports.Claims{}, ports.ErrTokenExpired
	}
	return ports.Claims{}, ports.ErrTokenInvalid
}

func TestMiddleware(t *testing.T) {
	var seen string
	h := NewValidator(tokens{}).Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = TravelerIDFrom(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))
	cases := []struct {
		header, code string
		status       int
	}{
		{"Bearer ok", "", 204},
		{"bearer ok", "", 204},
		{"Bearer vencido", "TOKEN_EXPIRADO", 401},
		{"Bearer malo", "TOKEN_INVALIDO", 401},
		{"Bearer ", "TOKEN_INVALIDO", 401},
		{"Basic ok", "TOKEN_INVALIDO", 401},
		{"", "TOKEN_INVALIDO", 401},
	}
	for _, c := range cases {
		seen = ""
		req := httptest.NewRequest("GET", "/", nil)
		if c.header != "" {
			req.Header.Set("Authorization", c.header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.status {
			t.Fatalf("%q: %d", c.header, rec.Code)
		}
		if c.code != "" {
			var b map[string]string
			_ = json.Unmarshal(rec.Body.Bytes(), &b)
			if b["codigo"] != c.code || seen != "" {
				t.Fatalf("%q: %v (el handler no debía ejecutarse)", c.header, b)
			}
		} else if seen != "v1" {
			t.Fatalf("%q: el idViajero debía inyectarse en el contexto", c.header)
		}
	}
}
