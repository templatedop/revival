package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// InstallmentMonitorWorkflow waits monthly for signals, handles default
func InstallmentMonitorWorkflow(ctx workflow.Context, policyNumber string, requestID string, remainingInstallments int) (string, error) {
	
	 ao := workflow.ActivityOptions{
        StartToCloseTimeout: time.Minute * 5,
        RetryPolicy: &temporal.RetryPolicy{
            InitialInterval: time.Second * 2,
            BackoffCoefficient: 2.0,
            MaximumAttempts: 3,
        },
    }
    ctx = workflow.WithActivityOptions(ctx, ao)
	
	// For each remaining installment, wait for signal "InstallmentPaid" or timer (1 month)
	for i := 1; i <= remainingInstallments; i++ {
		// Simplified: wait 30 days from now (in prod compute next 1st of month)
		dueTimer := workflow.NewTimer(ctx, 30*24*time.Hour)
		installmentCh := workflow.GetSignalChannel(ctx, "InstallmentPaid")
		selector := workflow.NewSelector(ctx)

		var paidPayload map[string]interface{}
		var got bool

		selector.AddReceive(installmentCh, func(c workflow.ReceiveChannel, more bool) {
			c.Receive(ctx, &paidPayload)
			got = true
		})
		selector.AddFuture(dueTimer, func(f workflow.Future) {
			// timer fired
		})

		selector.Select(ctx)

		if !got {
			// default
			workflow.ExecuteActivity(ctx, "Activities.MarkPolicyLapsed", policyNumber)
			workflow.ExecuteActivity(ctx, "Activities.MoveCollectionsToSuspense", policyNumber)
			return "DEFAULT", nil
		}

		// record payment
		workflow.ExecuteActivity(ctx, "Activities.RecordInstallmentPayment", requestID, paidPayload)
	}

	return "SUCCESS", nil
}
