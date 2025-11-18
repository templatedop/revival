package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"revival/internal/store"
)

// Note: Use activity names matching registered Activity methods
func RevivalParentWorkflow(ctx workflow.Context, requestID string) error {

	 ao := workflow.ActivityOptions{
        StartToCloseTimeout: time.Minute * 5,
        RetryPolicy: &temporal.RetryPolicy{
            InitialInterval: time.Second * 2,
            BackoffCoefficient: 2.0,
            MaximumAttempts: 3,
        },
    }
    ctx = workflow.WithActivityOptions(ctx, ao)
	// load request and policy
	var req store.RevivalRequest
	var pol store.Policy

	// Activities to load request and policy
	if err := workflow.ExecuteActivity(ctx, "Activities.LoadRevivalRequest", requestID).Get(ctx, &req); err != nil {
		return err
	}
	if err := workflow.ExecuteActivity(ctx, "Activities.LoadPolicy", req.PolicyNumber).Get(ctx, &pol); err != nil {
		return err
	}

	// 1) Maturity check
	if pol.MaturityDate != nil {
		if workflow.Now(ctx).After(*pol.MaturityDate) {
			workflow.ExecuteActivity(ctx, "Activities.MarkRevivalNotPermitted", requestID, "MaturityReached")
			return nil
		}
	}

	// 2) 5-year check: first unpaid is LastPaidToDate + 1 month
	firstUnpaid := pol.LastPaidToDate.AddDate(0, 1, 0)
	if workflow.Now(ctx).Sub(firstUnpaid).Hours() > (24.0 * 365.0 * 5.0) {
		workflow.ExecuteActivity(ctx, "Activities.MarkRevivalNotPermitted", requestID, "Beyond5Years")
		return nil
	}

	// Data entry & QC
	if err := workflow.ExecuteActivity(ctx, "Activities.PerformDataEntryAndQC", requestID).Get(ctx, nil); err != nil {
		return err
	}

	// Approver (simplified as activity)
	var approval string
	if err := workflow.ExecuteActivity(ctx, "Activities.PerformApproval", requestID).Get(ctx, &approval); err != nil {
		return err
	}
	if approval != "APPROVED" {
		workflow.ExecuteActivity(ctx, "Activities.MarkRequestTerminated", requestID, "Rejected")
		return nil
	}

	// Generate acceptance letter
	workflow.ExecuteActivity(ctx, "Activities.GenerateLetter", requestID, "ACCEPTANCE")

	// Start SLA timer for first installment - 60 days
	sla := workflow.NewTimer(ctx, 60*24*time.Hour)
	firstCh := workflow.GetSignalChannel(ctx, "FirstInstallmentPaid")
	var signalData map[string]interface{}
	var gotFirst bool

	selector := workflow.NewSelector(ctx)
	selector.AddReceive(firstCh, func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, &signalData)
		gotFirst = true
	})
	selector.AddFuture(sla, func(f workflow.Future) {
		// timer expired
	})

	selector.Select(ctx)

	if !gotFirst {
		workflow.ExecuteActivity(ctx, "Activities.MarkRequestTerminated", requestID, "SLAExpired")
		return nil
	}

	// Process first installment
	workflow.ExecuteActivity(ctx, "Activities.ProcessFirstInstallment", requestID, signalData)

	// Generate revival memo, update policy status
	workflow.ExecuteActivity(ctx, "Activities.GenerateLetter", requestID, "REVIVAL_MEMO")
	workflow.ExecuteActivity(ctx, "Activities.UpdatePolicyStatus", req.PolicyNumber, "AP")

	// Start child workflow: remaining installments = req.NoOfInstallments - 1
	childFuture := workflow.ExecuteChildWorkflow(ctx, InstallmentMonitorWorkflow, req.PolicyNumber, requestID, req.NoOfInstallments-1)
	var childResult string
	if err := childFuture.Get(ctx, &childResult); err != nil {
		workflow.ExecuteActivity(ctx, "Activities.MarkRequestTerminated", requestID, "ChildFailure")
		return err
	}

	switch childResult {
	case "SUCCESS":
		workflow.ExecuteActivity(ctx, "Activities.GenerateLetter", requestID, "COMPLETION")
		workflow.ExecuteActivity(ctx, "Activities.MarkRequestTerminated", requestID, "COMPLETED")
	case "DEFAULT":
		workflow.ExecuteActivity(ctx, "Activities.MarkPolicyLapsed", req.PolicyNumber)
		workflow.ExecuteActivity(ctx, "Activities.MoveCollectionsToSuspense", req.PolicyNumber)
	default:
		workflow.ExecuteActivity(ctx, "Activities.MarkRequestTerminated", requestID, childResult)
	}

	return nil
}
