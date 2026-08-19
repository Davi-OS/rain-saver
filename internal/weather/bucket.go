package weather

import "sort"

type MonthlyRainfall struct {
	Year       int
	Month      int
	RainfallMM float64
}

// BucketByCalendarMonth sums RainSumMM per actual (year, month) pair taken
// from each day's real date. The old backend's equivalent logic sliced the
// flat daily list into 4 equal-length chunks without ever looking at the
// dates, so a leap year, a month with 28 vs 31 days, or a request issued
// mid-month would silently skew which days landed in which "quarter".
// Bucketing by the actual calendar month avoids that class of bug entirely.
//
// Because the request window is "today minus 1 year", the result can
// contain up to 13 distinct (year, month) buckets when the window's start
// and end land in the same calendar month a year apart — the boundary
// month appears twice, once per year, each with only its partial days
// summed.
func BucketByCalendarMonth(daily []DailyRainfall) []MonthlyRainfall {
	type key struct{ year, month int }
	sums := make(map[key]float64)
	for _, d := range daily {
		k := key{d.Date.Year(), int(d.Date.Month())}
		sums[k] += d.RainSumMM
	}

	out := make([]MonthlyRainfall, 0, len(sums))
	for k, sum := range sums {
		out = append(out, MonthlyRainfall{Year: k.year, Month: k.month, RainfallMM: sum})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Year != out[j].Year {
			return out[i].Year < out[j].Year
		}
		return out[i].Month < out[j].Month
	})
	return out
}
