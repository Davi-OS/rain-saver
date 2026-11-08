package domain

import "time"

type WeatherStation struct {
	Code      string
	Name      string
	Latitude  float64
	Longitude float64
	UF        string
	CreatedAt time.Time
}

// StationMonthlyRainfall is a row already compiled by the (future) INMET
// ingestion lambda — one value per calendar month, per station. rain-saver
// never aggregates raw daily readings itself.
type StationMonthlyRainfall struct {
	StationCode string
	Year        int
	Month       int
	RainfallMM  float64
}

// StationMonthlySolar is a month's average daily global horizontal
// irradiation in kWh/m²/day — numerically the month's peak sun hours —
// compiled by the INMET ingestor from hourly readings.
type StationMonthlySolar struct {
	StationCode   string
	Year          int
	Month         int
	AvgDailyKWhM2 float64
}
