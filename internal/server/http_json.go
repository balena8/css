package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

var (
	ErrRequestBodyRequired = errors.New("request body is required")
	ErrSingleJSONRequired  = errors.New("request body must contain a single JSON object")
)

// decodeJSONRequest decodes a strict JSON request body.
//
// Unknown fields are rejected intentionally. Silent acceptance of misspelled
// fields makes API clients think an option was applied when the backend actually
// ignored it.
func decodeJSONRequest(
	w http.ResponseWriter,
	r *http.Request,
	destination any,
	maxBodySize int64,
) error {
	if r == nil || r.Body == nil {
		return ErrRequestBodyRequired
	}

	if destination == nil {
		return errors.New("decode destination is required")
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		if errors.Is(err, io.EOF) {
			return ErrRequestBodyRequired
		}

		return fmt.Errorf("invalid JSON request body: %w", err)
	}

	// Decode once more to ensure the body does not contain multiple JSON values:
	// `{...}{...}` should be rejected instead of partially accepted.
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return ErrSingleJSONRequired
	} else if !errors.Is(err, io.EOF) {
		return fmt.Errorf("invalid JSON request body: %w", err)
	}

	return nil
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)

	// There is not much useful recovery if response encoding fails after headers
	// were written. Logging belongs to middleware/server-level observability.
	_ = encoder.Encode(payload)
}

func writeJSONError(w http.ResponseWriter, statusCode int, code string, message string) {
	writeJSON(w, statusCode, errorResponse{
		Error: errorDetails{
			Code:    code,
			Message: message,
		},
	})
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}

	w.Header().Set("Allow", method)
	writeJSONError(w, http.StatusMethodNotAllowed, errorCodeMethodNotAllowed, "method not allowed")

	return false
}
