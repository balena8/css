package receiptpipeline

import (
	"context"
	"time"

	"github.com/blubaum/check-stateless-server/internal/receipt"
)

// ReceiptJob represents one unit of work submitted to the pipeline.
//
// The job carries the original request context so cancellation can propagate
// from the HTTP layer down to external API calls made by the receipt processor.
type ReceiptJob struct {
	ID string

	UserID   string
	Receipts []string

	Context context.Context

	// CreatedAt is assigned when the queue accepts the job.
	// It is useful for queue latency metrics, diagnostics, and user-facing
	// queue status responses.
	CreatedAt time.Time

	// Result is send-only from the worker perspective.
	// The caller owns receiving from this channel.
	Result chan<- ReceiptJobResult
}

// ReceiptJobResult is sent back to the caller after processing is finished.
type ReceiptJobResult struct {
	Response receipt.ProcessReceiptsResponse
	Err      error
}
