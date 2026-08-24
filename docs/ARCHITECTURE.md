# Architecture

This document describes the architecture of `check-stateless-server`.

The service is designed around a clear separation of responsibilities:

* HTTP delivery is handled by `internal/server`.
* Configuration is handled by `internal/config`.
* Receipt fetching, parsing, and mapping are handled by `internal/receipt`.
* Queueing and worker orchestration are handled by `internal/receiptpipeline`.
* Prompt construction is handled by `internal/prompt`.
* Product enrichment is handled by `internal/enrichment`.
* LLM provider integrations are handled by `llm`.

## High-Level Flow

```text
Client
  |
  | HTTP request
  v
internal/server
  |
  | validate request
  v
internal/receiptpipeline
  |
  | enqueue job
  v
worker pool
  |
  | process receipt QR codes
  v
internal/receipt
  |
  | external tax API
  | XML/base64 parsing
  | normalized receipt JSON
  v
internal/enrichment
  |
  | optional LLM enrichment
  v
response returned to client
```

## Request Flow

### Single receipt

```text
POST /receipts/parse
  |
  v
ReceiptHandler.ParseReceipt
  |
  v
ReceiptPipeline.Submit(user_id, []qr_code)
  |
  v
single receipt queue
  |
  v
single receipt worker
  |
  v
ReceiptProcessor.Process
  |
  v
ReceiptAPIClient.Fetch
  |
  v
BuildReceiptJSON
  |
  v
optional ReceiptLLMEnricher.EnrichReceipt
  |
  v
ProcessReceiptsResponse
```

### Batch receipt

```text
POST /receipts/parse/batch
  |
  v
ReceiptHandler.ParseReceiptBatch
  |
  v
ReceiptPipeline.Submit(user_id, qr_codes)
  |
  v
batch receipt queue
  |
  v
batch receipt worker
  |
  v
ReceiptProcessor.Process
  |
  v
per-receipt results
```

Single and batch requests are intentionally separated because they have different runtime characteristics.

Single requests are optimized for user-facing latency.

Batch requests are optimized for throughput and can process multiple receipts in one job.

## Server Layer

Package:

```text
internal/server
```

Responsibilities:

* Route registration.
* HTTP method validation.
* Strict JSON request decoding.
* Structured JSON responses.
* Structured error responses.
* CORS middleware.
* Request logging middleware.
* OpenAPI route exposure.
* Mapping processing errors to HTTP status codes.

Important routes:

```text
GET  /health
GET  /openapi.json
POST /receipts/parse
POST /receipts/parse/batch
GET  /receipts/queues
GET  /receipts/queues/single
GET  /receipts/queues/batch
GET  /receipts/queues/user
```

The server layer depends on the receipt pipeline through a narrow interface. It does not know how jobs are queued or processed internally.

## Configuration Layer

Package:

```text
internal/config
```

Responsibilities:

* Provide production-safe defaults.
* Load YAML configuration.
* Validate configuration.
* Load API keys from files or environment variables.
* Normalize provider names.
* Keep runtime initialization outside config.

The config layer does not create HTTP clients, queues, workers, processors, loggers, or LLM providers.

It only provides passive configuration data.

## Receipt Layer

Package:

```text
internal/receipt
```

Responsibilities:

* Validate and parse receipt QR URLs.
* Build requests to the external tax receipt API.
* Decode tax API responses.
* Normalize XML and base64 payloads.
* Map raw XML models into stable JSON response models.
* Optionally call enrichment after the base receipt model is built.

Core components:

```text
ReceiptAPIClient
ReceiptProcessor
BuildReceiptJSON
```

The receipt layer owns the transformation from external tax API data into the application receipt model.

## Receipt Pipeline Layer

Package:

```text
internal/receiptpipeline
```

Responsibilities:

* Accept receipt jobs.
* Split workloads into single and batch queues.
* Track pending jobs.
* Track active jobs.
* Run worker pools.
* Return processing results back to HTTP handlers.
* Expose queue statistics.

Core concepts:

```text
ReceiptPipeline
ReceiptQueue
ReceiptWorkerPool
ReceiptJob
ReceiptJobResult
```

The pipeline uses two queues:

```text
single queue
batch queue
```

This prevents large batch jobs from blocking smaller single-receipt requests.

## Queue Model

Each submitted job contains:

```text
job ID
user ID
receipt QR codes
context
created time
result channel
```

The queue tracks pending jobs separately from the channel length. This allows the API to expose meaningful queue state to clients and monitoring tools.

Queue stats include:

```text
pending jobs
pending users
active jobs
active users
queue capacity
worker IDs
job creation timestamps
job start timestamps
```

## Worker Pool

The worker pool owns execution.

A worker:

1. receives a job from the queue;
2. marks the job as active;
3. calls the receipt processor;
4. sends the result back through the job result channel;
5. removes the active job entry.

The worker pool isolates panics so one failed job cannot kill the whole server process.

## Prompt Layer

Package:

```text
internal/prompt
```

Responsibilities:

* Build LLM prompts for product enrichment.
* Render selected enrichment options.
* Include normalized receipt JSON.
* Include expected response shape.
* Keep prompt templates out of the receipt processor.

The prompt package does not call LLM providers directly.

It only builds the prompt text.

## Enrichment Layer

Package:

```text
internal/enrichment
```

Responsibilities:

* Define product taxonomy.
* Define supported enrichment options.
* Build enrichment option profiles.
* Call the LLM completion provider.
* Parse and validate LLM JSON responses.
* Retry transient completion failures.
* Return a normalized backend enrichment response.

The enrichment layer depends only on the abstract completion provider interface, not on Gemini, OpenAI, or Ollama directly.

## LLM Layer

Package:

```text
llm
```

Responsibilities:

* Define provider-neutral completion interfaces.
* Define request and response models.
* Provide provider-specific implementations.
* Provide a factory for selecting the configured provider.

Main interface:

```go
type CompletionProvider interface {
    Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
    CompleteStream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, <-chan error)
    ModelName() string
}
```

Implemented providers:

```text
llm/gemini
llm/openai
llm/ollama
```

Reserved provider:

```text
anthropic
```

Anthropic key loading exists, but `llm/anthropic` is not implemented yet.

## Dependency Direction

The intended dependency direction is:

```text
cmd
  -> internal/config
  -> internal/server
  -> internal/receiptpipeline
  -> internal/receipt
  -> internal/enrichment
  -> internal/prompt
  -> llm
```

Provider-specific packages are isolated:

```text
llm/factory
  -> llm/gemini
  -> llm/openai
  -> llm/ollama
```

Application logic should depend on interfaces, not on provider-specific packages.

## Error Handling

HTTP errors use a stable structure:

```json
{
  "error": {
    "code": "invalid_request",
    "message": "user_id is required"
  }
}
```

Internal errors are logged but not exposed directly to clients.

This prevents leaking external API errors, queue internals, provider errors, or infrastructure details.

## OpenAPI

The generated OpenAPI document is served at:

```text
GET /openapi.json
```

The OpenAPI package is separated from HTTP handlers:

```text
internal/server/openapi
```

This keeps API documentation generation isolated while still making it available through the server router.

## Design Principles

### Keep configuration passive

Configuration should describe the application, not create runtime objects.

Runtime objects are created during application bootstrap.

### Keep HTTP thin

Handlers should validate requests, call application services, and serialize responses.

Processing logic belongs outside HTTP handlers.

### Keep queues observable

Queue visibility is a first-class feature because receipt processing may involve external APIs and LLM calls.

### Keep LLM providers replaceable

The enrichment pipeline uses a provider interface, so provider-specific details stay outside business logic.

### Keep prompt construction separate

Prompt construction changes frequently. It should not be mixed with receipt parsing, queueing, or HTTP delivery.

### Fail fast during startup

Invalid configuration and missing required API keys should fail before the server starts accepting requests.
