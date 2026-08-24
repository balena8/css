package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/blubaum/check-stateless-server/internal/config"
	"github.com/blubaum/check-stateless-server/internal/enrichment"
	"github.com/blubaum/check-stateless-server/internal/receipt"
	"github.com/blubaum/check-stateless-server/internal/receiptpipeline"
	"github.com/blubaum/check-stateless-server/internal/server"
	"github.com/blubaum/check-stateless-server/llm/factory"
)

const shutdownTimeout = 30 * time.Second

func main() {
	if err := Run(); err != nil {
		log.Fatalf("application failed: %v", err)
	}
}

// Run wires the application and owns the top-level lifecycle.
//
// The main package is intentionally responsible only for composition:
// loading config, creating dependencies, starting background workers, starting
// HTTP delivery, and coordinating graceful shutdown.
func Run() error {
	configPath := flag.String("config", "", "path to YAML config file")
	flag.Parse()

	logger := newApplicationLogger()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	shutdownCtx, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	receiptPipeline, err := buildReceiptPipeline(cfg, logger)
	if err != nil {
		return fmt.Errorf("build receipt pipeline: %w", err)
	}

	// The pipeline gets its own lifecycle context instead of using shutdownCtx
	// directly. On SIGTERM we first stop HTTP traffic, then close queues and let
	// workers drain accepted jobs. Cancelling workers immediately could drop work
	// that was already accepted by the service.
	pipelineCtx, stopPipeline := context.WithCancel(context.Background())
	defer stopPipeline()

	if err := receiptPipeline.Start(pipelineCtx); err != nil {
		return fmt.Errorf("start receipt pipeline: %w", err)
	}

	defer func() {
		receiptPipeline.Close()
		stopPipeline()
		receiptPipeline.Wait()
	}()

	httpServer, err := server.NewServer(
		cfg,
		logger,
		receiptPipeline,
	)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	serverErrors := make(chan error, 1)

	go func() {
		serverErrors <- httpServer.Start()
	}()

	select {
	case <-shutdownCtx.Done():
		logger.Printf("shutdown signal received")

		if err := shutdownApplication(httpServer, receiptPipeline, logger); err != nil {
			return err
		}

		return nil

	case err := <-serverErrors:
		if errors.Is(err, server.ErrServerClosed) {
			return nil
		}

		return fmt.Errorf("start server: %w", err)
	}
}

func newApplicationLogger() *log.Logger {
	return log.New(
		os.Stdout,
		"[check-stateless-server] ",
		log.LstdFlags|log.Lmsgprefix,
	)
}

// buildReceiptPipeline wires receipt processing dependencies.
//
// The pipeline itself owns queues/workers. The main package only provides the
// domain processor and configuration values required to size the pipeline.
func buildReceiptPipeline(
	cfg *config.Config,
	logger *log.Logger,
) (*receiptpipeline.ReceiptPipeline, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}

	if logger == nil {
		logger = log.Default()
	}

	receiptEnricher, err := buildReceiptEnricher(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("build receipt enricher: %w", err)
	}

	apiClient := receipt.NewReceiptAPIClient(nil)

	receiptProcessor := receipt.NewReceiptProcessorWithDependencies(
		apiClient,
		logger,
		receiptEnricher,
	)

	pipelineConfig := receiptpipeline.Config{
		Queue: receiptpipeline.QueueConfig{
			SingleBuffer: cfg.Receipt.Processing.Queue.SingleBuffer,
			BatchBuffer:  cfg.Receipt.Processing.Queue.BatchBuffer,
		},
		Workers: receiptpipeline.WorkerConfig{
			SingleCount: cfg.Receipt.Processing.Workers.SingleWorkers,
			BatchCount:  cfg.Receipt.Processing.Workers.BatchWorkers,
		},
	}

	receiptPipeline, err := receiptpipeline.NewReceiptPipeline(
		receiptProcessor,
		logger,
		pipelineConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("create receipt pipeline: %w", err)
	}

	return receiptPipeline, nil
}

// buildReceiptEnricher creates the optional LLM enrichment component.
//
// Returning nil when enrichment is disabled is intentional. Receipt processing
// should still work without LLM enrichment, and the processor already treats a
// nil enricher as "parse only".
func buildReceiptEnricher(
	cfg *config.Config,
	logger *log.Logger,
) (*enrichment.ReceiptLLMEnricher, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}

	if logger == nil {
		logger = log.Default()
	}

	if !cfg.Receipt.Enrichment.Enabled {
		logger.Printf("receipt enrichment is disabled")
		return nil, nil
	}

	loadedKeys, err := loadReceiptEnrichmentKeys(cfg)
	if err != nil {
		return nil, err
	}

	completionProvider, err := factory.NewCompletionProvider(
		cfg.Receipt.Enrichment.Provider,
		cfg.Receipt.Enrichment.Model,
		cfg.Receipt.Enrichment.BaseURL,
		cfg.Receipt.Enrichment.Headers,
		loadedKeys,
	)
	if err != nil {
		return nil, fmt.Errorf("create receipt enrichment completion provider: %w", err)
	}

	receiptEnricher, err := enrichment.NewReceiptLLMEnricher(
		completionProvider,
		enrichment.ReceiptLLMEnricherConfig{
			Options:      enrichment.DefaultReceiptEnrichmentOptions(),
			SystemPrompt: cfg.Receipt.Enrichment.SystemPrompt,
			TokenBudget:  cfg.Receipt.Enrichment.TokenBudget,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create receipt llm enricher: %w", err)
	}

	logger.Printf(
		"receipt enrichment enabled provider=%s model=%s",
		cfg.Receipt.Enrichment.Provider,
		cfg.Receipt.Enrichment.Model,
	)

	return receiptEnricher, nil
}

// loadReceiptEnrichmentKeys loads provider credentials during startup.
//
// Failing fast is better than starting the server and discovering missing API
// keys only after the first user request reaches enrichment.
func loadReceiptEnrichmentKeys(cfg *config.Config) (*config.LoadedKeys, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}

	if !cfg.Receipt.Enrichment.Enabled {
		return &config.LoadedKeys{}, nil
	}

	keyLoader := config.NewAPIKeyLoader(cfg.APIKeys)

	loadedKeys, err := keyLoader.LoadKeysForReceiptEnrichment(
		cfg.Receipt.Enrichment,
	)
	if err != nil {
		return nil, fmt.Errorf("load receipt enrichment api keys: %w", err)
	}

	return loadedKeys, nil
}

// shutdownApplication coordinates graceful application shutdown.
//
// Order matters:
//  1. stop accepting new HTTP requests;
//  2. let in-flight handlers finish;
//  3. close receipt queues;
//  4. wait for workers to exit.
//
// Closing queues before HTTP shutdown can race with handlers that are still
// trying to enqueue work.
func shutdownApplication(
	httpServer *server.Server,
	receiptPipeline *receiptpipeline.ReceiptPipeline,
	logger *log.Logger,
) error {
	if logger == nil {
		logger = log.Default()
	}

	if err := shutdownServer(httpServer, logger); err != nil {
		return err
	}

	if receiptPipeline != nil {
		receiptPipeline.Close()

		if err := waitForReceiptPipeline(receiptPipeline, shutdownTimeout); err != nil {
			return err
		}
	}

	logger.Printf("application stopped gracefully")

	return nil
}

func shutdownServer(
	httpServer *server.Server,
	logger *log.Logger,
) error {
	if httpServer == nil {
		return nil
	}

	if logger == nil {
		logger = log.Default()
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Printf("graceful HTTP shutdown failed: %v", err)

		if closeErr := httpServer.Close(); closeErr != nil {
			return fmt.Errorf(
				"force close HTTP server after shutdown error %v: %w",
				err,
				closeErr,
			)
		}

		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	logger.Printf("HTTP server stopped gracefully")

	return nil
}

func waitForReceiptPipeline(
	receiptPipeline *receiptpipeline.ReceiptPipeline,
	timeout time.Duration,
) error {
	if receiptPipeline == nil {
		return nil
	}

	done := make(chan struct{})

	go func() {
		defer close(done)
		receiptPipeline.Wait()
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-done:
		return nil

	case <-timer.C:
		return fmt.Errorf("receipt pipeline shutdown timed out after %s", timeout)
	}
}
