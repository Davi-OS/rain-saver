package domain

import "testing"

func TestSolarEconomy_SavingsNeedATariff(t *testing.T) {
	se := &SolarEconomy{MonthlyGeneration: []SolarMonthlyGeneration{{GeneratedKWh: 100}}}

	if _, ok := se.MonthSavingsBRL(se.MonthlyGeneration[0]); ok {
		t.Error("MonthSavingsBRL() ok without a tariff")
	}
	if _, ok := se.TotalSavingsBRL(); ok {
		t.Error("TotalSavingsBRL() ok without a tariff")
	}
}

func TestSolarEconomy_SavingsAreRoundedPerMonthAndSummed(t *testing.T) {
	tariff := 0.95
	se := &SolarEconomy{
		TariffBRLPerKWh: &tariff,
		MonthlyGeneration: []SolarMonthlyGeneration{
			{GeneratedKWh: 330.37}, // 313.8515 -> 313.85
			{GeneratedKWh: 315.12}, // 299.364  -> 299.36
		},
	}

	if got, _ := se.MonthSavingsBRL(se.MonthlyGeneration[0]); got != 313.85 {
		t.Errorf("MonthSavingsBRL() = %v, want 313.85", got)
	}
	// Sum of the rounded months, not round(total × tariff) = 613.22.
	if got, _ := se.TotalSavingsBRL(); got != 613.21 {
		t.Errorf("TotalSavingsBRL() = %v, want 613.21", got)
	}
}
