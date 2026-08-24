package receiptpipeline

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrReceiptQueueNil    = errors.New("receipt queue is nil")
	ErrReceiptJobIDEmpty  = errors.New("receipt job id is required")
	ErrReceiptJobCanceled = errors.New("receipt job enqueue canceled")
)

// ReceiptQueue is a bounded in-memory queue for receipt processing jobs.
//
// A bounded channel gives the service natural backpressure under load. When the
// queue is full, Enqueue waits until a worker consumes a job or the caller's
// context is cancelled.
type ReceiptQueue struct {
	jobs chan ReceiptJob

	closeOnce sync.Once

	mu           sync.RWMutex
	pendingJobs  map[string]ReceiptPendingJobStats
	pendingOrder []string
}

// ReceiptPendingJobStats is a user-facing snapshot of jobs waiting in the queue.
//
// This is intentionally separate from ReceiptJob. The job contains runtime fields
// such as context and result channels that must never be exposed through API responses.
type ReceiptPendingJobStats struct {
	JobID        string `json:"job_id"`
	UserID       string `json:"user_id"`
	ReceiptCount int    `json:"receipt_count"`
	CreatedAt    string `json:"created_at"`
}

// NewReceiptQueue creates a queue with a safe minimum capacity.
//
// A non-positive buffer size is normalized to 1 so the pipeline cannot be
// accidentally configured with an unusable queue.
func NewReceiptQueue(bufferSize int) *ReceiptQueue {
	if bufferSize < 1 {
		bufferSize = defaultQueueBufferSize
	}

	return &ReceiptQueue{
		jobs:         make(chan ReceiptJob, bufferSize),
		pendingJobs:  make(map[string]ReceiptPendingJobStats),
		pendingOrder: make([]string, 0, bufferSize),
	}
}

// Enqueue adds a job to the queue while respecting caller cancellation.
//
// The job is added to pending stats before sending to the channel because from
// the API perspective the job is already waiting once Enqueue starts blocking
// on a full queue. If the context is cancelled before the job is accepted, the
// pending record is removed.
func (q *ReceiptQueue) Enqueue(ctx context.Context, job ReceiptJob) error {
	if q == nil {
		return ErrReceiptQueueNil
	}

	if ctx == nil {
		ctx = context.Background()
	}

	if job.ID == "" {
		return ErrReceiptJobIDEmpty
	}

	job.CreatedAt = time.Now()
	q.addPendingJob(job)

	select {
	case q.jobs <- job:
		return nil

	case <-ctx.Done():
		q.removePendingJob(job.ID)
		return errors.Join(ErrReceiptJobCanceled, ctx.Err())
	}
}

// Jobs exposes the queue as a read-only stream for workers.
//
// External code cannot push jobs directly into the queue, so enqueue behavior,
// pending tracking, and cancellation handling remain centralized in Enqueue.
func (q *ReceiptQueue) Jobs() <-chan ReceiptJob {
	if q == nil {
		return nil
	}

	return q.jobs
}

// MarkDequeued removes a job from pending stats when a worker takes it.
//
// The job may still be processing after this call. Active processing is tracked
// separately by the worker pool.
func (q *ReceiptQueue) MarkDequeued(job ReceiptJob) {
	if q == nil {
		return
	}

	q.removePendingJob(job.ID)
}

// PendingUsers returns a stable snapshot of pending jobs.
//
// A copy is returned so callers can safely sort, filter, or serialize the result
// without mutating queue internals.
func (q *ReceiptQueue) PendingUsers() []ReceiptPendingJobStats {
	if q == nil {
		return nil
	}

	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]ReceiptPendingJobStats, 0, len(q.pendingOrder))

	for _, jobID := range q.pendingOrder {
		job, exists := q.pendingJobs[jobID]
		if !exists {
			continue
		}

		result = append(result, job)
	}

	return result
}

// Stats returns queue state visible to API clients.
//
// Pending users are jobs still waiting in the channel. Active users are jobs
// already taken by workers and currently being processed.
func (q *ReceiptQueue) Stats(
	name string,
	activeUsers []ReceiptActiveJobStats,
) ReceiptQueueStats {
	if q == nil {
		return ReceiptQueueStats{Name: name}
	}

	pendingUsers := q.PendingUsers()

	return ReceiptQueueStats{
		Name:              name,
		Pending:           len(q.jobs),
		Capacity:          cap(q.jobs),
		PendingJobs:       len(pendingUsers),
		PendingUsersCount: len(pendingUsers),
		PendingUsers:      pendingUsers,
		ActiveJobs:        len(activeUsers),
		ActiveUsersCount:  len(activeUsers),
		ActiveUsers:       activeUsers,
	}
}

// Close stops the queue by closing the underlying job channel.
//
// This method is idempotent, but it should still only be called when the
// application has stopped accepting new requests. Sending to a closed channel
// panics in Go, so queue lifecycle must be owned by the application shutdown flow.
func (q *ReceiptQueue) Close() {
	if q == nil {
		return
	}

	q.closeOnce.Do(func() {
		close(q.jobs)
	})
}

func (q *ReceiptQueue) addPendingJob(job ReceiptJob) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if _, exists := q.pendingJobs[job.ID]; !exists {
		q.pendingOrder = append(q.pendingOrder, job.ID)
	}

	q.pendingJobs[job.ID] = ReceiptPendingJobStats{
		JobID:        job.ID,
		UserID:       job.UserID,
		ReceiptCount: len(job.Receipts),
		CreatedAt:    job.CreatedAt.Format(time.RFC3339),
	}
}

func (q *ReceiptQueue) removePendingJob(jobID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if _, exists := q.pendingJobs[jobID]; !exists {
		return
	}

	delete(q.pendingJobs, jobID)

	for index, pendingJobID := range q.pendingOrder {
		if pendingJobID == jobID {
			q.pendingOrder = append(q.pendingOrder[:index], q.pendingOrder[index+1:]...)
			return
		}
	}
}
