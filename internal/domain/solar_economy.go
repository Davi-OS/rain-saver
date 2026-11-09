package domain

import (
	"math"
	"time"
)

type SolarEconomy struct {
	ID          string
	EnderecoID  string
	UserID      string
	PanelPowerW float64
	PanelCount  int
	SystemKWp   float64
	// LossFactor is the fraction of the nominal output assumed lost
	// (inverter, wiring, heat, dirt), recorded per calculation so old
	// results stay explainable if the default changes.
	LossFactor  float64
	PeriodStart time.Time
	PeriodEnd   time.Time
	CreatedAt   time.Time

	StationCode string
	DistanceKm  float64

	// TariffBRLPerKWh is the electricity price the user entered, nil when
	// they asked for kWh only.
	TariffBRLPerKWh *float64

	MonthlyGeneration []SolarMonthlyGeneration
}

type SolarMonthlyGeneration struct {
	ID             string
	SolarEconomyID string
	Year           int
	Month          int
	AvgDailyKWhM2  float64
	GeneratedKWh   float64
}

// MonthSavingsBRL is what the month's generation is worth at the entered
// tariff, in cents precision. ok is false when no tariff was entered.
func (s *SolarEconomy) MonthSavingsBRL(m SolarMonthlyGeneration) (savings float64, ok bool) {
	if s.TariffBRLPerKWh == nil {
		return 0, false
	}
	return math.Round(m.GeneratedKWh**s.TariffBRLPerKWh*100) / 100, true
}

// TotalSavingsBRL sums the rounded monthly savings, so the total matches the
// table a user sees. ok is false when no tariff was entered.
func (s *SolarEconomy) TotalSavingsBRL() (total float64, ok bool) {
	if s.TariffBRLPerKWh == nil {
		return 0, false
	}
	for _, m := range s.MonthlyGeneration {
		savings, _ := s.MonthSavingsBRL(m)
		total += savings
	}
	return math.Round(total*100) / 100, true
}

// TotalKWh sums the generation over the months available — up to 12, fewer
// if the station's history is shorter.
func (s *SolarEconomy) TotalKWh() float64 {
	var total float64
	for _, m := range s.MonthlyGeneration {
		total += m.GeneratedKWh
	}
	return total
}
