package server

import (
	"errors"
	"log"
	"net/http"

	"github.com/blubaum/check-stateless-server/internal/config"
	"github.com/blubaum/check-stateless-server/internal/receiptpipeline"
	"github.com/blubaum/check-stateless-server/internal/server/openapi"
)

var (
	ErrConfigRequired = errors.New("server config is required")
)

// NewRouter wires HTTP routes to handlers.
//
// Router construction returns an error instead of panicking. Panics during server
// bootstrap make tests and application startup harder to control.
func NewRouter(
	cfg *config.Config,
	logger *log.Logger,
	receiptPipeline *receiptpipeline.ReceiptPipeline,
) (http.Handler, error) {
	if cfg == nil {
		return nil, ErrConfigRequired
	}

	if logger == nil {
		logger = log.Default()
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", handleHealthCheck)
	mux.HandleFunc("/openapi.json", handleOpenAPISpec)

	receiptHandler, err := NewReceiptHandler(receiptPipeline, logger)
	if err != nil {
		return nil, err
	}

	mux.HandleFunc("/receipts/parse", receiptHandler.ParseReceipt)
	mux.HandleFunc("/receipts/parse/batch", receiptHandler.ParseReceiptBatch)

	mux.HandleFunc("/receipts/queues", receiptHandler.GetReceiptQueues)
	mux.HandleFunc("/receipts/queues/single", receiptHandler.GetSingleReceiptQueue)
	mux.HandleFunc("/receipts/queues/batch", receiptHandler.GetBatchReceiptQueue)
	mux.HandleFunc("/receipts/queues/user", receiptHandler.GetReceiptUserQueueStatus)

	var router http.Handler = mux

	if cfg.Server.CORS.Enabled {
		router = WithCORS(router, cfg.Server.CORS)
	}

	router = WithRequestLogging(router, logger)

	return router, nil
}

func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

// handleOpenAPISpec serves the generated OpenAPI document.
//
// The spec is built from the same route contract used by the server package,
// so clients can discover the API surface without relying on a separate static
// file that may become outdated.
func handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	writeJSON(w, http.StatusOK, openapi.BuildOpenAPISpec())
}
