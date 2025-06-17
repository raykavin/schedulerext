// Package schedulerext provides a flexible task scheduler with support for cron expressions,
// intervals, and various execution modes. It allows scheduling tasks with different timing
// strategies including one-time execution, recurring intervals, and cron-based scheduling.
package schedulerext

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/gorhill/cronexpr"
)

// Package-level errors that can be returned by the scheduler operations.
var (
	// ErrTaskIDInUse is returned when attempting to add a task with an ID that already exists.
	ErrTaskIDInUse = errors.New("task id already in use")
	
	// ErrTaskFuncNil is returned when a task is created without a function to execute.
	ErrTaskFuncNil = errors.New("task function cannot be nil")
	
	// ErrTaskInterval is returned when a task is created without a valid interval.
	ErrTaskInterval = errors.New("task interval must be definied")
	
	// ErrTaskNotFound is returned when attempting to lookup a task that doesn't exist.
	ErrTaskNotFound = errors.New("could not find task within the task list")
)

// TaskContext provides contextual information for task execution, including
// the task's unique identifier and execution context.
type TaskContext struct {
	Context context.Context // Context for managing the task's lifecycle.
	id      string          // Unique identifier for the task.
}

// ID returns the unique identifier of the task.
func (ctx TaskContext) ID() string { return ctx.id }

// Task represents a scheduled task with its configuration and execution parameters.
// It supports various execution modes including one-time runs, recurring intervals,
// and cron-based scheduling.
type Task struct {
	sync.RWMutex // Managing concurrent modifications to task properties.
	
	// TaskContext provides contextual information and ID for the task.
	TaskContext TaskContext
	
	// Interval specifies when the task should run. Can be a duration string (e.g., "5s", "1m")
	// or a cron expression (e.g., "0 0 * * *" for daily at midnight).
	Interval string
	
	// RunOnce indicates if the task should run only once and then be removed from the scheduler.
	RunOnce bool
	
	// FirstRun indicates if the task should run immediately upon being scheduled.
	FirstRun bool
	
	// RunSingleInstance prevents concurrent instances of the task from running.
	// If true, a new execution will be skipped if the previous one is still running.
	RunSingleInstance bool
	
	// StartAfter specifies the earliest time when the task should start executing.
	// The task will not run before this time.
	StartAfter time.Time
	
	// TaskFunc is the main function to execute. It receives a context for cancellation.
	// Either TaskFunc or FuncWithTaskContext must be provided.
	TaskFunc func(ctx context.Context) error
	
	// ErrFunc is called when TaskFunc returns an error.
	ErrFunc func(error)
	
	// FuncWithTaskContext is an alternative to TaskFunc that receives TaskContext instead of context.Context.
	// Either TaskFunc or FuncWithTaskContext must be provided.
	FuncWithTaskContext func(TaskContext) error
	
	// ErrFuncWithTaskContext is called when FuncWithTaskContext returns an error.
	ErrFuncWithTaskContext func(TaskContext, error)
	
	// Private fields for internal state management
	id       string             // Unique identifier for the task.
	running  sync.Mutex         // Mutex to manage task's running state.
	timer    *time.Timer        // Timer for scheduling the task execution.
	ctx      context.Context    // Context for managing the task's lifecycle.
	cancel   context.CancelFunc // Cancel function to terminate the task's context.
	interval time.Duration      // The duration parsed from interval string.
}

// Clone creates a deep copy of the task. This is useful for creating task templates
// or when you need to inspect a task's configuration without affecting the original.
// The clone shares the same context and timer references as the original.
func (t *Task) Clone() *Task {
	task := new(Task)

	t.safeOps(func() {
		task.TaskFunc = t.TaskFunc
		task.FuncWithTaskContext = t.FuncWithTaskContext
		task.ErrFunc = t.ErrFunc
		task.ErrFuncWithTaskContext = t.ErrFuncWithTaskContext
		task.Interval = t.Interval
		task.StartAfter = t.StartAfter
		task.RunOnce = t.RunOnce
		task.FirstRun = t.FirstRun
		task.RunSingleInstance = t.RunSingleInstance
		task.TaskContext = t.TaskContext
		task.id = t.id
		task.ctx = t.ctx
		task.cancel = t.cancel
		task.timer = t.timer
	})

	return task
}

// safeOps executes the provided function while holding the task's write lock.
// This ensures thread-safe access to the task's properties.
func (t *Task) safeOps(fn func()) {
	t.Lock()
	defer t.Unlock()

	fn()
}

// Scheduler manages a collection of scheduled tasks. It provides thread-safe
// operations for adding, removing, and looking up tasks by their unique identifiers.
type Scheduler struct {
	sync.RWMutex              // RWMutex for managing concurrent access to tasks.
	tasks        map[string]*Task // Map of scheduled tasks, keyed by their unique ID.
}

// NewTaskScheduler creates and returns a new instance of the task scheduler.
// The scheduler is ready to use immediately after creation.
func NewTaskScheduler() *Scheduler {
	scheduler := &Scheduler{
		tasks: make(map[string]*Task),
	}

	return scheduler
}

// Add registers a new task with the scheduler using the provided ID.
// The task will be validated and scheduled according to its configuration.
// Returns an error if the ID is already in use or if the task configuration is invalid.
//
// Example:
//   task := &Task{
//       Interval: "5s",
//       TaskFunc: func(ctx context.Context) error {
//           fmt.Println("Task executed")
//           return nil
//       },
//   }
//   err := scheduler.Add("my-task", task)
func (s *Scheduler) Add(id string, task *Task) error {
	log.Printf("Adding task id %s to scheduler manager...\n", id)

	if err := s.validate(task); err != nil {
		return err
	}

	task.ctx, task.cancel = context.WithCancel(context.Background())
	task.TaskContext.id = id

	s.Lock()
	defer s.Unlock()

	if _, ok := s.tasks[id]; ok {
		return ErrTaskIDInUse
	}

	task.id = id

	s.tasks[id] = task

	return s.scheduleTask(task)
}

// Del removes a task from the scheduler by its ID.
// The task's context will be cancelled and its timer will be stopped.
// If the task doesn't exist, this operation is a no-op.
func (s *Scheduler) Del(name string) {
	t, err := s.Lookup(name)
	if err != nil {
		return
	}

	defer t.cancel()

	t.Lock()
	defer t.Unlock()

	if t.timer != nil {
		defer t.timer.Stop()
	}

	s.Lock()
	defer s.Unlock()

	delete(s.tasks, name)
}

// Lookup retrieves a task by its ID. Returns a clone of the task to prevent
// external modifications to the original task state.
// Returns ErrTaskNotFound if the task doesn't exist.
func (s *Scheduler) Lookup(name string) (*Task, error) {
	s.RLock()
	defer s.RUnlock()

	t, ok := s.tasks[name]
	if ok {
		return t.Clone(), nil
	}

	return t, ErrTaskNotFound
}

// Tasks returns a map of all scheduled tasks, keyed by their IDs.
// The returned tasks are clones to prevent external modifications.
// This method is useful for inspecting the current state of all scheduled tasks.
func (s *Scheduler) Tasks() map[string]*Task {
	s.RLock()
	defer s.RUnlock()

	m := make(map[string]*Task)

	for k, v := range s.tasks {
		m[k] = v.Clone()
	}

	return m
}

// scheduleTask sets up the initial scheduling for a task based on its configuration.
// It parses the interval, handles FirstRun and StartAfter settings, and creates
// the appropriate timer for task execution.
func (s *Scheduler) scheduleTask(t *Task) error {
	var err error
	now := time.Now()

	t.interval, err = s.getInterval(t.Interval, now)
	if err != nil {
		return errors.New("invalid scheduler task interval")
	}

	runFn := func() {
		s.execTask(t)
		// if !t.FirstRun && !t.RunOnce {
		// 	s.logNextRun(t)
		// }
	}

	if t.FirstRun {
		runFn()
	}

	_ = time.AfterFunc(time.Until(t.StartAfter), func() {
		var err error

		t.safeOps(func() { err = t.ctx.Err() })
		if err != nil {
			return
		}

		t.safeOps(func() {
			t.timer = time.AfterFunc(t.interval, runFn)
		})
	})

	// s.logNextRun(t)

	return nil
}

// logNextRun logs information about the next scheduled execution of a task.
// This method is currently commented out but can be used for debugging purposes.
// func (s *Scheduler) logNextRun(t *Task) {
// 	var nextRunStr string

// 	nextRunTime := time.Now()
// 	nextRunStr = nextRunTime.String()

// 	s.logger.WithFields(map[string]interface{}{
// 		"next_run": nextRunStr,
// 		"task_id":  t.id,
// 	}).Debug("Task scheduled successfully")
// }

// getInterval parses the interval string and returns the corresponding duration.
// It supports both standard Go duration strings (e.g., "5s", "1m", "1h") and
// cron expressions (e.g., "0 0 * * *"). For cron expressions, it calculates
// the time until the next execution.
func (s *Scheduler) getInterval(intervalStr string, now time.Time) (
	time.Duration,
	error,
) {
	d, err := time.ParseDuration(intervalStr)
	if err == nil {
		return d, nil
	}

	expr, err := cronexpr.Parse(intervalStr)
	if err != nil {
		return 0, err
	}

	next := expr.Next(now)

	return next.Sub(now), nil
}

// execTask executes a task in a separate goroutine, handling various execution modes
// and error scenarios. It respects the RunSingleInstance setting, calls appropriate
// error handlers, and manages task lifecycle for RunOnce tasks.
func (s *Scheduler) execTask(t *Task) {
	go func() {
		// start := time.Now()

		if t.RunSingleInstance {
			if !t.running.TryLock() {
				return
			}

			defer t.running.Unlock()
		}

		var err error
		if t.FuncWithTaskContext != nil {
			err = t.FuncWithTaskContext(t.TaskContext)
		} else {
			err = t.TaskFunc(t.ctx)
		}

		// duration := time.Since(start)

		// s.logger.WithFields(map[string]interface{}{
		// 	"task_id":  t.id,
		// 	"duration": duration.String(),
		// }).Debug("Task execution finished")

		if err != nil && (t.ErrFunc != nil || t.ErrFuncWithTaskContext != nil) {
			if t.ErrFuncWithTaskContext != nil {
				go t.ErrFuncWithTaskContext(t.TaskContext, err)
			} else {
				go t.ErrFunc(err)
			}
		}

		if t.FirstRun {
			t.FirstRun = false
		}

		if t.RunOnce {
			defer s.Del(t.id)
		}
	}()

	if !t.RunOnce {
		t.safeOps(func() {
			if t.timer != nil {
				t.timer.Reset(t.interval)
			}
		})
	}
}

// validate checks if a task has the minimum required configuration to be scheduled.
// It ensures that either TaskFunc or FuncWithTaskContext is provided and that
// an interval is specified.
func (s *Scheduler) validate(t *Task) error {
	if t.TaskFunc == nil && t.FuncWithTaskContext == nil {
		return ErrTaskFuncNil
	}

	if len(t.Interval) == 0 {
		return ErrTaskInterval
	}

	return nil
}
