package receiptpipeline

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/blubaum/check-stateless-server/internal/receipt"
)

var (
	ErrReceiptPipelineNil      = errors.New("receipt pipeline is nil")
	ErrReceiptProcessorMissing = errors.New("receipt processor is required")
	ErrNoReceiptsProvided      = errors.New("at least one receipt is required")
	ErrInvalidReceiptCount     = errors.New("receipt count must be greater than zero")
)

const (
	userQueueStatePending = "pending"
	userQueueStateActive  = "active"
)

// ReceiptPipeline is the public entry point for asynchronous receipt processing.
//
// It owns queues and workers so callers do not need to know how jobs are routed,
// processed, tracked, or delivered back. This keeps HTTP handlers focused on
// request/response concerns instead of background execution details.
type ReceiptPipeline struct {
	singleQueue *ReceiptQueue
	batchQueue  *ReceiptQueue
	workerPool  *ReceiptWorkerPool

	jobIDCounter atomic.Uint64
}

type ReceiptQueuesStatsResponse struct {
	Single ReceiptQueueStats `json:"single"`
	Batch  ReceiptQueueStats `json:"batch"`
}

type ReceiptQueueStats struct {
	Name     string `json:"name"`
	Pending  int    `json:"pending"`
	Capacity int    `json:"capacity"`

	PendingJobs       int                      `json:"pending_jobs"`
	PendingUsersCount int                      `json:"pending_users_count"`
	PendingUsers      []ReceiptPendingJobStats `json:"pending_users"`

	ActiveJobs       int                     `json:"active_jobs"`
	ActiveUsersCount int                     `json:"active_users_count"`
	ActiveUsers      []ReceiptActiveJobStats `json:"active_users"`
}

type UserQueueStatusResponse struct {
	UserID    string             `json:"user_id"`
	IsQueued  bool               `json:"is_queued"`
	IsPending bool               `json:"is_pending"`
	IsActive  bool               `json:"is_active"`
	Summary   UserQueueSummary   `json:"summary"`
	Queues    []UserQueueDetails `json:"queues"`
}

type UserQueueSummary struct {
	PendingCount int `json:"pending_count"`
	ActiveCount  int `json:"active_count"`
	TotalCount   int `json:"total_count"`
}

type UserQueueDetails struct {
	Queue        string `json:"queue"`
	State        string `json:"state"`
	JobID        string `json:"job_id"`
	UserID       string `json:"user_id"`
	ReceiptCount int    `json:"receipt_count"`

	WorkerID  int    `json:"worker_id,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	StartedAt string `json:"started_at,omitempty"`
}

// NewReceiptPipeline creates a fully wired receipt processing pipeline.
//
// The constructor builds queues and the worker pool from configuration. Keeping
// this wiring inside the pipeline prevents queue ownership from being spread
// across application bootstrap code.
func NewReceiptPipeline(
	processor ReceiptProcessor,
	logger *log.Logger,
	config Config,
) (*ReceiptPipeline, error) {
	if processor == nil {
		return nil, ErrReceiptProcessorMissing
	}

	if logger == nil {
		logger = log.Default()
	}

	config = config.normalized()

	singleQueue := NewReceiptQueue(config.Queue.SingleBuffer)
	batchQueue := NewReceiptQueue(config.Queue.BatchBuffer)

	workerPool, err := NewReceiptWorkerPool(
		singleQueue,
		batchQueue,
		processor,
		logger,
		config.Workers.SingleCount,
		config.Workers.BatchCount,
	)
	if err != nil {
		return nil, fmt.Errorf("create receipt worker pool: %w", err)
	}

	return &ReceiptPipeline{
		singleQueue: singleQueue,
		batchQueue:  batchQueue,
		workerPool:  workerPool,
	}, nil
}

// Start launches background workers.
//
// The provided context controls the lifetime of all worker goroutines. When the
// context is cancelled, workers stop reading jobs and exit.
func (p *ReceiptPipeline) Start(ctx context.Context) error {
	if p == nil {
		return ErrReceiptPipelineNil
	}

	if ctx == nil {
		ctx = context.Background()
	}

	p.workerPool.Start(ctx)

	return nil
}

// Wait blocks until all worker goroutines have exited.
//
// Call this during graceful shutdown after the context passed to Start has been
// cancelled.
func (p *ReceiptPipeline) Wait() {
	if p == nil || p.workerPool == nil {
		return
	}

	p.workerPool.Wait()
}

// Close closes internal queues.
//
// In most services workers are stopped by cancelling the Start context first.
// Close is useful when shutdown code wants to unblock workers waiting on queues.
// It should be called only after the application stops accepting new submissions.
func (p *ReceiptPipeline) Close() {
	if p == nil {
		return
	}

	p.singleQueue.Close()
	p.batchQueue.Close()
}

// Submit enqueues receipts for processing and waits for the final response.
//
// Single-receipt and batch requests are routed to separate queues. This prevents
// large batch jobs from delaying smaller user-facing requests.
func (p *ReceiptPipeline) Submit(
	ctx context.Context,
	userID string,
	receipts []string,
) (receipt.ProcessReceiptsResponse, error) {
	if p == nil {
		return receipt.ProcessReceiptsResponse{}, ErrReceiptPipelineNil
	}

	if ctx == nil {
		ctx = context.Background()
	}

	receipts = normalizeReceipts(receipts)
	if len(receipts) == 0 {
		return receipt.ProcessReceiptsResponse{}, ErrNoReceiptsProvided
	}

	// The channel is buffered so a worker never blocks forever if the caller
	// stops waiting because of timeout, cancellation, or client disconnect.
	resultChan := make(chan ReceiptJobResult, 1)

	job := ReceiptJob{
		ID:       p.nextJobID(),
		UserID:   strings.TrimSpace(userID),
		Receipts: receipts,
		Context:  ctx,
		Result:   resultChan,
	}

	queue, err := p.getQueueByReceiptCount(len(receipts))
	if err != nil {
		return receipt.ProcessReceiptsResponse{}, err
	}

	if err := queue.Enqueue(ctx, job); err != nil {
		return receipt.ProcessReceiptsResponse{}, fmt.Errorf("enqueue receipt job: %w", err)
	}

	select {
	case result := <-resultChan:
		return result.Response, result.Err

	case <-ctx.Done():
		return receipt.ProcessReceiptsResponse{}, ctx.Err()
	}
}

func (p *ReceiptPipeline) GetQueuesStats() ReceiptQueuesStatsResponse {
	if p == nil {
		return ReceiptQueuesStatsResponse{}
	}

	return ReceiptQueuesStatsResponse{
		Single: p.GetSingleQueueStats(),
		Batch:  p.GetBatchQueueStats(),
	}
}

func (p *ReceiptPipeline) GetSingleQueueStats() ReceiptQueueStats {
	if p == nil || p.singleQueue == nil || p.workerPool == nil {
		return ReceiptQueueStats{Name: receiptQueueNameSingle}
	}

	return p.singleQueue.Stats(
		receiptQueueNameSingle,
		p.workerPool.ActiveSingleUsers(),
	)
}

func (p *ReceiptPipeline) GetBatchQueueStats() ReceiptQueueStats {
	if p == nil || p.batchQueue == nil || p.workerPool == nil {
		return ReceiptQueueStats{Name: receiptQueueNameBatch}
	}

	return p.batchQueue.Stats(
		receiptQueueNameBatch,
		p.workerPool.ActiveBatchUsers(),
	)
}

// GetUserQueueStatus returns pending and active jobs for a specific user.
//
// The response intentionally combines both queues because clients usually care
// about the user's overall processing state, not internal queue topology.
func (p *ReceiptPipeline) GetUserQueueStatus(userID string) UserQueueStatusResponse {
	userID = strings.TrimSpace(userID)

	response := UserQueueStatusResponse{
		UserID: userID,
		Queues: make([]UserQueueDetails, 0),
	}

	if p == nil || p.workerPool == nil || p.singleQueue == nil || p.batchQueue == nil {
		return response
	}

	response.addPendingJobs(receiptQueueNameSingle, p.singleQueue.PendingUsers(), userID)
	response.addPendingJobs(receiptQueueNameBatch, p.batchQueue.PendingUsers(), userID)

	response.addActiveJobs(receiptQueueNameSingle, p.workerPool.ActiveSingleUsers(), userID)
	response.addActiveJobs(receiptQueueNameBatch, p.workerPool.ActiveBatchUsers(), userID)

	response.Summary.TotalCount = response.Summary.PendingCount + response.Summary.ActiveCount
	response.IsPending = response.Summary.PendingCount > 0
	response.IsActive = response.Summary.ActiveCount > 0
	response.IsQueued = response.Summary.TotalCount > 0

	return response
}

func (r *UserQueueStatusResponse) addPendingJobs(
	queueName string,
	jobs []ReceiptPendingJobStats,
	userID string,
) {
	if r == nil {
		return
	}

	for _, job := range jobs {
		if job.UserID != userID {
			continue
		}

		r.Summary.PendingCount++

		r.Queues = append(r.Queues, UserQueueDetails{
			Queue:        queueName,
			State:        userQueueStatePending,
			JobID:        job.JobID,
			UserID:       job.UserID,
			ReceiptCount: job.ReceiptCount,
			CreatedAt:    job.CreatedAt,
		})
	}
}

func (r *UserQueueStatusResponse) addActiveJobs(
	queueName string,
	jobs []ReceiptActiveJobStats,
	userID string,
) {
	if r == nil {
		return
	}

	for _, job := range jobs {
		if job.UserID != userID {
			continue
		}

		r.Summary.ActiveCount++

		r.Queues = append(r.Queues, UserQueueDetails{
			Queue:        queueName,
			State:        userQueueStateActive,
			JobID:        job.JobID,
			UserID:       job.UserID,
			ReceiptCount: job.ReceiptCount,
			WorkerID:     job.WorkerID,
			StartedAt:    job.StartedAt,
		})
	}
}

func (p *ReceiptPipeline) getQueueByReceiptCount(receiptCount int) (*ReceiptQueue, error) {
	if receiptCount <= 0 {
		return nil, ErrInvalidReceiptCount
	}

	if receiptCount == 1 {
		return p.singleQueue, nil
	}

	return p.batchQueue, nil
}

func (p *ReceiptPipeline) nextJobID() string {
	nextID := p.jobIDCounter.Add(1)

	return "job-" + strconv.FormatUint(nextID, 10)
}

func normalizeReceipts(receipts []string) []string {
	if len(receipts) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(receipts))

	for _, value := range receipts {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		normalized = append(normalized, value)
	}

	return normalized
}
