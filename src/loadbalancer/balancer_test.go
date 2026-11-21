package loadbalancer

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func backend(name string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, name)
	}))
}

func TestRoundRobinAlternatesReplicas(t *testing.T) {
	a, b := backend("A"), backend("B")
	defer a.Close()
	defer b.Close()
	ua, _ := url.Parse(a.URL)
	ub, _ := url.Parse(b.URL)
	lb := httptest.NewServer(NewLoadBalancer([]Backend{{ID: "a", URL: ua}, {ID: "b", URL: ub}}))
	defer lb.Close()

	var got string
	for i := 0; i < 4; i++ {
		resp, err := http.Get(lb.URL)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		got += string(body)
	}
	if got != "ABAB" {
		t.Fatalf("orden %q, se esperaba ABAB", got)
	}
}

func TestNoReplicasReturns503(t *testing.T) {
	rec := httptest.NewRecorder()
	NewLoadBalancer(nil).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("código %d", rec.Code)
	}
}

func TestUnhealthyReplicaIsSkippedAndReincorporated(t *testing.T) {
	a, b := backend("A"), backend("B")
	defer a.Close()
	defer b.Close()
	ua, _ := url.Parse(a.URL)
	ub, _ := url.Parse(b.URL)
	lbh := NewLoadBalancer([]Backend{{ID: "a", URL: ua}, {ID: "b", URL: ub}})
	lb := httptest.NewServer(lbh)
	defer lb.Close()
	get := func(n int) string {
		var got string
		for i := 0; i < n; i++ {
			resp, err := http.Get(lb.URL)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			got += string(body)
		}
		return got
	}

	lbh.SetHealthy("a", false)
	if got := get(3); got != "BBB" {
		t.Fatalf("con A retirada se esperaba BBB, hubo %q", got)
	}
	lbh.SetHealthy("a", true)
	if got := get(2); got != "AB" && got != "BA" {
		t.Fatalf("con A reincorporada se esperaba AB o BA, hubo %q", got)
	}
	lbh.SetHealthy("a", false)
	lbh.SetHealthy("b", false)
	rec := httptest.NewRecorder()
	lbh.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("sin réplicas vivas se esperaba 503, hubo %d", rec.Code)
	}
}
