package workflow

import (
    "go.temporal.io/sdk/workflow"
    "time"
    
)

// Signals expected:
//  - "FirstInstallmentPaid" -> payload contains payment info (receiptID, amount, paidAt)
//  - "WithdrawRequest" etc.

func RevivalParentWorkflow(ctx workflow.Context, requestID string) error {
    // load request + policy (activity)
    var req RevivalRequest
    var pol Policy
    err := workflow.ExecuteActivity(ctx, "LoadRevivalRequest", requestID).Get(ctx, &req)
    if err != nil { return err }
    err = workflow.ExecuteActivity(ctx, "LoadPolicy", req.PolicyNumber).Get(ctx, &pol)
    if err != nil { return err }

    // 1) Pre-check: maturity date
    if pol.MaturityDate != nil {
        if workflow.Now(ctx).After(*pol.MaturityDate) {
            // call activity to mark request as not permitted (maturity)
            _ = workflow.ExecuteActivity(ctx, "MarkRevivalNotPermitted", requestID, "MaturityReached")
            return nil
        }
    }

    // 2) Pre-check: within 5 years from first unpaid premium
    firstUnpaid := pol.LastPaidToDate.AddDate(0, 1, 0)
    diff := workflow.Now(ctx).Sub(firstUnpaid)
    if diff.Hours() > (24.0 * 365.0 * 5.0) {
        _ = workflow.ExecuteActivity(ctx, "MarkRevivalNotPermitted", requestID, "Beyond5Years")
        return nil
    }

    // 3) Indexing/ Data entry/ QC activities (can be parallel or sequential)
    // these are activities that may be human tasks; they can use ContinueAsNew or blocking waits for signals.
    if err := workflow.ExecuteActivity(ctx, "PerformDataEntryAndQC", requestID).Get(ctx, nil); err != nil { return err }

    // 4) Approver review (activity which may return Approved/Rejected)
    var approvalResult string
    if err := workflow.ExecuteActivity(ctx, "PerformApproval", requestID).Get(ctx, &approvalResult); err != nil { return err }
    if approvalResult != "APPROVED" {
        _ = workflow.ExecuteActivity(ctx, "MarkRequestRejected", requestID)
        return nil
    }

    // 5) Generate acceptance letter
    _ = workflow.ExecuteActivity(ctx, "GenerateLetter", requestID, "ACCEPTANCE")

    // 6) Start 60-day SLA timer waiting for FirstInstallmentPaid signal
    sla := workflow.NewTimer(ctx, 60*24*time.Hour)
    firstInstallmentCh := workflow.GetSignalChannel(ctx, "FirstInstallmentPaid")
    var sigPayload map[string]interface{}
    selector := workflow.NewSelector(ctx)
    var gotFirst bool
    selector.AddReceive(firstInstallmentCh, func(c workflow.ReceiveChannel, more bool) {
        c.Receive(ctx, &sigPayload)
        gotFirst = true
    })
    selector.AddFuture(sla, func(f workflow.Future) { /* timer fired */ })
    selector.Select(ctx)

    if !gotFirst {
        // SLA expired
        _ = workflow.ExecuteActivity(ctx, "MarkRequestTerminated", requestID, "SLAExpired")
        return nil
    }

    // 7) Process first installment (activity will persist payment into DB)
    _ = workflow.ExecuteActivity(ctx, "ProcessFirstInstallment", requestID, sigPayload).Get(ctx, nil)
    _ = workflow.ExecuteActivity(ctx, "GenerateLetter", requestID, "REVIVAL_MEMO")
    _ = workflow.ExecuteActivity(ctx, "UpdatePolicyStatus", req.PolicyNumber, "AP")

    // 8) Start child workflow for subsequent installments
    // pass required context (policy number, requestID, installments etc)
    childFuture := workflow.ExecuteChildWorkflow(ctx, InstallmentMonitorWorkflow, req.PolicyNumber, requestID, req.NoOfInstallments-1)
    var childResult string
    if err := childFuture.Get(ctx, &childResult); err != nil {
        // handle child failure
        _ = workflow.ExecuteActivity(ctx, "MarkRequestTerminated", requestID, "ChildFailure")
        return err
    }

    switch childResult {
    case "SUCCESS":
        _ = workflow.ExecuteActivity(ctx, "MarkRequestCompleted", requestID)
    case "DEFAULT":
        _ = workflow.ExecuteActivity(ctx, "MarkPolicyLapsed", req.PolicyNumber)
        _ = workflow.ExecuteActivity(ctx, "MoveCollectionsToSuspense", req.PolicyNumber)
    default:
        _ = workflow.ExecuteActivity(ctx, "MarkRequestTerminated", requestID, childResult)
    }

    return nil
}
