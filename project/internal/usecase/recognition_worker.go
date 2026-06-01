package usecase

import (
	"context"
	"log"
	"time"

	"diplom.com/m/internal/ports"
)

// RecognitionProcessor is the slice of DocumentService the worker needs: run the
// recognition pipeline for one document. *DocumentService satisfies it.
type RecognitionProcessor interface {
	ProcessRecognition(ctx context.Context, documentID int64) error
}

// RecognitionWorker drains a durable RecognitionQueue and runs recognition for
// each claimed job (Option B in docs/async-recognition-design.md). It polls the
// queue on an interval; when a job is found it keeps draining until the queue is
// empty before sleeping again, so a burst of uploads is processed promptly.
//
// Durability and retries live in the queue: Claim reserves a job, Complete
// finalizes success, and Fail re-queues with backoff or gives up after
// MaxAttempts. The worker itself is stateless and safe to run as multiple
// instances against the same queue.
type RecognitionWorker struct {
	Queue     ports.RecognitionQueue
	Processor RecognitionProcessor

	// PollInterval is how long to wait after finding the queue empty before
	// polling again. Defaults to one second when zero.
	PollInterval time.Duration
	// RetryBackoff is how long a failed job waits before becoming runnable
	// again. Defaults to thirty seconds when zero.
	RetryBackoff time.Duration
	// Logger, when set, receives one line per processed/failed job. Defaults to
	// the standard logger.
	Logger *log.Logger
}

func (w *RecognitionWorker) pollInterval() time.Duration {
	if w.PollInterval > 0 {
		return w.PollInterval
	}
	return time.Second
}

func (w *RecognitionWorker) retryBackoff() time.Duration {
	if w.RetryBackoff > 0 {
		return w.RetryBackoff
	}
	return 30 * time.Second
}

func (w *RecognitionWorker) logf(format string, args ...any) {
	if w.Logger != nil {
		w.Logger.Printf(format, args...)
		return
	}
	log.Printf(format, args...)
}

// Run blocks until ctx is cancelled, processing jobs as they appear. It is the
// long-lived goroutine started from main.
func (w *RecognitionWorker) Run(ctx context.Context) {
	w.logf("recognition worker: started (poll %s, backoff %s)", w.pollInterval(), w.retryBackoff())
	timer := time.NewTimer(w.pollInterval())
	defer timer.Stop()
	for {
		// Drain the queue greedily; processedSomething tells us whether to poll
		// again immediately or sleep.
		processedSomething := w.drain(ctx)
		if ctx.Err() != nil {
			w.logf("recognition worker: stopped")
			return
		}
		if processedSomething {
			continue
		}
		timer.Reset(w.pollInterval())
		select {
		case <-ctx.Done():
			w.logf("recognition worker: stopped")
			return
		case <-timer.C:
		}
	}
}

// drain claims and processes jobs until the queue is empty or an error/cancel
// occurs. It returns true if at least one job was claimed.
func (w *RecognitionWorker) drain(ctx context.Context) bool {
	processedSomething := false
	for {
		if ctx.Err() != nil {
			return processedSomething
		}
		job, ok, err := w.Queue.Claim(ctx)
		if err != nil {
			w.logf("recognition worker: claim error: %v", err)
			return processedSomething
		}
		if !ok {
			return processedSomething
		}
		processedSomething = true
		w.process(ctx, job)
	}
}

func (w *RecognitionWorker) process(ctx context.Context, job ports.RecognitionJob) {
	if err := w.Processor.ProcessRecognition(ctx, job.DocumentID); err != nil {
		if ferr := w.Queue.Fail(ctx, job.ID, err.Error(), w.retryBackoff()); ferr != nil {
			w.logf("recognition worker: fail bookkeeping error for job %d: %v", job.ID, ferr)
		}
		w.logf("recognition worker: job %d (doc %d) failed (attempt %d/%d): %v",
			job.ID, job.DocumentID, job.Attempts+1, job.MaxAttempts, err)
		return
	}
	if err := w.Queue.Complete(ctx, job.ID); err != nil {
		w.logf("recognition worker: complete bookkeeping error for job %d: %v", job.ID, err)
		return
	}
	w.logf("recognition worker: job %d (doc %d) done", job.ID, job.DocumentID)
}
