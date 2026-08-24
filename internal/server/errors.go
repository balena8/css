package server

const (
	errorCodeInvalidRequest      = "invalid_request"
	errorCodeMethodNotAllowed    = "method_not_allowed"
	errorCodeRequestCanceled     = "request_canceled"
	errorCodeRequestTimeout      = "request_timeout"
	errorCodeReceiptProcessing   = "receipt_processing_failed"
	errorCodeInternalServerError = "internal_server_error"
)

// errorResponse is the public error shape returned by HTTP handlers.
//
// Keep this shape stable. Frontend clients, OpenAPI docs, and batch handlers can
// rely on `error.code` for machine-readable behavior and `error.message` for UI.
type errorResponse struct {
	Error errorDetails `json:"error"`
}

type errorDetails struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}
