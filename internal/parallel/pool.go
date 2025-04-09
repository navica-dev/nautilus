package parallel

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog/log"
)

// Pool manages a pool of worker goroutines for parallel execution
type Pool struct {
	// Number of worker goroutines
	workers int

	// Size of task channel buffer
	bufferSize int

	// Task channel
	tasks chan Task

	// Wait group for workers
	wg sync.WaitGroup

	// State
	running bool
	mu      sync.RWMutex

	// Stats
	totalTasks     int64
	completedTasks int64
	failedTasks    int64
}

// Task represents a unit of work to be executed by the pool
type Task struct {
	// The function to execute
	Fn func(context.Context) error

	// Context for the task
	Ctx context.Context

	// Result channel
	Result chan<- error
}

// NewPool creates a new worker pool
func NewPool(workers, bufferSize int) *Pool {
	if workers <= 0 {
		workers = 1
	}

	if bufferSize <= 0 {
		bufferSize = 10
	}

	return &Pool{
		workers:    workers,
		bufferSize: bufferSize,
		tasks:      make(chan Task, bufferSize),
		running:    false,
	}
}

// Start starts the worker pool
func (p *Pool) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return
	}

	p.running = true
	p.tasks = make(chan Task, p.bufferSize)

	// Start worker goroutines
	p.wg.Add(p.workers)
	for i := range p.workers {
		go p.worker(i)
	}

	log.Debug().
		Int("workers", p.workers).
		Int("buffer_size", p.bufferSize).
		Msg("Started parallel execution pool")
}

// Stop stops the worker pool
func (p *Pool) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return
	}

	close(p.tasks)
	p.wg.Wait()
	p.running = false

	log.Debug().
		Int64("total_tasks", p.totalTasks).
		Int64("completed_tasks", p.completedTasks).
		Int64("failed_tasks", p.failedTasks).
		Msg("Stopped parallel execution pool")
}

// Execute submits a task to the pool and waits for it to complete
func (p *Pool) Execute(ctx context.Context, fn func(context.Context) error) error {
	// Start the pool if not already running
	p.mu.RLock()
	running := p.running
	p.mu.RUnlock()

	if !running {
		p.Start()
	}

	// Create result channel
	resultCh := make(chan error, 1)

	// Submit task
	task := Task{
		Fn:     fn,
		Ctx:    ctx,
		Result: resultCh,
	}

	atomic.AddInt64(&p.totalTasks, 1)

	select {
	case p.tasks <- task:
		// Task submitted successfully
	case <-ctx.Done():
		// Context cancelled before task could be submitted
		atomic.AddInt64(&p.failedTasks, 1)
		return ctx.Err()
	}

	// Wait for result
	select {
	case err := <-resultCh:
		if err != nil {
			atomic.AddInt64(&p.failedTasks, 1)
		} else {
			atomic.AddInt64(&p.completedTasks, 1)
		}
		return err
	case <-ctx.Done():
		// Context cancelled while waiting for result
		atomic.AddInt64(&p.failedTasks, 1)
		return ctx.Err()
	}
}

// ExecuteAll submits multiple tasks to the pool and waits for all to complete
func (p *Pool) ExecuteAll(ctx context.Context, fns []func(context.Context) error) []error {
	// Start the pool if not already running
	p.mu.RLock()
	running := p.running
	p.mu.RUnlock()

	if !running {
		p.Start()
	}

	// Create a context that can be cancelled if one task fails
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Create result channels
	var wg sync.WaitGroup
	errors := make([]error, len(fns))

	// Submit all tasks
	for i, fn := range fns {
		wg.Add(1)

		// Capture loop variables
		index := i
		taskFn := fn

		go func() {
			defer wg.Done()

			err := p.Execute(ctx, taskFn)
			errors[index] = err

			// Cancel context if task failed
			if err != nil {
				cancel()
			}
		}()
	}

	// Wait for all tasks to complete
	wg.Wait()

	return errors
}

// worker is a goroutine that processes tasks from the pool
func (p *Pool) worker(id int) {
	defer p.wg.Done()

	logger := log.With().Int("worker_id", id).Logger()
	logger.Debug().Msg("Worker started")

	//Recovery mechanism
	defer func() {
		if r := recover(); r != nil {
			// Log the panic
			logger.Error().Interface("panic", r).Msg("worker recovered from panic")

			// Optionally increment a metric for panics
			atomic.AddInt64(&p.failedTasks, 1)
		}
	}()

	for task := range p.tasks {
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Log the panic with task info
					logger.Error().Interface("panic", r).Msg("task execution panicked")

					// Try to send error to result channel
					select {
					case task.Result <- fmt.Errorf("task panicked: %v", r):
					case <-task.Ctx.Done():
						// Context cancelled, but try to send error anyway
						select {
						case task.Result <- fmt.Errorf("task panicked and cancelled: %v", r):
						default:
							// Result channel is closed or full
						}
					}
				}
			}()

			// Check if context is already cancelled
			if task.Ctx.Err() != nil {
				task.Result <- task.Ctx.Err()
				return
			}

			// Execute the task
			err := task.Fn(task.Ctx)

			// Send result
			select {
			case task.Result <- err:
				// Result sent successfully
			case <-task.Ctx.Done():
				// Context cancelled, but try to send error anyway
				select {
				case task.Result <- fmt.Errorf("task cancelled: %w", task.Ctx.Err()):
				default:
					// Result channel is closed or full
				}
			}
		}()
	}

	logger.Debug().Msg("Worker stopped")
}

// GetStats returns statistics about the pool
func (p *Pool) GetStats() (totalTasks, completedTasks, failedTasks int64) {
	return atomic.LoadInt64(&p.totalTasks),
		atomic.LoadInt64(&p.completedTasks),
		atomic.LoadInt64(&p.failedTasks)
}

// IsRunning returns true if the pool is running
func (p *Pool) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// Resize changes the number of workers in the pool
func (p *Pool) Resize(workers int) {
	if workers <= 0 {
		return
	}

	// Stop and restart the pool with new size
	p.Stop()

	p.mu.Lock()
	p.workers = workers
	p.mu.Unlock()

	p.Start()
}
