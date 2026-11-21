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
	lb := httptest.NewServer(NewLoadBalancer([]*url.URL{ua, ub}))
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
