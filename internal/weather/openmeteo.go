package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const archiveAPIURL = "https://archive-api.open-meteo.com/v1/archive"

type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{httpClient: httpClient, baseURL: archiveAPIURL}
}

// WithBaseURL overrides the Open-Meteo endpoint — used by tests to point at
// a local fake server.
func (c *Client) WithBaseURL(baseURL string) *Client {
	c.baseURL = baseURL
	return c
}

type DailyRainfall struct {
	Date      time.Time
	RainSumMM float64
}

type archiveResponse struct {
	Daily struct {
		Time    []string  `json:"time"`
		RainSum []float64 `json:"rain_sum"`
	} `json:"daily"`
}

// FetchDailyRainfall retrieves one year of daily rain_sum ending today for
// the given coordinates — same window and query shape as the old backend's
// integration (start_date = today-1y, end_date = today, daily=rain_sum,
// timezone=GMT).
func (c *Client) FetchDailyRainfall(ctx context.Context, latitude, longitude float64) ([]DailyRainfall, error) {
	end := time.Now().UTC()
	start := end.AddDate(-1, 0, 0)

	q := url.Values{}
	q.Set("latitude", strconv.FormatFloat(latitude, 'f', -1, 64))
	q.Set("longitude", strconv.FormatFloat(longitude, 'f', -1, 64))
	q.Set("start_date", start.Format("2006-01-02"))
	q.Set("end_date", end.Format("2006-01-02"))
	q.Set("daily", "rain_sum")
	q.Set("timezone", "GMT")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather: open-meteo returned status %d", resp.StatusCode)
	}

	var body archiveResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	if len(body.Daily.Time) != len(body.Daily.RainSum) {
		return nil, fmt.Errorf("weather: mismatched time/rain_sum lengths (%d vs %d)", len(body.Daily.Time), len(body.Daily.RainSum))
	}

	out := make([]DailyRainfall, 0, len(body.Daily.Time))
	for i, dateStr := range body.Daily.Time {
		d, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return nil, fmt.Errorf("weather: parse date %q: %w", dateStr, err)
		}
		out = append(out, DailyRainfall{Date: d, RainSumMM: body.Daily.RainSum[i]})
	}
	return out, nil
}
