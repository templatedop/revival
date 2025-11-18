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
	c, err := client.NewClient(client.Options{})
	if err != nil {
		log.Fatalln("unable to create Temporal client", err)
	}
	defer c.Close()

	// create a simple request in the in-memory store via activity-less hack:
	// (In real world store, you'd have API to create request and policy.)
	// For prototype, we assume requestID and policy exist (store seeded in worker)
	requestID := "REQ-1001"
	//policyNo := "POL-1001"

	// Start workflow (parent)
	we, err := c.ExecuteWorkflow(context.Background(), client.StartWorkflowOptions{
		ID:        "revival-" + uuid.NewString(),
		TaskQueue: "revival-task-queue",
	}, "RevivalParentWorkflow", requestID)
	if err != nil {
		log.Fatalln("start workflow error:", err)
	}

	fmt.Println("Started Revival Workflow. WorkflowID:", we.GetID(), "RunID:", we.GetRunID())

	// Simulate sending first installment signal after 3s (in real life after payment)
	time.Sleep(3 * time.Second)
	err = c.SignalWorkflow(context.Background(), we.GetID(), we.GetRunID(), "FirstInstallmentPaid", map[string]interface{}{
		"amount": 9363.00,
		"receipt": "RCPT-001",
	})
	if err != nil {
		log.Fatalln("signal error:", err)
	}
	fmt.Println("Sent FirstInstallmentPaid signal")

	// Simulate subsequent installments (loop) - in prototype send one after some seconds
	time.Sleep(5 * time.Second)
	err = c.SignalWorkflow(context.Background(), we.GetID(), we.GetRunID(), "InstallmentPaid", map[string]interface{}{
		"amount": 8572.89,
		"receipt": "RCPT-002",
	})
	if err != nil {
		log.Fatalln("signal error 2:", err)
	}
	fmt.Println("Sent InstallmentPaid signal")
}
