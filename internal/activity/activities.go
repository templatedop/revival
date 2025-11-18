package activity

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/activity"

	"revival/internal/calc"
	"revival/internal/store"
)

type Activities struct {
	Store *store.InMemoryStore
}

func NewActivities(s *store.InMemoryStore) *Activities {
	return &Activities{Store: s}
}

// Load policy
func (a *Activities) LoadPolicy(ctx context.Context, policyNumber string) (store.Policy, error) {
	// simple passthrough
	p, err := a.Store.GetPolicy(policyNumber)
	if err != nil {
		return store.Policy{}, err
	}
	return p, nil
}

// Load request
func (a *Activities) LoadRevivalRequest(ctx context.Context, requestID string) (store.RevivalRequest, error) {
	r, err := a.Store.GetRequest(requestID)
	if err != nil {
		return store.RevivalRequest{}, err
	}
	return r, nil
}

// Mark request not permitted (maturity/beyond5years)
func (a *Activities) MarkRevivalNotPermitted(ctx context.Context, requestID string, reason string) error {
	_ = a.Store.UpdateRequestStatus(requestID, "NOT_PERMITTED")
	activity.GetLogger(ctx).Info("Marked not permitted", "request", requestID, "reason", reason)
	return nil
}

// PerformDataEntryAndQC - placeholder
func (a *Activities) PerformDataEntryAndQC(ctx context.Context, requestID string) error {
	activity.GetLogger(ctx).Info("Data entry and QC done", "request", requestID)
	// for prototype, assume done
	return nil
}

// PerformApproval - simulate approval (in real system this is human)
func (a *Activities) PerformApproval(ctx context.Context, requestID string) (string, error) {
	activity.GetLogger(ctx).Info("Approver auto-approved for prototype", "request", requestID)
	_ = a.Store.UpdateRequestStatus(requestID, "APPROVED")
	return "APPROVED", nil
}

func (a *Activities) GenerateLetter(ctx context.Context, requestID string, template string) error {
	activity.GetLogger(ctx).Info("Generated letter", "request", requestID, "template", template)
	return nil
}

// ProcessFirstInstallment - records payment and verifies amount using calc package
func (a *Activities) ProcessFirstInstallment(ctx context.Context, requestID string, payload map[string]interface{}) error {
	// payload should contain: amount(float64), receiptID(optional), paidAt(optional)
	amount, _ := payload["amount"].(float64)
	receipt, _ := payload["receipt"].(string)
	if receipt == "" {
		receipt = uuid.NewString()
	}

	r, err := a.Store.GetRequest(requestID)
	if err != nil {
		return err
	}
	pol, err := a.Store.GetPolicy(r.PolicyNumber)
	if err != nil {
		return err
	}

	// run calculation to compute expected first installment
	res := calc.CalculateRevival(pol.ModalPremium, r.UnpaidMonths, r.InterestMonths, r.NoOfInstallments, r.GSTPercent, r.MonthlyInterest, pol.PremiumFrequency, true)

	// persist payment (prototype: accept any amount but log mismatch)
	a.Store.RecordPayment(store.Payment{
		RequestID: requestID,
		Amount:    amount,
		When:      time.Now(),
		ReceiptID: receipt,
	})

	if !almostEqual(amount, res.FirstInstallmentAmount) {
		activity.GetLogger(ctx).Warn("First installment amount differs from expected", "expected", res.FirstInstallmentAmount, "received", amount)
	} else {
		activity.GetLogger(ctx).Info("First installment matches expected", "amount", amount)
	}

	return nil
}

func (a *Activities) RecordInstallmentPayment(ctx context.Context, requestID string, payload map[string]interface{}) error {
	amount, _ := payload["amount"].(float64)
	receipt, _ := payload["receipt"].(string)
	if receipt == "" {
		receipt = uuid.NewString()
	}
	a.Store.RecordPayment(store.Payment{RequestID: requestID, Amount: amount, When: time.Now(), ReceiptID: receipt})
	activity.GetLogger(ctx).Info("Recorded installment payment", "request", requestID, "amount", amount)
	return nil
}

func (a *Activities) MarkPolicyLapsed(ctx context.Context, policyNumber string) error {
	activity.GetLogger(ctx).Info("Marking policy lapsed", "policy", policyNumber)
	// In prototype, don't change policy struct; in real system update DB
	return nil
}

func (a *Activities) MoveCollectionsToSuspense(ctx context.Context, policyNumber string) error {
	activity.GetLogger(ctx).Info("Moved collections to suspense", "policy", policyNumber)
	return nil
}

func (a *Activities) MarkRequestTerminated(ctx context.Context, requestID string, reason string) error {
	activity.GetLogger(ctx).Info("Mark request terminated", "request", requestID, "reason", reason)
	_ = a.Store.UpdateRequestStatus(requestID, "TERMINATED")
	return nil
}

func (a *Activities) UpdatePolicyStatus(ctx context.Context, policyNumber string, status string) error {
	activity.GetLogger(ctx).Info("Updating policy status", "policy", policyNumber, "status", status)
	// In prototype, don't change policy struct; in real system update DB
	// For now, just log it
	return nil
}

func (a *Activities) MarkRequestCompleted(ctx context.Context, requestID string) error {
	activity.GetLogger(ctx).Info("Mark request completed", "request", requestID)
	_ = a.Store.UpdateRequestStatus(requestID, "COMPLETED")
	return nil
}

// ProcessRefund handles refund of amounts when revival is not approved or death before approval
// Implements Sankalan Rule 58(3): Amount refunded (with interest) if revival not approved
func (a *Activities) ProcessRefund(ctx context.Context, requestID string, reason string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Processing refund for revival request", "request", requestID, "reason", reason)

	// Get all payments made for this request
	payments := a.Store.GetPaymentsByRequest(requestID)
	if len(payments) == 0 {
		logger.Info("No payments to refund")
		return nil
	}

	// Calculate total amount to refund
	var totalAmount float64
	for _, payment := range payments {
		totalAmount += payment.Amount
		logger.Info("Payment to be refunded",
			"receiptID", payment.ReceiptID,
			"amount", payment.Amount,
			"paidAt", payment.When)
	}

	// In production:
	// 1. Calculate interest on refund amount
	// 2. Create refund transaction in accounting system
	// 3. Update suspense accounts
	// 4. Generate refund letter/advice
	// 5. Process actual refund through payment gateway

	logger.Info("Refund processed successfully",
		"totalAmount", totalAmount,
		"paymentCount", len(payments),
		"reason", reason)

	// For prototype, just log the refund
	return nil
}

// IncrementRevivalCount increments the revival count for a policy after successful revival
// Tracks revival history per SRS IR_29 (max 2 revivals)
func (a *Activities) IncrementRevivalCount(ctx context.Context, policyNumber string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Incrementing revival count", "policy", policyNumber)

	// In production: UPDATE policies SET revival_count = revival_count + 1 WHERE policy_number = ?
	// For prototype, just log
	return nil
}

// helper
func almostEqual(a, b float64) bool {
	if a == b {
		return true
	}
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < 0.01
}
