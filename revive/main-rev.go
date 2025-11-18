package revive

import (
	"math"
	"strings"
	"time"
)

type FrequencyChange struct {
	EffectiveFrom time.Time
	Frequency     string
	ModalPremium  float64
}

type Policy struct {
	PolicyNumber     string
	IssueDate        time.Time
	MaturityDate     time.Time
	ProductCode      string
	PremiumFrequency string
	ModalPremium     float64
	LastPaidToDate   time.Time
	FrequencyHistory []FrequencyChange
}

// ---------- Revival Parameters ----------

type RevivalParams struct {
	Policy                Policy
	RevivalIndexDate      time.Time
	UnpaidMonths          int
	InterestMonths        int
	NoOfInstallments      int
	GSTPercent            float64
	MonthlyInterest       float64
	IncludeTaxOnPremium   bool
	IncludeCurrentPremium bool
	RevivalType           string // "INSTALLMENT" or "LUMPSUM"
}

// ---------- Revival Result ----------

type RevivalResult struct {
	TotalUnpaidPremiums    float64
	MonthlyPremium         float64
	RevivalAmount          float64
	InterestComponent      float64
	TotalRevivalAmount     float64
	InstallmentAmount      float64
	TaxOnUnpaidPremiums    float64
	FirstInstallmentAmount float64
	SubsequentInstallment  float64
}

// ---------- Calculation Function ----------

func CalculateRevival(p RevivalParams) RevivalResult {
	// Step 1: Calculate total unpaid premiums
	totalUnpaid := p.Policy.ModalPremium / frequencyMonths(p.Policy.PremiumFrequency) * float64(p.UnpaidMonths)

	// Step 2: Monthly premium normalization
	monthlyPremium := totalUnpaid / float64(p.UnpaidMonths)

	// Step 3: Revival interest calculation (SRS logic)
	i := p.MonthlyInterest
	m := float64(p.InterestMonths)
	n := float64(p.NoOfInstallments)

	revivalAmount := (math.Pow(1+i, m) - 1) * 101 * monthlyPremium
	interestComponent := revivalAmount - (monthlyPremium * m)
	totalRevivalAmount := totalUnpaid + interestComponent

	// Step 4: Installment (EMI formula)
	emiNumerator := math.Pow(1+i, n)
	installment := (totalRevivalAmount * i * emiNumerator) / (emiNumerator - 1)

	// Step 5: Tax calculation
	taxOnUnpaid := totalUnpaid * (p.GSTPercent / 100)

	taxOnModal := p.Policy.ModalPremium * (p.GSTPercent / 100)
	// firstInstallment := installment + taxOnUnpaid + p.Policy.ModalPremium + taxOnModal
	// subsequentInstallment := installment + p.Policy.ModalPremium + taxOnModal

	// Step 6: First & Subsequent Installments
	firstInstallment := installment + taxOnUnpaid
	subsequentInstallment := installment // for this SRS example, no modal premium taxes added

	if strings.ToUpper(p.Policy.PremiumFrequency) == "MONTHLY" {
		firstInstallment += p.Policy.ModalPremium + taxOnModal
		subsequentInstallment += p.Policy.ModalPremium + taxOnModal
	}

	return RevivalResult{
		TotalUnpaidPremiums:    round2(totalUnpaid),
		MonthlyPremium:         round2(monthlyPremium),
		RevivalAmount:          round2(revivalAmount),
		InterestComponent:      round2(interestComponent),
		TotalRevivalAmount:     round2(totalRevivalAmount),
		InstallmentAmount:      round2(installment),
		TaxOnUnpaidPremiums:    round2(taxOnUnpaid),
		FirstInstallmentAmount: round2(firstInstallment),
		SubsequentInstallment:  round2(subsequentInstallment),
	}
}
