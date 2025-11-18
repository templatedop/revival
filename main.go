package main

import (
	"fmt"
	"revival/revive"
	"time"
)

func main() {

// 	policy := revive.Policy{
//     PolicyNumber:     "PLI00001",
//     IssueDate:        time.Date(2014, 1, 30, 0, 0, 0, 0, time.UTC),
//     MaturityDate:     time.Date(2034, 1, 30, 0, 0, 0, 0, time.UTC),
//     ProductCode:      "WLA",
//     PremiumFrequency: "MONTHLY",
//     ModalPremium:     500,  // same as monthly premium
//     LastPaidToDate:   time.Date(2017, 11, 30, 0, 0, 0, 0, time.UTC),
// }

// params := revive.RevivalParams{
//     Policy:               policy,
//     RevivalIndexDate:     time.Date(2018, 11, 30, 0, 0, 0, 0, time.UTC),
//     UnpaidMonths:         11,      // 11 months unpaid
//     InterestMonths:       11,      // same, since “current month – 1” means full months here
//     NoOfInstallments:     5,
//     GSTPercent:           2.25,
//     MonthlyInterest:      0.01,
//     IncludeTaxOnPremium:  true,
//     IncludeCurrentPremium: true,
//     RevivalType:          "INSTALLMENT",
// }







	policy := revive.Policy{
		PolicyNumber:     "PLI12345",
		IssueDate:        time.Date(2014, 1, 30, 0, 0, 0, 0, time.UTC),
		MaturityDate:     time.Date(2034, 1, 30, 0, 0, 0, 0, time.UTC),
		ProductCode:      "CWLA",
		PremiumFrequency: "SEMIANNUAL",
		ModalPremium:     5852,
		LastPaidToDate:   time.Date(2015, 8, 31, 0, 0, 0, 0, time.UTC),
		FrequencyHistory: []revive.FrequencyChange{
			{EffectiveFrom: time.Date(2014, 12, 1, 0, 0, 0, 0, time.UTC), Frequency: "QUARTERLY", ModalPremium: 2966},
			{EffectiveFrom: time.Date(2015, 3, 1, 0, 0, 0, 0, time.UTC), Frequency: "SEMIANNUAL", ModalPremium: 5852},
		},
	}

	params := revive.RevivalParams{
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

	result := revive.CalculateRevival(params)

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
