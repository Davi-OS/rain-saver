package weather

import (
	"testing"
	"time"
)

func day(dateStr string, rain float64) DailyRainfall {
	d, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		panic(err)
	}
	return DailyRainfall{Date: d, RainSumMM: rain}
}

func TestBucketByCalendarMonth_GroupsByActualDate(t *testing.T) {
	// A naive "slice into 4 equal chunks" bucketing (the bug in the old
	// backend this replaces) would split these 10 days into two groups of
	// 5 without ever looking at the month boundary landing on day 3.
	daily := []DailyRainfall{
		day("2026-01-29", 1),
		day("2026-01-30", 2),
		day("2026-01-31", 3),
		day("2026-02-01", 10),
		day("2026-02-02", 20),
		day("2026-02-03", 30),
		day("2026-02-04", 40),
		day("2026-02-05", 50),
		day("2026-02-06", 60),
		day("2026-02-07", 70),
	}

	got := BucketByCalendarMonth(daily)

	want := []MonthlyRainfall{
		{Year: 2026, Month: 1, RainfallMM: 6},
		{Year: 2026, Month: 2, RainfallMM: 280},
	}

	if len(got) != len(want) {
		t.Fatalf("got %d buckets, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("bucket[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestBucketByCalendarMonth_LeapYearFebruary(t *testing.T) {
	daily := []DailyRainfall{
		day("2024-02-28", 1),
		day("2024-02-29", 2), // leap day
		day("2024-03-01", 3),
	}

	got := BucketByCalendarMonth(daily)

	if len(got) != 2 {
		t.Fatalf("got %d buckets, want 2: %+v", len(got), got)
	}
	if got[0] != (MonthlyRainfall{Year: 2024, Month: 2, RainfallMM: 3}) {
		t.Errorf("february bucket = %+v, want {2024 2 3}", got[0])
	}
	if got[1] != (MonthlyRainfall{Year: 2024, Month: 3, RainfallMM: 3}) {
		t.Errorf("march bucket = %+v, want {2024 3 3}", got[1])
	}
}

func TestBucketByCalendarMonth_BoundaryMonthSpansTwoYears(t *testing.T) {
	// A one-year-back window can revisit the same calendar month a year
	// apart (e.g. Aug 2025 and Aug 2026) — each occurrence must be its own
	// bucket, not merged.
	daily := []DailyRainfall{
		day("2025-08-15", 5),
		day("2026-08-01", 7),
	}

	got := BucketByCalendarMonth(daily)

	want := []MonthlyRainfall{
		{Year: 2025, Month: 8, RainfallMM: 5},
		{Year: 2026, Month: 8, RainfallMM: 7},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d buckets, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("bucket[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestBucketByCalendarMonth_Empty(t *testing.T) {
	got := BucketByCalendarMonth(nil)
	if len(got) != 0 {
		t.Errorf("got %d buckets, want 0", len(got))
	}
}
