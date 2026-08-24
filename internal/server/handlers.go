package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/blubaum/check-stateless-server/internal/receipt"
	"github.com/blubaum/check-stateless-server/internal/receiptpipeline"
)

const maxReceiptRequestBodySize = 1 << 20 // 1 MB

var (
	ErrReceiptPipelineRequired = errors.New("receipt pipeline is required")
	ErrUserIDRequired          = errors.New("user_id is required")
	ErrQRCodeRequired          = errors.New("qr_code is required")
	ErrQRCodesRequired         = errors.New("qr_codes must contain at least one QR code")
)

// ReceiptPipeline describes only the behavior required by HTTP handlers.
//
// The handler should not know whether processing is done directly, through
// queues, workers, or another transport. This keeps HTTP delivery independent
// from the processing implementation.
type ReceiptPipeline interface {
	Submit(ctx context.Context, userID string, receipts []string) (receipt.ProcessReceiptsResponse, error)

	GetQueuesStats() receiptpipeline.ReceiptQueuesStatsResponse
	GetSingleQueueStats() receiptpipeline.ReceiptQueueStats
	GetBatchQueueStats() receiptpipeline.ReceiptQueueStats
	GetUserQueueStatus(userID string) receiptpipeline.UserQueueStatusResponse
}

type ReceiptHandler struct {
	pipeline ReceiptPipeline
	logger   *log.Logger
}

func NewReceiptHandler(
	pipeline ReceiptPipeline,
	logger *log.Logger,
) (*ReceiptHandler, error) {
	if pipeline == nil {
		return nil, ErrReceiptPipelineRequired
	}

	if logger == nil {
		logger = log.Default()
	}

	return &ReceiptHandler{
		pipeline: pipeline,
		logger:   logger,
	}, nil
}

type parseReceiptRequest struct {
	UserID string `json:"user_id"`
	QRCode string `json:"qr_code"`
}

type parseReceiptBatchRequest struct {
	UserID  string   `json:"user_id"`
	QRCodes []string `json:"qr_codes"`
}

// ParseReceipt handles a single receipt QR code.
//
// Single parsing has its own endpoint because clients often use it for fast
// user-facing flows where latency matters more than throughput.
func (h *ReceiptHandler) ParseReceipt(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var request parseReceiptRequest
	if err := decodeJSONRequest(w, r, &request, maxReceiptRequestBodySize); err != nil {
		writeJSONError(w, http.StatusBadRequest, errorCodeInvalidRequest, err.Error())
		return
	}

	request.normalize()

	if err := request.validate(); err != nil {
		writeJSONError(w, http.StatusBadRequest, errorCodeInvalidRequest, err.Error())
		return
	}

	response, err := h.pipeline.Submit(
		r.Context(),
		request.UserID,
		[]string{request.QRCode},
	)
	if err != nil {
		h.writePipelineError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

// ParseReceiptBatch handles multiple receipt QR codes in one request.
//
// Batch parsing still returns per-receipt results. The processing layer decides
// whether each QR code succeeds or fails, so one invalid receipt does not have
// to fail the whole batch request.
func (h *ReceiptHandler) ParseReceiptBatch(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var request parseReceiptBatchRequest
	if err := decodeJSONRequest(w, r, &request, maxReceiptRequestBodySize); err != nil {
		writeJSONError(w, http.StatusBadRequest, errorCodeInvalidRequest, err.Error())
		return
	}

	request.normalize()

	if err := request.validate(); err != nil {
		writeJSONError(w, http.StatusBadRequest, errorCodeInvalidRequest, err.Error())
		return
	}

	response, err := h.pipeline.Submit(
		r.Context(),
		request.UserID,
		request.QRCodes,
	)
	if err != nil {
		h.writePipelineError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

// GetReceiptQueues returns stats for both receipt queues.
func (h *ReceiptHandler) GetReceiptQueues(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	writeJSON(w, http.StatusOK, h.pipeline.GetQueuesStats())
}

// GetSingleReceiptQueue returns stats for the single-receipt queue.
func (h *ReceiptHandler) GetSingleReceiptQueue(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	writeJSON(w, http.StatusOK, h.pipeline.GetSingleQueueStats())
}

// GetBatchReceiptQueue returns stats for the batch-receipt queue.
func (h *ReceiptHandler) GetBatchReceiptQueue(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	writeJSON(w, http.StatusOK, h.pipeline.GetBatchQueueStats())
}

// GetReceiptUserQueueStatus returns queue status for one user.
//
// It checks both single and batch queues, including jobs waiting in queue and
// jobs currently processed by workers.
func (h *ReceiptHandler) GetReceiptUserQueueStatus(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		writeJSONError(w, http.StatusBadRequest, errorCodeInvalidRequest, "user_id query parameter is required")
		return
	}

	writeJSON(w, http.StatusOK, h.pipeline.GetUserQueueStatus(userID))
}

func (r *parseReceiptRequest) normalize() {
	if r == nil {
		return
	}

	r.UserID = strings.TrimSpace(r.UserID)
	r.QRCode = strings.TrimSpace(r.QRCode)
}

func (r parseReceiptRequest) validate() error {
	if strings.TrimSpace(r.UserID) == "" {
		return ErrUserIDRequired
	}

	if strings.TrimSpace(r.QRCode) == "" {
		return ErrQRCodeRequired
	}

	return nil
}

func (r *parseReceiptBatchRequest) normalize() {
	if r == nil {
		return
	}

	r.UserID = strings.TrimSpace(r.UserID)

	for index := range r.QRCodes {
		r.QRCodes[index] = strings.TrimSpace(r.QRCodes[index])
	}
}

func (r parseReceiptBatchRequest) validate() error {
	if strings.TrimSpace(r.UserID) == "" {
		return ErrUserIDRequired
	}

	if len(r.QRCodes) == 0 {
		return ErrQRCodesRequired
	}

	for index, qrCode := range r.QRCodes {
		if strings.TrimSpace(qrCode) == "" {
			return fmt.Errorf("qr_codes[%d] must not be empty", index)
		}
	}

	return nil
}

func (h *ReceiptHandler) writePipelineError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, receiptpipeline.ErrNoReceiptsProvided):
		writeJSONError(w, http.StatusBadRequest, errorCodeInvalidRequest, err.Error())

	case errors.Is(err, context.Canceled):
		writeJSONError(w, http.StatusRequestTimeout, errorCodeRequestCanceled, "request was cancelled")

	case errors.Is(err, context.DeadlineExceeded):
		writeJSONError(w, http.StatusGatewayTimeout, errorCodeRequestTimeout, "receipt processing timed out")

	default:
		// Do not expose the internal error to clients. It can contain external API
		// details, queue state, provider errors, or infrastructure messages.
		h.logger.Printf("receipt processing failed: %v", err)
		writeJSONError(w, http.StatusInternalServerError, errorCodeReceiptProcessing, "receipt processing failed")
	}
}
