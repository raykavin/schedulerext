package schedulerext

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/gorhill/cronexpr"
)

var (
	ErrTaskIDInUse  = errors.New("task id already in use")
	ErrTaskFuncNil  = errors.New("task function cannot be nil")
	ErrTaskInterval = errors.New("task interval must be definied")
	ErrTaskNotFound = errors.New("could not find task within the task list")
)

type TaskContext struct {
	Context context.Context // Context for managing the task's lifecycle.
	id      string          // Unique identifier for the task.
}

func (ctx TaskContext) ID() string { return ctx.id }

type Task struct {
	sync.RWMutex                                           // Managing concurrent modifications to task properties.
	TaskContext            TaskContext                     // Contextual information and ID for the task.
	Interval               string                          // Interval at which the task should run.
	RunOnce                bool                            // If true, the task will run only once.
	FirstRun               bool                            // If true, the task will run immediately.
	RunSingleInstance      bool                            // If true, prevents concurrent instances of the task from running.
	StartAfter             time.Time                       // Specifies when the task should start.
	TaskFunc               func(ctx context.Context) error // The main function or logic of the task.
	ErrFunc                func(error)                     // Function to handle errors occurring in TaskFunc.
	FuncWithTaskContext    func(TaskContext) error         // Task function with access to the TaskContext.
	ErrFuncWithTaskContext func(TaskContext, error)        // Error handler with access to the TaskContext.
	id                     string                          // Unique identifier for the task.
	running                sync.Mutex                      // Mutex to manage task's running state.
	timer                  *time.Timer                     // Timer for scheduling the task execution.
	ctx                    context.Context                 // Context for managing the task's lifecycle.
	cancel                 context.CancelFunc              // Cancel function to terminate the task's context.
	interval               time.Duration                   // The duration parsed from interval
}

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

func (t *Task) safeOps(fn func()) {
	t.Lock()
	defer t.Unlock()

	fn()
}

type Scheduler struct {
	sync.RWMutex                  // RWMutex for managing concurrent access to tasks.
	tasks        map[string]*Task // Map of scheduled tasks, keyed by their unique ID.
}

func NewTaskScheduler() *Scheduler {
	scheduler := &Scheduler{
		tasks: make(map[string]*Task),
	}

	return scheduler
}

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

func (s *Scheduler) Lookup(name string) (*Task, error) {
	s.RLock()
	defer s.RUnlock()

	t, ok := s.tasks[name]
	if ok {
		return t.Clone(), nil
	}

	return t, ErrTaskNotFound
}

func (s *Scheduler) Tasks() map[string]*Task {
	s.RLock()
	defer s.RUnlock()

	m := make(map[string]*Task)

	for k, v := range s.tasks {
		m[k] = v.Clone()
	}

	return m
}

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

// func (s *Scheduler) logNextRun(t *Task) {
// 	var nextRunStr string

// 	nextRunTime := time.Now()
// 	nextRunStr = nextRunTime.String()

// 	s.logger.WithFields(map[string]interface{}{
// 		"next_run": nextRunStr,
// 		"task_id":  t.id,
// 	}).Debug("Task scheduled successfully")
// }

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

func (s *Scheduler) validate(t *Task) error {
	if t.TaskFunc == nil && t.FuncWithTaskContext == nil {
		return ErrTaskFuncNil
	}

	if len(t.Interval) == 0 {
		return ErrTaskInterval
	}

	return nil
}
