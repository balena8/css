package server

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/blubaum/check-stateless-server/internal/config"
)

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// WithRequestLogging logs request method, path, status code and duration.
//
// The middleware intentionally does not log query strings because receipt routes
// may contain user IDs or other request-specific identifiers.
func WithRequestLogging(next http.Handler, logger *log.Logger) http.Handler {
	if logger == nil {
		logger = log.Default()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		recorder := newStatusRecorder(w)

		next.ServeHTTP(recorder, r)

		logger.Printf(
			"%s %s status=%d completed_in=%s",
			r.Method,
			r.URL.Path,
			recorder.statusCode,
			time.Since(startedAt),
		)
	})
}

// WithCORS applies CORS headers from config.
//
// When credentials are needed in the future, do not use "*" for origins.
// Browsers reject wildcard origins with credentials, and reflecting arbitrary
// origins without validation would be unsafe.
func WithCORS(next http.Handler, cfg config.CORSConfig) http.Handler {
	allowedOrigins := strings.Join(cfg.AllowedOrigins, ", ")
	allowedMethods := strings.Join(cfg.AllowedMethods, ", ")
	allowedHeaders := strings.Join(cfg.AllowedHeaders, ", ")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if allowedOrigins != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigins)
		}

		if allowedMethods != "" {
			w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
		}

		if allowedHeaders != "" {
			w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
