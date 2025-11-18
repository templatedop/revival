package workflow

import (
    "go.temporal.io/sdk/workflow"
    "time"
)

func InstallmentMonitorWorkflow(ctx workflow.Context, policyNumber string, requestID string, remainingInstallments int) (string, error) {
    // Feed from DB: due dates are 1st of each month after first payment
    // Loop over remainingInstallments
    for i := 1; i <= remainingInstallments; i++ {
        // compute next due date or use 1 month timer
        dueTimer := workflow.NewTimer(ctx, 30*24*time.Hour) // simplified - compute actual next 1st-of-month in prod

        installmentCh := workflow.GetSignalChannel(ctx, "InstallmentPaid")
        selector := workflow.NewSelector(ctx)
        var paidPayload map[string]interface{}
        var paid bool

        selector.AddReceive(installmentCh, func(c workflow.ReceiveChannel, more bool){
            c.Receive(ctx, &paidPayload)
            paid = true
        })
        selector.AddFuture(dueTimer, func(f workflow.Future){ /* timer fired */ })
        selector.Select(ctx)

        if !paid {
            // default - call activity to mark policy AL and move amounts to suspense
            _ = workflow.ExecuteActivity(ctx, "MarkPolicyLapsed", policyNumber)
            _ = workflow.ExecuteActivity(ctx, "MoveCollectionsToSuspense", policyNumber)
            return "DEFAULT", nil
        }

        // record payment in DB via activity
        _ = workflow.ExecuteActivity(ctx, "RecordInstallmentPayment", requestID, paidPayload).Get(ctx, nil)
        // continue to next installment
    }

    return "SUCCESS", nil
}
