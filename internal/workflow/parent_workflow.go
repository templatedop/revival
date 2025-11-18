package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"revival/internal/store"
)

// RevivalParentWorkflow implements the parent workflow as per flow.mmd
// This workflow handles the complete revival request lifecycle from indexing to completion
func RevivalParentWorkflow(ctx workflow.Context, requestID string) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting Revival Parent Workflow", "requestID", requestID)

	// Configure activity options
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 5,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second * 2,
			BackoffCoefficient: 2.0,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// ========== STEP 1: Load Revival Request and Policy ==========
	// Index Revival Request (IR_2) - Load data
	var req store.RevivalRequest
	var pol store.Policy

	if err := workflow.ExecuteActivity(ctx, "LoadRevivalRequest", requestID).Get(ctx, &req); err != nil {
		logger.Error("Failed to load revival request", "error", err)
		return err
	}

	if err := workflow.ExecuteActivity(ctx, "LoadPolicy", req.PolicyNumber).Get(ctx, &pol); err != nil {
		logger.Error("Failed to load policy", "error", err)
		return err
	}

	logger.Info("Loaded request and policy", "policyNumber", req.PolicyNumber)

	// ========== STEP 2: Pre-checks - Maturity Date (Rule 58) ==========
	// Check: Has Policy Reached Maturity Date?
	if pol.MaturityDate != nil && workflow.Now(ctx).After(*pol.MaturityDate) {
		logger.Info("Revival not permitted - policy has reached maturity date")
		workflow.ExecuteActivity(ctx, "MarkRevivalNotPermitted", requestID, "MaturityReached")
		return nil // END: Revival Not Permitted (Maturity Reached)
	}

	// ========== STEP 3: Pre-checks - 5 Years from First Unpaid Premium (Rule 58(1)) ==========
	// Check: Within 5 Years from First Unpaid Premium?
	firstUnpaidDate := pol.LastPaidToDate.AddDate(0, 1, 0) // First unpaid is 1 month after last paid
	fiveYearsLimit := firstUnpaidDate.AddDate(5, 0, 0)

	if workflow.Now(ctx).After(fiveYearsLimit) {
		logger.Info("Revival not permitted - beyond 5 years from first unpaid premium")
		workflow.ExecuteActivity(ctx, "MarkRevivalNotPermitted", requestID, "Beyond5Years")
		return nil // END: Revival Not Permitted (Beyond 5 Years)
	}

	// ========== STEP 3A: Check Maximum Revivals (IR_29) ==========
	// SRS Rule IR_29: Maximum 2 revivals allowed per policy
	if pol.RevivalCount >= 2 {
		logger.Info("Revival not permitted - maximum 2 revivals already exhausted", "revivalCount", pol.RevivalCount)
		workflow.ExecuteActivity(ctx, "MarkRevivalNotPermitted", requestID, "MaxRevivalsExceeded")
		return nil // END: Revival Not Permitted (Max 2 Revivals)
	}

	// ========== STEP 3B: Validate Installment Count (IR_3, IR_4) ==========
	// SRS Rule IR_3: Minimum 2 installments
	// SRS Rule IR_4: Maximum 12 installments
	if req.NoOfInstallments < 2 {
		logger.Error("Invalid installment count - minimum is 2", "requested", req.NoOfInstallments)
		workflow.ExecuteActivity(ctx, "MarkRevivalNotPermitted", requestID, "InstallmentsBelowMinimum")
		return nil
	}
	if req.NoOfInstallments > 12 {
		logger.Error("Invalid installment count - maximum is 12", "requested", req.NoOfInstallments)
		workflow.ExecuteActivity(ctx, "MarkRevivalNotPermitted", requestID, "InstallmentsExceedMaximum")
		return nil
	}

	logger.Info("Pre-checks passed",
		"revivalCount", pol.RevivalCount,
		"requestedInstallments", req.NoOfInstallments)

	// ========== STEP 4: Data Entry and QC Verification ==========
	logger.Info("Performing data entry and QC verification")
	if err := workflow.ExecuteActivity(ctx, "PerformDataEntryAndQC", requestID).Get(ctx, nil); err != nil {
		logger.Error("Data entry and QC failed", "error", err)
		return err
	}

	// ========== STEP 5: Approver Review ==========
	logger.Info("Sending request for approval")
	var approvalResult string
	if err := workflow.ExecuteActivity(ctx, "PerformApproval", requestID).Get(ctx, &approvalResult); err != nil {
		logger.Error("Approval activity failed", "error", err)
		return err
	}

	// Handle approval decision
	if approvalResult == "REJECTED" {
		logger.Info("Request rejected by approver")
		workflow.ExecuteActivity(ctx, "GenerateLetter", requestID, "REJECTION")
		workflow.ExecuteActivity(ctx, "MarkRequestTerminated", requestID, "Rejected")
		// Sankalan Rule 58(3): Refund amount if not approved
		workflow.ExecuteActivity(ctx, "ProcessRefund", requestID, "Rejected")
		return nil // END: Request Rejected
	} else if approvalResult == "WITHDRAWN" {
		logger.Info("Request withdrawn")
		workflow.ExecuteActivity(ctx, "MarkRequestTerminated", requestID, "Withdrawn")
		// Sankalan Rule 58(3): Refund amount if withdrawn
		workflow.ExecuteActivity(ctx, "ProcessRefund", requestID, "Withdrawn")
		return nil // END: Request Withdrawn (IR_35)
	} else if approvalResult != "APPROVED" {
		logger.Warn("Unexpected approval result", "result", approvalResult)
		workflow.ExecuteActivity(ctx, "MarkRequestTerminated", requestID, "UnexpectedApprovalResult")
		workflow.ExecuteActivity(ctx, "ProcessRefund", requestID, "UnexpectedApprovalResult")
		return nil
	}

	// ========== STEP 6: Generate Acceptance Letter (IR_25) ==========
	logger.Info("Request approved - generating acceptance letter")
	workflow.ExecuteActivity(ctx, "GenerateLetter", requestID, "ACCEPTANCE")

	// ========== STEP 7: Start 60-Day SLA Timer (IR_10) ==========
	// Wait for FirstInstallmentPaid signal with 60-day timeout
	logger.Info("Starting 60-day SLA timer for first installment payment")
	slaTimer := workflow.NewTimer(ctx, 60*24*time.Hour)
	firstInstallmentChannel := workflow.GetSignalChannel(ctx, "FirstInstallmentPaid")

	var firstInstallmentPayload map[string]interface{}
	var receivedFirstInstallment bool

	selector := workflow.NewSelector(ctx)

	// Add receiver for FirstInstallmentPaid signal
	selector.AddReceive(firstInstallmentChannel, func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, &firstInstallmentPayload)
		receivedFirstInstallment = true
		logger.Info("Received FirstInstallmentPaid signal")
	})

	// Add timer expiry handler
	selector.AddFuture(slaTimer, func(f workflow.Future) {
		logger.Info("60-day SLA timer expired")
	})

	// Wait for either signal or timer
	selector.Select(ctx)

	// Check if timer expired without receiving payment
	if !receivedFirstInstallment {
		logger.Info("Request terminated - SLA expired without payment")
		workflow.ExecuteActivity(ctx, "MarkRequestTerminated", requestID, "SLAExpired")
		// Sankalan Rule 58(3): Refund any advance payments if SLA expired
		workflow.ExecuteActivity(ctx, "ProcessRefund", requestID, "SLAExpired")
		return nil // END: Request Terminated (SLA Expired)
	}

	// ========== STEP 8: Process First Installment Payment ==========
	logger.Info("Processing first installment payment")
	if err := workflow.ExecuteActivity(ctx, "ProcessFirstInstallment", requestID, firstInstallmentPayload).Get(ctx, nil); err != nil {
		logger.Error("Failed to process first installment", "error", err)
		return err
	}

	// ========== STEP 9: Generate Revival Memo (IR_25) ==========
	logger.Info("Generating revival memo")
	workflow.ExecuteActivity(ctx, "GenerateLetter", requestID, "REVIVAL_MEMO")

	// ========== STEP 10: Update Policy Status to AP (IR_13) ==========
	logger.Info("Updating policy status to AP (Active Premium)")
	workflow.ExecuteActivity(ctx, "UpdatePolicyStatus", req.PolicyNumber, "AP")

	// ========== STEP 11: Start Child Workflow - Installment Monitor ==========
	// Calculate remaining installments (total - 1 for first already paid)
	remainingInstallments := req.NoOfInstallments - 1
	logger.Info("Starting child workflow for installment monitoring", "remainingInstallments", remainingInstallments)

	// Configure child workflow options
	childOptions := workflow.ChildWorkflowOptions{
		WorkflowID: "installment-monitor-" + requestID,
		TaskQueue:  "revival-task-queue",
	}
	childCtx := workflow.WithChildOptions(ctx, childOptions)

	// Execute child workflow
	childFuture := workflow.ExecuteChildWorkflow(childCtx, InstallmentMonitorWorkflow, req.PolicyNumber, requestID, remainingInstallments)

	var childResult string
	if err := childFuture.Get(ctx, &childResult); err != nil {
		logger.Error("Child workflow failed", "error", err)
		workflow.ExecuteActivity(ctx, "MarkRequestTerminated", requestID, "ChildWorkflowFailed")
		return err
	}

	// ========== STEP 12: Handle Child Workflow Result ==========
	logger.Info("Child workflow completed", "result", childResult)

	switch childResult {
	case "SUCCESS":
		// All installments paid successfully (IR_15)
		logger.Info("Revival completed successfully - all installments paid")
		workflow.ExecuteActivity(ctx, "GenerateLetter", requestID, "COMPLETION")
		workflow.ExecuteActivity(ctx, "MarkRequestCompleted", requestID)
		// SRS IR_29: Increment revival count (max 2 allowed)
		workflow.ExecuteActivity(ctx, "IncrementRevivalCount", req.PolicyNumber)
		logger.Info("Revival workflow completed successfully", "policyNumber", req.PolicyNumber)
		return nil // END: Revival Successful

	case "DEFAULT":
		// Installment payment defaulted (IR_16)
		logger.Info("Installment payment defaulted - marking policy as lapsed")
		workflow.ExecuteActivity(ctx, "MarkPolicyLapsed", req.PolicyNumber)
		workflow.ExecuteActivity(ctx, "MoveCollectionsToSuspense", req.PolicyNumber)
		return nil // END: Policy Lapsed (moved to suspense IR_24)

	default:
		// Unexpected result or termination
		logger.Warn("Child workflow returned unexpected result", "result", childResult)
		workflow.ExecuteActivity(ctx, "MarkRequestTerminated", requestID, childResult)
		return nil // END: Request Terminated
	}
}
