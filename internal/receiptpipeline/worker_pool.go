package receiptpipeline

import (
	"context"
	"errors"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/blubaum/check-stateless-server/internal/receipt"
)

var (
	ErrSingleReceiptQueueRequired = errors.New("single receipt queue is required")
	ErrBatchReceiptQueueRequired  = errors.New("batch receipt queue is required")
)

// ReceiptProcessor is the minimal processing dependency required by workers.
//
// The worker pool depends on an interface to stay independent from the concrete
// receipt implementation. This lets processing logic evolve without changing
// queue or worker orchestration code.
type ReceiptProcessor interface {
	Process(ctx context.Context, receipts []string) ([]receipt.ProcessReceiptResult, error)
}

// ReceiptWorkerPool manages background workers that consume receipt jobs.
//
// Single and batch queues are processed by separate worker groups. This protects
// low-latency single-receipt requests from being blocked by larger batch jobs.
type ReceiptWorkerPool struct {
	singleQueue *ReceiptQueue
	batchQueue  *ReceiptQueue

	processor ReceiptProcessor
	logger    *log.Logger

	singleWorkers int
	batchWorkers  int

	activeMu     sync.RWMutex
	activeSingle map[string]ReceiptActiveJobStats
	activeBatch  map[string]ReceiptActiveJobStats

	wg sync.WaitGroup
}

type ReceiptActiveJobStats struct {
	JobID        string `json:"job_id"`
	UserID       string `json:"user_id"`
	ReceiptCount int    `json:"receipt_count"`
	WorkerID     int    `json:"worker_id"`
	StartedAt    string `json:"started_at"`
}

const (
	receiptQueueNameSingle = "single"
	receiptQueueNameBatch  = "batch"
)

// NewReceiptWorkerPool creates a configured worker pool.
//
// Required dependencies are validated early so application startup fails fast
// instead of creating a pipeline that accepts jobs but cannot process them.
func NewReceiptWorkerPool(
	singleQueue *ReceiptQueue,
	batchQueue *ReceiptQueue,
	processor ReceiptProcessor,
	logger *log.Logger,
	singleWorkers int,
	batchWorkers int,
) (*ReceiptWorkerPool, error) {
	if singleQueue == nil {
		return nil, ErrSingleReceiptQueueRequired
	}

	if batchQueue == nil {
		return nil, ErrBatchReceiptQueueRequired
	}

	if processor == nil {
		return nil, ErrReceiptProcessorMissing
	}

	if logger == nil {
		logger = log.Default()
	}

	return &ReceiptWorkerPool{
		singleQueue:   singleQueue,
		batchQueue:    batchQueue,
		processor:     processor,
		logger:        logger,
		singleWorkers: normalizeWorkerCount(singleWorkers),
		batchWorkers:  normalizeWorkerCount(batchWorkers),
		activeSingle:  make(map[string]ReceiptActiveJobStats),
		activeBatch:   make(map[string]ReceiptActiveJobStats),
	}, nil
}

// Start launches all configured workers.
//
// Workers exit when the application context is cancelled or when their queue is
// closed during shutdown.
func (p *ReceiptWorkerPool) Start(ctx context.Context) {
	if p == nil {
		return
	}

	if ctx == nil {
		ctx = context.Background()
	}

	p.startWorkers(ctx, receiptQueueNameSingle, p.singleQueue, p.singleWorkers)
	p.startWorkers(ctx, receiptQueueNameBatch, p.batchQueue, p.batchWorkers)
}

// Wait blocks until all started workers exit.
//
// This is used during graceful shutdown to avoid leaking goroutines.
func (p *ReceiptWorkerPool) Wait() {
	if p == nil {
		return
	}

	p.wg.Wait()
}

func (p *ReceiptWorkerPool) ActiveSingleUsers() []ReceiptActiveJobStats {
	return p.activeUsersByQueue(receiptQueueNameSingle)
}

func (p *ReceiptWorkerPool) ActiveBatchUsers() []ReceiptActiveJobStats {
	return p.activeUsersByQueue(receiptQueueNameBatch)
}

func (p *ReceiptWorkerPool) startWorkers(
	ctx context.Context,
	queueName string,
	queue *ReceiptQueue,
	workerCount int,
) {
	if queue == nil {
		return
	}

	for workerID := 1; workerID <= workerCount; workerID++ {
		p.wg.Add(1)

		go func(id int) {
			defer p.wg.Done()
			p.runWorker(ctx, queueName, id, queue)
		}(workerID)
	}
}

func (p *ReceiptWorkerPool) runWorker(
	ctx context.Context,
	queueName string,
	workerID int,
	queue *ReceiptQueue,
) {
	defer func() {
		if recovered := recover(); recovered != nil {
			p.logger.Printf(
				"receipt worker recovered from panic queue=%s worker=%d panic=%v",
				queueName,
				workerID,
				recovered,
			)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return

		case job, ok := <-queue.Jobs():
			if !ok {
				return
			}

			queue.MarkDequeued(job)
			p.processJob(queueName, workerID, job)
		}
	}
}

// processJob executes the business processing step and delivers the result back
// to the caller.
//
// Result delivery is intentionally non-blocking. If the original request was
// cancelled, the caller may no longer be waiting on the channel. Blocking here
// would leak a worker goroutine and eventually stall the whole pool.
func (p *ReceiptWorkerPool) processJob(
	queueName string,
	workerID int,
	job ReceiptJob,
) {
	p.markActiveJob(queueName, workerID, job)
	defer p.removeActiveJob(queueName, job.ID)

	jobCtx := job.Context
	if jobCtx == nil {
		jobCtx = context.Background()
	}

	results, err := p.processor.Process(jobCtx, job.Receipts)

	response := receipt.ProcessReceiptsResponse{
		UserID:  job.UserID,
		Count:   len(results),
		Results: results,
	}

	p.sendResult(job.Result, ReceiptJobResult{
		Response: response,
		Err:      err,
	})
}

func (p *ReceiptWorkerPool) sendResult(
	resultChan chan<- ReceiptJobResult,
	result ReceiptJobResult,
) {
	select {
	case resultChan <- result:
	default:
		p.logger.Printf("receipt job result was dropped because caller is no longer waiting")
	}
}

func (p *ReceiptWorkerPool) markActiveJob(
	queueName string,
	workerID int,
	job ReceiptJob,
) {
	p.activeMu.Lock()
	defer p.activeMu.Unlock()

	activeJob := ReceiptActiveJobStats{
		JobID:        job.ID,
		UserID:       job.UserID,
		ReceiptCount: len(job.Receipts),
		WorkerID:     workerID,
		StartedAt:    time.Now().Format(time.RFC3339),
	}

	switch queueName {
	case receiptQueueNameSingle:
		p.activeSingle[job.ID] = activeJob

	case receiptQueueNameBatch:
		p.activeBatch[job.ID] = activeJob
	}
}

func (p *ReceiptWorkerPool) removeActiveJob(queueName string, jobID string) {
	p.activeMu.Lock()
	defer p.activeMu.Unlock()

	switch queueName {
	case receiptQueueNameSingle:
		delete(p.activeSingle, jobID)

	case receiptQueueNameBatch:
		delete(p.activeBatch, jobID)
	}
}

func (p *ReceiptWorkerPool) activeUsersByQueue(queueName string) []ReceiptActiveJobStats {
	if p == nil {
		return nil
	}

	p.activeMu.RLock()
	defer p.activeMu.RUnlock()

	var source map[string]ReceiptActiveJobStats

	switch queueName {
	case receiptQueueNameSingle:
		source = p.activeSingle

	case receiptQueueNameBatch:
		source = p.activeBatch

	default:
		return nil
	}

	result := make([]ReceiptActiveJobStats, 0, len(source))

	for _, activeJob := range source {
		result = append(result, activeJob)
	}

	// Map iteration order is random in Go. Sorting keeps API responses stable,
	// which makes frontend rendering, tests and debugging easier.
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].StartedAt < result[j].StartedAt
	})

	return result
}

func normalizeWorkerCount(value int) int {
	if value < 1 {
		return defaultWorkerCount
	}

	return value
}
