package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchDailyRainfall_ParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"daily":{"time":["2026-01-01","2026-01-02"],"rain_sum":[1.5,2.5]}}`))
	}))
	defer server.Close()

	c := NewClient(nil).WithBaseURL(server.URL)

	got, err := c.FetchDailyRainfall(context.Background(), -23.5, -46.6)
	if err != nil {
		t.Fatalf("FetchDailyRainfall() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d days, want 2", len(got))
	}
	if got[0].RainSumMM != 1.5 || got[1].RainSumMM != 2.5 {
		t.Errorf("rain sums = %+v", got)
	}
	if got[0].Date.Format("2006-01-02") != "2026-01-01" {
		t.Errorf("date[0] = %v, want 2026-01-01", got[0].Date)
	}
}

func TestFetchDailyRainfall_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := NewClient(nil).WithBaseURL(server.URL)

	if _, err := c.FetchDailyRainfall(context.Background(), 0, 0); err == nil {
		t.Fatal("expected an error for non-200 status, got nil")
	}
}

func TestFetchDailyRainfall_MismatchedLengths(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"daily":{"time":["2026-01-01"],"rain_sum":[1.5,2.5]}}`))
	}))
	defer server.Close()

	c := NewClient(nil).WithBaseURL(server.URL)

	if _, err := c.FetchDailyRainfall(context.Background(), 0, 0); err == nil {
		t.Fatal("expected an error for mismatched array lengths, got nil")
	}
}
