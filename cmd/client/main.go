package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
)

func main() {
	// Create Temporal client
	c, err := client.NewClient(client.Options{})
	if err != nil {
		log.Fatalln("Unable to create Temporal client:", err)
	}
	defer c.Close()

	// Use the test data seeded in the worker
	// Policy: POL-1001, Request: REQ-1001
	requestID := "REQ-1001"

	fmt.Println("========== Revival Workflow Demo ==========")
	fmt.Println("Request ID:", requestID)
	fmt.Println("Policy Number: POL-1001")
	fmt.Println()

	// Start the parent workflow
	workflowID := "revival-" + uuid.NewString()
	we, err := c.ExecuteWorkflow(context.Background(), client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "revival-task-queue",
	}, "RevivalParentWorkflow", requestID)
	if err != nil {
		log.Fatalln("Failed to start workflow:", err)
	}

	fmt.Printf("✓ Started Revival Parent Workflow\n")
	fmt.Printf("  Workflow ID: %s\n", we.GetID())
	fmt.Printf("  Run ID: %s\n", we.GetRunID())
	fmt.Println()

	// Wait a bit for workflow to process initial steps
	fmt.Println("Workflow is processing:")
	fmt.Println("  - Loading policy and request data")
	fmt.Println("  - Checking maturity date and 5-year limit")
	fmt.Println("  - Performing data entry and QC")
	fmt.Println("  - Approver review (auto-approved for demo)")
	fmt.Println("  - Generating acceptance letter")
	fmt.Println("  - Starting 60-day SLA timer...")
	fmt.Println()
	time.Sleep(3 * time.Second)

	// Send first installment payment signal
	// Note: Actual amounts calculated by system based on 13 unpaid months
	fmt.Println("Simulating first installment payment...")
	err = c.SignalWorkflow(context.Background(), we.GetID(), we.GetRunID(), "FirstInstallmentPaid", map[string]interface{}{
		"amount":  24348.55,
		"receipt": "RCPT-FIRST-001",
	})
	if err != nil {
		log.Fatalln("Failed to send FirstInstallmentPaid signal:", err)
	}
	fmt.Printf("✓ Sent FirstInstallmentPaid signal (Amount: ₹24,348.55)\n")
	fmt.Println("  - Processing first installment")
	fmt.Println("  - Generating revival memo")
	fmt.Println("  - Updating policy status to AP (Active Premium)")
	fmt.Println("  - Starting child workflow for remaining 4 installments")
	fmt.Println()

	// Simulate subsequent installment payments (13 unpaid months scenario)
	installments := []struct {
		number  int
		amount  float64
		receipt string
		delay   time.Duration
	}{
		{2, 22636.84, "RCPT-002", 3 * time.Second},
		{3, 22636.84, "RCPT-003", 2 * time.Second},
		{4, 22636.84, "RCPT-004", 2 * time.Second},
		{5, 22636.84, "RCPT-005", 2 * time.Second},
	}

	for _, inst := range installments {
		time.Sleep(inst.delay)
		fmt.Printf("Simulating installment #%d payment...\n", inst.number)
		err = c.SignalWorkflow(context.Background(), we.GetID(), we.GetRunID(), "InstallmentPaid", map[string]interface{}{
			"amount":  inst.amount,
			"receipt": inst.receipt,
		})
		if err != nil {
			log.Printf("Failed to send InstallmentPaid signal #%d: %v\n", inst.number, err)
		} else {
			fmt.Printf("✓ Sent InstallmentPaid signal #%d (Amount: ₹%.2f)\n", inst.number, inst.amount)
		}
	}

	fmt.Println()
	fmt.Println("Waiting for workflow to complete...")
	fmt.Println()

	// Wait for workflow completion (with timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = we.Get(ctx, nil)
	if err != nil {
		log.Fatalln("Workflow execution failed:", err)
	}

	fmt.Println("========================================")
	fmt.Println("✓ Revival Workflow Completed Successfully!")
	fmt.Println("  All 5 installments paid")
	fmt.Println("  Policy revival completed")
	fmt.Println("  Request marked as COMPLETED")
	fmt.Println("========================================")
}
