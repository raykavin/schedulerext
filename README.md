# Go Scheduler Package

The Go Scheduler package provides a flexible and lightweight task scheduling mechanism for Go applications. You can schedule tasks to run periodically using either duration strings or cron expressions. It supports delayed starts, one-time execution, single-instance execution, and custom error handling.

## Features

- **Flexible Scheduling:**  
  Schedule tasks using either standard Go duration strings (e.g., `"1s"`, `"500ms"`) or cron expressions (e.g., `"* * * * * *"`).

- **Immediate or Delayed Start:**  
  Configure tasks to start immediately (`FirstRun`) or at a later time using `StartAfter`.

- **One-Time or Recurring Tasks:**  
  Use `RunOnce` to run a task only once or leave it running repeatedly.

- **Single Instance Execution:**  
  Prevent concurrent execution of tasks with the `RunSingleInstance` flag.

- **Error Handling:**  
  Supply custom error handlers (`ErrFunc` or `ErrFuncWithTaskContext`) to process errors from task execution.

- **Thread-Safe:**  
  Built-in synchronization ensures safe concurrent access and modifications.

## Installation

Install the package using `go get`. Replace `github.com/yourusername/schedulerext` with your actual module path.

```bash
go get github.com/yourusername/schedulerext
```

## Usage

Import the package into your project:

```go
import "github.com/yourusername/schedulerext/pkg"
```

## Scheduling a Simple Task

The following example demonstrates scheduling a task that prints a message every second.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yourusername/schedulerext/pkg"
)

func main() {
	scheduler := pkg.NewTaskScheduler()

	task := &pkg.Task{
		Interval: "1s",     // Execute every 1 second
		RunOnce:  false,    // Recurring task
		FirstRun: true,     // Execute immediately upon scheduling
		TaskFunc: func(ctx context.Context) error {
			fmt.Println("Simple task executed!")
			return nil
		},
	}

	if err := scheduler.Add("simpleTask", task); err != nil {
		log.Fatalf("Error adding task: %v", err)
	}

	// Keep the application running
	select {}
}
```

## Scheduling a Cron Task

Schedule a task using a cron expression (with seconds) to fire every 5 seconds.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yourusername/schedulerext/pkg"
)

func main() {
	scheduler := pkg.NewTaskScheduler()

	cronTask := &pkg.Task{
		Interval: "* * * * * *", // Cron expression: every second
		RunOnce:  false,         // Recurring task
		FirstRun: true,          // Execute immediately upon scheduling
		TaskFunc: func(ctx context.Context) error {
			fmt.Println("Cron task executed!")
			return nil
		},
	}

	if err := scheduler.Add("cronTask", cronTask); err != nil {
		log.Fatalf("Error adding cron task: %v", err)
	}

	select {}
}
```

## Delayed Start Task

Schedule a task to start after a specific delay using `StartAfter`.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yourusername/schedulerext/pkg"
)

func main() {
	scheduler := pkg.NewTaskScheduler()

	delayedTask := &pkg.Task{
		Interval:   "2s",                           // Task repeats every 2 seconds
		RunOnce:    false,                          // Recurring task
		FirstRun:   false,                          // Do not run immediately
		StartAfter: time.Now().Add(5 * time.Second),  // Delay start by 5 seconds
		TaskFunc: func(ctx context.Context) error {
			fmt.Println("Delayed task executed!")
			return nil
		},
	}

	if err := scheduler.Add("delayedTask", delayedTask); err != nil {
		log.Fatalf("Error adding delayed task: %v", err)
	}

	select {}
}
```

## Error Handling

Handle errors produced by a task using a custom error callback.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yourusername/schedulerext/pkg"
)

func main() {
	scheduler := pkg.NewTaskScheduler()

	errorTask := &pkg.Task{
		Interval: "1s",    // Execute every 1 second
		RunOnce:  true,    // Run only once
		FirstRun: true,    // Execute immediately
		TaskFunc: func(ctx context.Context) error {
			return fmt.Errorf("simulated error")
		},
		ErrFunc: func(err error) {
			fmt.Printf("Error handler invoked: %v\n", err)
		},
	}

	if err := scheduler.Add("errorTask", errorTask); err != nil {
		log.Fatalf("Error adding error task: %v", err)
	}

	time.Sleep(1500 * time.Millisecond)
}
```

## Testing

The package includes a suite of tests covering task scheduling, error handling, cloning, and more. To run the tests, use:

```bash
go test -v
```

## Contributing

Contributions are welcome! Feel free to open issues or submit pull requests.


## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
