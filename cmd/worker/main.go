package main

import (
	"log"

	"revival/internal/activity"
	"revival/internal/store"
	"revival/internal/workflow"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	// Create in-memory store
	s := store.NewInMemoryStore()

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
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalln("Worker failed", err)
	}
}
