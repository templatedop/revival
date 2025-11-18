package main

import (
	"fmt"
	"math"
	"time"
)

// ---------- Policy Metadata ----------

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

	// Step 6: First & Subsequent Installments
	firstInstallment := installment + taxOnUnpaid
	subsequentInstallment := installment // for this SRS example, no modal premium taxes added

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

// ---------- Helper Functions ----------

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

// ---------- Example Run (SRS Case) ----------

func main() {
	policy := Policy{
		PolicyNumber:     "PLI12345",
		IssueDate:        time.Date(2014, 1, 30, 0, 0, 0, 0, time.UTC),
		MaturityDate:     time.Date(2034, 1, 30, 0, 0, 0, 0, time.UTC),
		ProductCode:      "CWLA",
		PremiumFrequency: "SEMIANNUAL",
		ModalPremium:     5852,
		LastPaidToDate:   time.Date(2015, 8, 31, 0, 0, 0, 0, time.UTC),
		FrequencyHistory: []FrequencyChange{
			{EffectiveFrom: time.Date(2014, 12, 1, 0, 0, 0, 0, time.UTC), Frequency: "QUARTERLY", ModalPremium: 2966},
			{EffectiveFrom: time.Date(2015, 3, 1, 0, 0, 0, 0, time.UTC), Frequency: "SEMIANNUAL", ModalPremium: 5852},
		},
	}

	params := RevivalParams{
		Policy:              policy,
		RevivalIndexDate:    time.Date(2018, 7, 31, 0, 0, 0, 0, time.UTC),
		UnpaidMonths:        36,
		InterestMonths:      34,
		NoOfInstallments:    5,
		GSTPercent:          2.25,
		MonthlyInterest:     0.01,
		IncludeTaxOnPremium: true,
		RevivalType:         "INSTALLMENT",
	}

	result := CalculateRevival(params)

	fmt.Println("========== Revival Calculation (SRS Case) ==========")
	fmt.Printf("Total Unpaid Premiums      : ₹%.2f\n", result.TotalUnpaidPremiums)
	fmt.Printf("Monthly Premium            : ₹%.2f\n", result.MonthlyPremium)
	fmt.Printf("Revival Amount (Interest)  : ₹%.2f\n", result.RevivalAmount)
	fmt.Printf("Interest Component          : ₹%.2f\n", result.InterestComponent)
	fmt.Printf("Total Revival Amount       : ₹%.2f\n", result.TotalRevivalAmount)
	fmt.Printf("Installment Amount         : ₹%.2f\n", result.InstallmentAmount)
	fmt.Printf("Tax on Unpaid Premiums     : ₹%.2f\n", result.TaxOnUnpaidPremiums)
	fmt.Printf("First Installment Due      : ₹%.2f\n", result.FirstInstallmentAmount)
	fmt.Printf("Subsequent Installment     : ₹%.2f\n", result.SubsequentInstallment)
}
