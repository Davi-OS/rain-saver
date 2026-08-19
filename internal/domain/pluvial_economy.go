package domain

import "time"

type PluvialEconomy struct {
	ID          string
	EnderecoID  string
	UserID      string
	AreaM2      float64
	PeriodStart time.Time
	PeriodEnd   time.Time
	CreatedAt   time.Time

	// StationCode and DistanceKm record which weather station fed this
	// calculation and how far it is from the address, for auditability.
	StationCode string
	DistanceKm  float64

	MonthlyCaptation []PluvialMonthlyCaptation
}

type PluvialMonthlyCaptation struct {
	ID               string
	PluvialEconomyID string
	Year             int
	Month            int
	RainfallMM       float64
	CapturedLiters   float64
}
