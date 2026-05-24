package worker

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"notification-service/pkg/queue"
)

// Pool manages a group of concurrent workers processing jobs.
type Pool struct {
	concurrency int
	consumer    *queue.Consumer
	dispatcher  *Dispatcher
	logger      *slog.Logger

	// runCtx handles gracefully draining in-flight dispatches
	runCtx    context.Context
	runCancel context.CancelFunc

	// dequeueCtx handles instantly stopping BLMOVE blocking pops
	dequeueCtx    context.Context
	dequeueCancel context.CancelFunc

	wg sync.WaitGroup
}

// NewPool creates a new worker pool.
func NewPool(concurrency int, consumer *queue.Consumer, dispatcher *Dispatcher, logger *slog.Logger) *Pool {
	return &Pool{
		concurrency: concurrency,
		consumer:    consumer,
		dispatcher:  dispatcher,
		logger:      logger,
	}
}

// Start initializes the worker pool and begins consuming jobs.
func (p *Pool) Start(ctx context.Context) {
	p.runCtx, p.runCancel = context.WithCancel(ctx)
	p.dequeueCtx, p.dequeueCancel = context.WithCancel(ctx)

	p.logger.Info("Starting worker pool", "concurrency", p.concurrency)

	for i := 0; i < p.concurrency; i++ {
		p.wg.Add(1)
		go p.workerLoop(i)
	}
}

// Stop gracefully shuts down the worker pool, allowing in-flight jobs to finish.
func (p *Pool) Stop() {
	p.logger.Info("Stopping worker pool...")
	// 1. Cancel dequeue context to wake up idle workers blocked on BLMOVE
	p.dequeueCancel()
	
	// 2. Wait for all active worker loops to drain and exit
	p.wg.Wait()
	
	// 3. Cancel the run context for clean up
	p.runCancel()
	p.logger.Info("Worker pool stopped gracefully")
}

func (p *Pool) workerLoop(id int) {
	defer p.wg.Done()
	p.logger.Debug("Worker started", "worker_id", id)

	for {
		// Dequeue blocks until a job is available or dequeueCtx is cancelled
		job, err := p.consumer.Dequeue(p.dequeueCtx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				p.logger.Debug("Worker shutting down", "worker_id", id)
				return
			}
			if errors.Is(err, queue.ErrNoJob) {
				continue
			}
			p.logger.Error("Failed to dequeue job", "worker_id", id, "error", err)
			
			// Sleep briefly on non-cancellation errors to avoid busy loops
			select {
			case <-p.dequeueCtx.Done():
				return
			case <-time.After(1 * time.Second):
			}
			continue
		}

		// Create a separate timeout context for the dispatch so it's not bound to dequeue cancellation
		// This ensures in-flight requests finish during graceful shutdown
		dispatchCtx, cancel := context.WithTimeout(p.runCtx, 15*time.Second)
		p.dispatcher.Dispatch(dispatchCtx, job)
		cancel()
	}
}
