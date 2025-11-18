package revive

import "math"

// frequencyMonths converts frequency to number of months.
func frequencyMonths(freq string) float64 {
	switch freq {
	case "MONTHLY":
		return 1
	case "QUARTERLY":
		return 3
	case "HALFYEARLY", "SEMIANNUAL":
		return 6
	case "YEARLY":
		return 12
	default:
		return 1
	}
}

func round2(val float64) float64 {
	return math.Round(val*100) / 100
}
