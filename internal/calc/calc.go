package calc

import "math"

// Result holds outputs of calculation
type Result struct {
	TotalUnpaidPremiums    float64
	MonthlyPremium         float64
	RevivalAmount          float64
	InterestComponent      float64
	TotalRevivalAmount     float64
	InstallmentAmount      float64
	TaxOnUnpaidPremiums    float64
	TaxOnModalPremium      float64
	FirstInstallmentAmount float64
	SubsequentInstallment  float64
}

func Round2(v float64) float64 { return math.Round(v*100) / 100 }

func frequencyMonths(freq string) float64 {
	switch freq {
	case "MONTHLY":
		return 1
	case "QUARTERLY":
		return 3
	case "SEMIANNUAL", "HALFYEARLY":
		return 6
	case "YEARLY":
		return 12
	default:
		return 1
	}
}

// CalculateRevival implements SRS formulas (IR_5, IR_6).
// modalPremium: modal premium (per frequency). unpaidMonths: total unpaid months (e.g. 36).
// interestMonths: months for interest (e.g. 34). noOfInstallments: n.
func CalculateRevival(modalPremium float64, unpaidMonths int, interestMonths int, noOfInstallments int, gstPercent float64, monthlyInterest float64, freq string, includeCurrentPremium bool) Result {
	totalUnpaid := modalPremium / frequencyMonths(freq) * float64(unpaidMonths)
	monthlyPremium := totalUnpaid / float64(unpaidMonths)

	i := monthlyInterest
	m := float64(interestMonths)
	n := float64(noOfInstallments)

	revivalAmount := (math.Pow(1+i, m) - 1) * 101 * monthlyPremium
	interestComponent := revivalAmount - (monthlyPremium * m)
	totalRevivalAmount := totalUnpaid + interestComponent

	emiNumerator := math.Pow(1+i, n)
	installment := (totalRevivalAmount * i * emiNumerator) / (emiNumerator - 1)

	taxOnUnpaid := totalUnpaid * (gstPercent / 100)
	taxOnModal := modalPremium * (gstPercent / 100)

	first := installment + taxOnUnpaid
	subsequent := installment

	if freq == "MONTHLY" && includeCurrentPremium {
		first += modalPremium + taxOnModal
		subsequent += modalPremium + taxOnModal
	}

	return Result{
		TotalUnpaidPremiums:    Round2(totalUnpaid),
		MonthlyPremium:         Round2(monthlyPremium),
		RevivalAmount:          Round2(revivalAmount),
		InterestComponent:      Round2(interestComponent),
		TotalRevivalAmount:     Round2(totalRevivalAmount),
		InstallmentAmount:      Round2(installment),
		TaxOnUnpaidPremiums:    Round2(taxOnUnpaid),
		TaxOnModalPremium:      Round2(taxOnModal),
		FirstInstallmentAmount: Round2(first),
		SubsequentInstallment:  Round2(subsequent),
	}
}
