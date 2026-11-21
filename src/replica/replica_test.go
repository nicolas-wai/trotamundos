package replica_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/nicolas-wai/trotamundos/src/config"
	"github.com/nicolas-wai/trotamundos/src/loadbalancer"
	"github.com/nicolas-wai/trotamundos/src/replica"
)

// Criterio de la Fase 1: una request al LB atraviesa una réplica y recibe 501 NO_IMPLEMENTADO.
func TestRequestThroughLoadBalancerIsNotImplemented(t *testing.T) {
	var targets []*url.URL
	for _, id := range []string{"r1", "r2"} {
		srv := httptest.NewServer(replica.New(id, config.Default()).Handler())
		defer srv.Close()
		u, _ := url.Parse(srv.URL)
		targets = append(targets, u)
	}
	lb := httptest.NewServer(loadbalancer.NewLoadBalancer(targets))
	defer lb.Close()

	for i := 0; i < 2; i++ { // una request por réplica (round-robin)
		resp, err := http.Get(lb.URL + "/dashboard")
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotImplemented {
			t.Fatalf("código %d", resp.StatusCode)
		}
		var body map[string]string
		if err := json.Unmarshal(raw, &body); err != nil || body["codigo"] != "NO_IMPLEMENTADO" {
			t.Fatalf("cuerpo inesperado: %s", raw)
		}
	}
}
