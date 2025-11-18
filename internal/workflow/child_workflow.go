package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// InstallmentMonitorWorkflow implements the child workflow as per flow.mmd
// This workflow monitors monthly installment payments and handles defaults
// Returns: "SUCCESS" if all installments paid, "DEFAULT" if payment missed
func InstallmentMonitorWorkflow(ctx workflow.Context, policyNumber string, requestID string, remainingInstallments int) (string, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting Installment Monitor Child Workflow",
		"policyNumber", policyNumber,
		"requestID", requestID,
		"remainingInstallments", remainingInstallments)

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

	// ========== Monitor Each Remaining Installment ==========
	// Loop through each installment that needs to be paid
	for installmentNumber := 1; installmentNumber <= remainingInstallments; installmentNumber++ {
		logger.Info("Waiting for installment payment",
			"installmentNumber", installmentNumber,
			"totalRemaining", remainingInstallments)

		// ========== Wait for Next Installment Due Date ==========
		// Timer for 1st of next month (simplified to 30 days for prototype)
		// In production, calculate exact date to 1st of next month
		dueDate := 30 * 24 * time.Hour
		logger.Info("Setting timer for next installment due date", "duration", dueDate)

		installmentTimer := workflow.NewTimer(ctx, dueDate)
		installmentChannel := workflow.GetSignalChannel(ctx, "InstallmentPaid")

		var installmentPayload map[string]interface{}
		var paymentReceived bool

		selector := workflow.NewSelector(ctx)

		// Add receiver for InstallmentPaid signal
		selector.AddReceive(installmentChannel, func(c workflow.ReceiveChannel, more bool) {
			c.Receive(ctx, &installmentPayload)
			paymentReceived = true
			logger.Info("Received InstallmentPaid signal",
				"installmentNumber", installmentNumber,
				"payload", installmentPayload)
		})

		// Add timer expiry handler (due date reached)
		selector.AddFuture(installmentTimer, func(f workflow.Future) {
			logger.Info("Installment due date reached", "installmentNumber", installmentNumber)
		})

		// Wait for either payment signal or due date
		selector.Select(ctx)

		// ========== Check if Payment was Received ==========
		if !paymentReceived {
			// ========== INSTALLMENT DEFAULT (IR_16) ==========
			logger.Warn("Installment payment not received - triggering default",
				"installmentNumber", installmentNumber,
				"policyNumber", policyNumber)

			// Mark policy as lapsed (AL status)
			if err := workflow.ExecuteActivity(ctx, "MarkPolicyLapsed", policyNumber).Get(ctx, nil); err != nil {
				logger.Error("Failed to mark policy as lapsed", "error", err)
				// Continue with default process even if activity fails
			}

			// Move collections to suspense (IR_24)
			if err := workflow.ExecuteActivity(ctx, "MoveCollectionsToSuspense", policyNumber).Get(ctx, nil); err != nil {
				logger.Error("Failed to move collections to suspense", "error", err)
				// Continue with default process even if activity fails
			}

			logger.Info("Default processing completed - returning DEFAULT to parent")
			return "DEFAULT", nil // END: Return DEFAULT to Parent
		}

		// ========== INSTALLMENT PAID - Record Payment ==========
		logger.Info("Recording installment payment in database",
			"installmentNumber", installmentNumber,
			"requestID", requestID)

		if err := workflow.ExecuteActivity(ctx, "RecordInstallmentPayment", requestID, installmentPayload).Get(ctx, nil); err != nil {
			logger.Error("Failed to record installment payment", "error", err)
			return "", err
		}

		logger.Info("Installment payment recorded successfully",
			"installmentNumber", installmentNumber,
			"remaining", remainingInstallments-installmentNumber)

		// Continue to next installment
	}

	// ========== ALL INSTALLMENTS PAID SUCCESSFULLY (IR_15) ==========
	logger.Info("All installments paid successfully - revival completed",
		"policyNumber", policyNumber,
		"requestID", requestID,
		"totalInstallments", remainingInstallments)

	return "SUCCESS", nil // END: Return SUCCESS to Parent
}
