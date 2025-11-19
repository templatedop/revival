package main

import (
	"log"
	"time"

	"revival/internal/activity"
	"revival/internal/store"
	"revival/internal/workflow"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	// Create in-memory store
	s := store.NewInMemoryStore()

	// Seed the store with test data for prototype
	seedTestData(s)

	// Create Temporal client
	c, err := client.NewClient(client.Options{})
	if err != nil {
		log.Fatalln("Unable to create Temporal client", err)
	}
	defer c.Close()

	// Create worker
	w := worker.New(c, "revival-task-queue", worker.Options{})

	// Register workflows and activities
	w.RegisterWorkflow(workflow.RevivalParentWorkflow)
	w.RegisterWorkflow(workflow.InstallmentMonitorWorkflow)

	act := activity.NewActivities(s)
	w.RegisterActivity(act)

	// Start worker
	log.Println("Starting worker for task queue revival-task-queue...")
	log.Println("Test data seeded: Policy POL-1001, Request REQ-1001")
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalln("Worker failed", err)
	}
}

// seedTestData populates the in-memory store with test data
func seedTestData(s *store.InMemoryStore) {
	// Create a test policy
	// Last paid: Oct 2024, so policy lapsed Nov 2024 (13 months ago from today)
	testPolicy := store.Policy{
		PolicyNumber:     "POL-1001",
		IssueDate:        mustParseDate("2014-01-30"),
		MaturityDate:     ptr(mustParseDate("2034-01-30")),
		PremiumFrequency: "MONTHLY",
		ModalPremium:     5852.0,
		LastPaidToDate:   mustParseDate("2024-10-31"),
		RevivalCount:     0,
	}
	s.SavePolicy(testPolicy)

	// Create a test revival request
	// Indexed Nov 2024, 1 month after lapse, 13 months unpaid premiums
	testRequest := store.RevivalRequest{
		RequestID:        "REQ-1001",
		PolicyNumber:     "POL-1001",
		IndexedAt:        mustParseDate("2024-11-30"),
		NoOfInstallments: 5,
		GSTPercent:       2.25,
		MonthlyInterest:  0.01,
		InterestMonths:   12,  // 12 months of interest accumulation
		UnpaidMonths:     13,  // 13 months unpaid
		Status:           "PENDING",
	}
	s.SaveRequest(testRequest)

	log.Println("Test data seeded successfully")
}

// Helper functions
func mustParseDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func ptr(t time.Time) *time.Time {
	return &t
}
