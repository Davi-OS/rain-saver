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
