package geocoding

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultNominatimURL is the public OpenStreetMap instance. Its usage policy
// (https://operations.osmfoundation.org/policies/nominatim/) requires at most
// 1 request/second and an identifying User-Agent, both enforced here.
const DefaultNominatimURL = "https://nominatim.openstreetmap.org"

// Nominatim geocodes Brazilian addresses with Nominatim's structured search.
//
// Queries go from most to least precise and stop at the first match: street
// with number, street without number, then the city itself. The city centre
// is coarse, but enough to pick the nearest INMET station (tens of km
// apart). The CEP is deliberately not sent: Nominatim's postcode coverage in
// Brazil is poor and including it makes otherwise good queries return nothing.
type Nominatim struct {
	baseURL     string
	userAgent   string
	client      *http.Client
	minInterval time.Duration

	mu   sync.Mutex
	last time.Time
}

func NewNominatim(baseURL, userAgent string) *Nominatim {
	return &Nominatim{
		baseURL:     strings.TrimRight(baseURL, "/"),
		userAgent:   userAgent,
		client:      &http.Client{Timeout: 5 * time.Second},
		minInterval: time.Second,
	}
}

func (n *Nominatim) Geocode(ctx context.Context, a Address) (Point, error) {
	base := url.Values{}
	if a.Cidade != "" {
		base.Set("city", a.Cidade)
	}
	if a.Estado != "" {
		base.Set("state", a.Estado)
	}
	if len(base) == 0 {
		return Point{}, ErrNotFound // street alone is far too ambiguous
	}

	var attempts []url.Values
	if a.Logradouro != "" {
		if a.Number != "" {
			attempts = append(attempts, with(base, "street", a.Number+" "+a.Logradouro))
		}
		attempts = append(attempts, with(base, "street", a.Logradouro))
	}
	attempts = append(attempts, base)

	for _, q := range attempts {
		p, found, err := n.search(ctx, q)
		if err != nil {
			return Point{}, err
		}
		if found {
			return p, nil
		}
	}
	return Point{}, ErrNotFound
}

func with(base url.Values, key, value string) url.Values {
	q := url.Values{}
	for k, v := range base {
		q[k] = v
	}
	q.Set(key, value)
	return q
}

func (n *Nominatim) search(ctx context.Context, q url.Values) (Point, bool, error) {
	if err := n.waitTurn(ctx); err != nil {
		return Point{}, false, err
	}

	q.Set("format", "jsonv2")
	q.Set("limit", "1")
	q.Set("countrycodes", "br")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, n.baseURL+"/search?"+q.Encode(), nil)
	if err != nil {
		return Point{}, false, err
	}
	req.Header.Set("User-Agent", n.userAgent)
	req.Header.Set("Accept-Language", "pt-BR")

	resp, err := n.client.Do(req)
	if err != nil {
		return Point{}, false, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Point{}, false, fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}

	var results []struct {
		Lat string `json:"lat"`
		Lon string `json:"lon"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return Point{}, false, fmt.Errorf("%w: decode response: %v", ErrUnavailable, err)
	}
	if len(results) == 0 {
		return Point{}, false, nil
	}
	lat, errLat := strconv.ParseFloat(results[0].Lat, 64)
	lon, errLon := strconv.ParseFloat(results[0].Lon, 64)
	if errLat != nil || errLon != nil {
		return Point{}, false, fmt.Errorf("%w: invalid coordinates %q,%q", ErrUnavailable, results[0].Lat, results[0].Lon)
	}
	return Point{Latitude: lat, Longitude: lon}, true, nil
}

// waitTurn spaces requests at least minInterval apart (process-wide).
func (n *Nominatim) waitTurn(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if wait := n.minInterval - time.Since(n.last); wait > 0 {
		t := time.NewTimer(wait)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
	n.last = time.Now()
	return nil
}
