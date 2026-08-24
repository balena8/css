# ADR-0005: Strict JSON Request Decoding and Structured Error Responses

## Status

Accepted

## Date

2026-06-13

## Context

The HTTP API accepts JSON requests from clients.

Receipt parsing requests are small, but they are part of a larger processing flow that can trigger external API calls, queueing, worker execution, and optional LLM enrichment.

The API should reject invalid input as early as possible.

A common issue with JSON APIs is silently accepting unknown fields. For example, if a client sends:

```json
{
  "user_id": "user-1",
  "qr": "..."
}
```

instead of:

```json
{
  "user_id": "user-1",
  "qr_code": "..."
}
```

the server could ignore `qr` and later fail with a less useful error. This makes client bugs harder to detect.

The service also needs a stable error response format that frontend clients, scripts, tests, and future generated clients can rely on.

## Decision

The server uses strict JSON decoding.

The decoder:

* limits request body size;
* rejects empty request bodies;
* rejects unknown fields;
* rejects multiple JSON values in one body;
* validates request DTOs after decoding.

The server returns structured error responses:

```json
{
  "error": {
    "code": "invalid_request",
    "message": "user_id is required"
  }
}
```

The public error shape is stable:

```go
type errorResponse struct {
    Error errorDetails `json:"error"`
}

type errorDetails struct {
    Code    string         `json:"code"`
    Message string         `json:"message"`
    Details map[string]any `json:"details,omitempty"`
}
```

## Consequences

### Positive

* Client mistakes are detected early.
* Misspelled JSON fields do not get silently ignored.
* Error responses are consistent across handlers.
* Frontend clients can use `error.code` for machine-readable behavior.
* Human-readable messages are available through `error.message`.
* OpenAPI documentation can describe one stable error format.
* Tests can assert exact error behavior more easily.

### Negative

* Clients must send exact field names.
* Adding request fields requires updating request DTOs and OpenAPI schemas.
* Strict decoding can be less forgiving for quickly prototyped clients.

## Error Codes

Current stable error codes:

```text
invalid_request
method_not_allowed
request_canceled
request_timeout
receipt_processing_failed
internal_server_error
```

These codes should stay stable because clients may rely on them.

## Request Body Limits

Receipt API request bodies are limited to prevent accidental or abusive large payloads.

The current receipt request body limit is:

```text
1 MB
```

This is enough for receipt QR code requests and batch requests while still protecting the server from oversized payloads.

## Error Exposure Policy

Internal errors are logged, but not exposed directly to clients.

For example, provider errors, external tax API details, queue internals, or infrastructure errors should not be returned directly as public HTTP response messages.

Instead, the server returns a stable public message:

```json
{
  "error": {
    "code": "receipt_processing_failed",
    "message": "receipt processing failed"
  }
}
```

The detailed error remains in logs.

## Alternatives Considered

### Lenient JSON decoding

Rejected.

Lenient decoding makes client bugs harder to find and can silently ignore important fields.

### Plain string errors

Rejected.

Plain errors are difficult for frontend clients and tests to handle reliably.

### Exposing raw internal errors to clients

Rejected.

Internal errors may contain provider messages, external API details, infrastructure information, or implementation details.

## Related Components

```text
internal/server/
  errors.go
  http_json.go
  receipt_handler.go
```
