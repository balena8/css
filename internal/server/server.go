package server

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/blubaum/check-stateless-server/internal/config"
	"github.com/blubaum/check-stateless-server/internal/receiptpipeline"
)

var ErrServerClosed = http.ErrServerClosed

type Server struct {
	httpServer *http.Server
	logger     *log.Logger
}

func NewServer(
	cfg *config.Config,
	logger *log.Logger,
	receiptPipeline *receiptpipeline.ReceiptPipeline,
) (*Server, error) {
	if cfg == nil {
		return nil, ErrConfigRequired
	}

	if logger == nil {
		logger = log.Default()
	}

	router, err := NewRouter(cfg, logger, receiptPipeline)
	if err != nil {
		return nil, err
	}

	return &Server{
		httpServer: &http.Server{
			Addr:         cfg.ServerAddress(),
			Handler:      router,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
			IdleTimeout:  cfg.Server.IdleTimeout,
		},
		logger: logger,
	}, nil
}

// Start starts the HTTP server and normalizes graceful shutdown errors.
//
// http.ErrServerClosed is not a real failure during controlled shutdown, so the
// method returns ErrServerClosed explicitly to let the application entrypoint
// decide whether to ignore or log it.
func (s *Server) Start() error {
	if s == nil || s.httpServer == nil {
		return errors.New("server is not initialized")
	}

	s.logger.Printf("server listening on %s", s.httpServer.Addr)

	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return ErrServerClosed
	}

	return err
}

// Shutdown gracefully stops the HTTP server.
//
// Existing in-flight requests are allowed to finish until the provided context
// expires. This should be used during normal application shutdown.
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.httpServer == nil {
		return nil
	}

	return s.httpServer.Shutdown(ctx)
}

// Close immediately closes the HTTP server.
//
// Prefer Shutdown for normal lifecycle management. Close is useful for emergency
// stop paths or tests where graceful draining is not needed.
func (s *Server) Close() error {
	if s == nil || s.httpServer == nil {
		return nil
	}

	return s.httpServer.Close()
}
