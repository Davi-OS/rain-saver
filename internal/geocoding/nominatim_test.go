package geocoding

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"
)

// fakeNominatim answers from a table keyed by the "street" param ("" for the
// city-only query) and records every query.
type fakeNominatim struct {
	mu      sync.Mutex
	queries []url.Values
	agents  []string
	answers map[string]string
	status  int
}

func (f *fakeNominatim) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries = append(f.queries, r.URL.Query())
	f.agents = append(f.agents, r.UserAgent())
	if f.status != 0 {
		w.WriteHeader(f.status)
		return
	}
	body, ok := f.answers[r.URL.Query().Get("street")]
	if !ok {
		body = "[]"
	}
	_, _ = w.Write([]byte(body))
}

func newTestNominatim(t *testing.T, f *fakeNominatim) *Nominatim {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	n := NewNominatim(srv.URL, "rain-saver-test")
	n.minInterval = 0
	return n
}

var paulista = Address{Logradouro: "Avenida Paulista", Number: "1000", Cidade: "São Paulo", Estado: "SP", Cep: "01310-100"}

func TestNominatimFirstQueryIsStreetWithNumber(t *testing.T) {
	f := &fakeNominatim{answers: map[string]string{"1000 Avenida Paulista": `[{"lat":"-23.56","lon":"-46.65"}]`}}
	p, err := newTestNominatim(t, f).Geocode(context.Background(), paulista)
	if err != nil {
		t.Fatalf("Geocode: %v", err)
	}
	if p.Latitude != -23.56 || p.Longitude != -46.65 {
		t.Errorf("point = %+v", p)
	}
	if len(f.queries) != 1 {
		t.Fatalf("queries = %d, want 1", len(f.queries))
	}
	q := f.queries[0]
	if q.Get("city") != "São Paulo" || q.Get("state") != "SP" || q.Get("countrycodes") != "br" {
		t.Errorf("unexpected query %v", q)
	}
	if q.Has("postalcode") {
		t.Error("postalcode must not be sent (it breaks Nominatim matches in Brazil)")
	}
	if f.agents[0] != "rain-saver-test" {
		t.Errorf("User-Agent = %q", f.agents[0])
	}
}

func TestNominatimFallsBackToStreetThenCity(t *testing.T) {
	f := &fakeNominatim{answers: map[string]string{"": `[{"lat":"-23.5","lon":"-46.6"}]`}}
	p, err := newTestNominatim(t, f).Geocode(context.Background(), paulista)
	if err != nil {
		t.Fatalf("Geocode: %v", err)
	}
	if p.Latitude != -23.5 {
		t.Errorf("point = %+v, want the city fallback", p)
	}
	got := []string{f.queries[0].Get("street"), f.queries[1].Get("street"), f.queries[2].Get("street")}
	want := []string{"1000 Avenida Paulista", "Avenida Paulista", ""}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("query %d street = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNominatimNotFoundAndUnavailable(t *testing.T) {
	if _, err := newTestNominatim(t, &fakeNominatim{}).Geocode(context.Background(), paulista); !errors.Is(err, ErrNotFound) {
		t.Errorf("no match: err = %v, want ErrNotFound", err)
	}
	if _, err := newTestNominatim(t, &fakeNominatim{status: http.StatusTooManyRequests}).Geocode(context.Background(), paulista); !errors.Is(err, ErrUnavailable) {
		t.Errorf("429: err = %v, want ErrUnavailable", err)
	}
	f := &fakeNominatim{}
	if _, err := newTestNominatim(t, f).Geocode(context.Background(), Address{Logradouro: "Rua X", Number: "1"}); !errors.Is(err, ErrNotFound) || len(f.queries) != 0 {
		t.Errorf("no city/state: err = %v, queries = %d; want ErrNotFound without calling the provider", err, len(f.queries))
	}
}

func TestNominatimRateLimit(t *testing.T) {
	f := &fakeNominatim{answers: map[string]string{"1000 Avenida Paulista": `[{"lat":"1","lon":"2"}]`}}
	n := newTestNominatim(t, f)
	n.minInterval = 150 * time.Millisecond

	start := time.Now()
	for i := 0; i < 3; i++ {
		if _, err := n.Geocode(context.Background(), paulista); err != nil {
			t.Fatal(err)
		}
	}
	if elapsed := time.Since(start); elapsed < 300*time.Millisecond {
		t.Errorf("3 requests took %v, want them spaced by at least 150ms", elapsed)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := n.Geocode(ctx, paulista); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled ctx: err = %v, want context.Canceled", err)
	}
}
