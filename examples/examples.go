package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/raykavin/goschd"
)

func main() {
	// Create a new scheduler.
	scheduler := goschd.NewTaskScheduler()

	// ------------------------------
	// Example 1: Simple Recurring Task
	// ------------------------------
	simpleTask := &goschd.Task{
		Interval: "2s",  // run every 2 seconds
		RunOnce:  false, // repeat the task
		FirstRun: true,  // execute immediately upon scheduling
		TaskFunc: func(ctx context.Context) error {
			fmt.Println("[Simple Task] Hello, world!")
			return nil
		},
	}
	if err := scheduler.Add("simpleTask", simpleTask); err != nil {
		log.Fatalf("Error adding simpleTask: %v", err)
	}

	// ------------------------------
	// Example 2: Cron Task
	// ------------------------------
	cronTask := &goschd.Task{
		Interval: "*/5 * * * * *", // using a cron expression: fires every 5 seconds (with seconds field)
		RunOnce:  false,           // repeat the task
		FirstRun: true,            // execute immediately upon scheduling
		TaskFunc: func(ctx context.Context) error {
			fmt.Println("[Cron Task] Fired every 5 seconds")
			return nil
		},
	}
	if err := scheduler.Add("cronTask", cronTask); err != nil {
		log.Fatalf("Error adding cronTask: %v", err)
	}

	// ------------------------------
	// Example 3: Error Handling Task
	// ------------------------------
	errorTask := &goschd.Task{
		Interval: "3s",  // run every 3 seconds
		RunOnce:  false, // repeat the task
		FirstRun: true,  // execute immediately upon scheduling
		TaskFunc: func(ctx context.Context) error {
			fmt.Println("[Error Task] About to simulate an error")
			return fmt.Errorf("simulated error")
		},
		ErrFunc: func(err error) {
			fmt.Printf("[Error Handler] Received error: %v\n", err)
		},
	}
	if err := scheduler.Add("errorTask", errorTask); err != nil {
		log.Fatalf("Error adding errorTask: %v", err)
	}

	// ------------------------------
	// Example 4: Delayed Start Task
	// ------------------------------
	delayedTask := &goschd.Task{
		Interval:   "4s",                             // repeat every 4 seconds
		RunOnce:    false,                            // repeat the task
		FirstRun:   false,                            // do not run immediately
		StartAfter: time.Now().Add(10 * time.Second), // delay start by 10 seconds
		TaskFunc: func(ctx context.Context) error {
			fmt.Println("[Delayed Task] Started after delay")
			return nil
		},
	}
	if err := scheduler.Add("delayedTask", delayedTask); err != nil {
		log.Fatalf("Error adding delayedTask: %v", err)
	}

	// ------------------------------
	// Keep the application running.
	// ------------------------------
	fmt.Println("Scheduler started. Press Ctrl+C to exit.")
	select {} // Block forever.
}
