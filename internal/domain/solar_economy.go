package domain

import "time"

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

// TotalKWh sums the generation over the months available — up to 12, fewer
// if the station's history is shorter.
func (s *SolarEconomy) TotalKWh() float64 {
	var total float64
	for _, m := range s.MonthlyGeneration {
		total += m.GeneratedKWh
	}
	return total
}
