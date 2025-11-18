// internal/types.go
package workflow

import "time"

type Policy struct {
    PolicyNumber     string
    IssueDate        time.Time
    MaturityDate     *time.Time
    PremiumFrequency string
    ModalPremium     float64
    LastPaidToDate   time.Time
    RevivalCount     int
}

type RevivalRequest struct {
    RequestID        string
    PolicyNumber     string
    IndexedBy        string
    IndexedAt        time.Time
    ApprovedBy       string
    ApprovedAt       *time.Time
    NoOfInstallments int
    GSTPercent       float64
    MonthlyInterest  float64
    InterestMonths   int
    UnpaidMonths     int
    Status           string // "PENDING","APPROVED","TERMINATED","REJECTED","COMPLETED"
}
