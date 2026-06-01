// Package inprocqueue is an ALTERNATIVE, example-only implementation of
// asynchronous document recognition: a buffered Go channel drained by a pool of
// goroutines (Option A in docs/async-recognition-design.md).
//
// It is intentionally NOT the default. It is here to show the simplest possible
// async design and to contrast it with the durable, Postgres-backed queue
// (Option B, internal/adapters/pgcore/recognition_queue_repo.go) that the system
// actually ships with and tests against.
//
// Trade-offs (why this is example-only):
//   - Not durable: jobs live only in memory. A process restart loses every
//     queued and in-flight job, leaving those documents stuck in `uploaded`.
//   - No retries/backoff: a failed recognition is logged and dropped.
//   - Single-instance: the channel is local, so it cannot coordinate work
//     across multiple backend instances.
//
// Prefer Option B for anything beyond a quick local demo.
package inprocqueue

import (
	"context"
	"log"
	"sync"

	"diplom.com/m/internal/ports"
)

// Processor runs recognition for one document. *usecase.DocumentService
// satisfies it (via ProcessRecognition).
type Processor interface {
	ProcessRecognition(ctx context.Context, documentID int64) error
}

// Queue is an in-process recognition queue backed by a buffered channel and a
// fixed pool of worker goroutines. It implements ports.RecognitionEnqueuer, so
// DocumentService can use it as a drop-in producer in place of the DB queue.
type Queue struct {
	jobs      chan int64
	processor Processor
	workers   int
	logger    *log.Logger

	wg   sync.WaitGroup
	once sync.Once
}

// New builds a Queue with the given buffer size and worker count. Call Start to
// spawn the workers.
func New(processor Processor, buffer, workers int, logger *log.Logger) *Queue {
	if buffer < 1 {
		buffer = 64
	}
	if workers < 1 {
		workers = 4
	}
	if logger == nil {
		logger = log.Default()
	}
	return &Queue{
		jobs:      make(chan int64, buffer),
		processor: processor,
		workers:   workers,
		logger:    logger,
	}
}

// Start launches the worker goroutines. They run until ctx is cancelled, after
// which Start drains and closes the queue and waits for in-flight work to stop.
// Run it in its own goroutine from main.
func (q *Queue) Start(ctx context.Context) {
	q.logger.Printf("inproc recognition queue: started (%d workers)", q.workers)
	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go q.worker(ctx)
	}
	<-ctx.Done()
	q.once.Do(func() { close(q.jobs) })
	q.wg.Wait()
	q.logger.Printf("inproc recognition queue: stopped")
}

func (q *Queue) worker(ctx context.Context) {
	defer q.wg.Done()
	for documentID := range q.jobs {
		// Detach from the request context (which is already done by now) but
		// honor shutdown: a fresh context tied to the worker's lifetime.
		if err := q.processor.ProcessRecognition(context.WithoutCancel(ctx), documentID); err != nil {
			// Example-only: no retry. Just log and move on.
			q.logger.Printf("inproc recognition queue: doc %d failed (dropped, no retry): %v", documentID, err)
			continue
		}
		q.logger.Printf("inproc recognition queue: doc %d done", documentID)
	}
}

// Enqueue pushes a document onto the channel. If the buffer is full this blocks
// the caller (backpressure) until a worker frees a slot or ctx is cancelled.
func (q *Queue) Enqueue(ctx context.Context, documentID int64) error {
	select {
	case q.jobs <- documentID:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

var _ ports.RecognitionEnqueuer = (*Queue)(nil)
