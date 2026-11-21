package httpresponse

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
)

func TestWriteErrorUsesGivenStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, http.StatusUnauthorized, apierror.TokenExpirado, "token vencido")
	if rec.Code != 401 {
		t.Fatalf("código %d", rec.Code)
	}
}
