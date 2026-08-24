# check-stateless-server

Fiscal receipt processing API with asynchronous queues and optional LLM-based product enrichment.

`check-stateless-server` accepts fiscal receipt QR codes, fetches receipt data from the external tax receipt API, maps raw receipt XML into a normalized JSON model, and optionally enriches receipt products through an LLM provider such as Gemini, OpenAI, or Ollama.

## Features

* Parse a single fiscal receipt QR code.
* Parse multiple receipt QR codes in one batch request.
* Separate queues for single-receipt and batch-receipt workloads.
* Worker pool based processing.
* Queue visibility endpoints for monitoring pending and active jobs.
* Optional LLM enrichment for receipt products.
* Supported completion providers:

  * Gemini
  * OpenAI
  * Ollama
* Reserved provider configuration:

  * Anthropic API key loading exists, but the Anthropic completion provider is not implemented yet.
* Strict JSON request decoding.
* Stable structured error responses.
* Generated OpenAPI specification available at `/openapi.json`.
* Docker-based runtime setup.
* PowerShell-based smoke and load tests.

## Project Structure

```text
check-stateless-server/
  cmd/
    main.go

  internal/
    config/
      apikeys.go
      config.go
      loader.go
      validation.go

    enrichment/
      models.go
      options.go
      receipt_llm_enricher.go
      taxonomy.go

    prompt/
      defaults.go
      prompt.go
      renderer.go
      schema.go
      template.go

    receipt/
      api_client.go
      mapper.go
      models.go
      processor.go

    receiptpipeline/
      config.go
      job.go
      pipeline.go
      queue.go
      worker_pool.go

    server/
      errors.go
      http_json.go
      middleware.go
      receipt_handler.go
      router.go
      server.go

      openapi/
        openapi_builder.go
        openapi_helpers.go
        openapi_paths.go
        openapi_schemas.go
        openapi_types.go

  llm/
    factory/
      factory.go

    gemini/
      completion.go

    httpclient/
      auth.go
      client.go

    ollama/
      client.go
      completion.go

    openai/
      client.go
      completion.go

    provider.go

  secrets/
    gemini.key
    openai.key
    ollama.key

  tests/
    receipt/
      helpers/
      results/
      smoke-single.ps1
      smoke-batch.ps1
      load-single-sequential.ps1
      load-single-parallel.ps1
      load-batch-sequential.ps1
      load-batch-parallel.ps1
      monitor-queues.ps1

  Dockerfile
  docker-compose.yml
  config.docker.yaml
  go.mod
  go.sum
```

## Requirements

* Go 1.23+
* Docker Desktop, for container-based local runtime
* PowerShell, for test scripts
* A Gemini or OpenAI API key if LLM enrichment is enabled with a remote provider
* Ollama installed and running locally if using the `ollama` provider

## Quick Start

Create the secrets directory:

```powershell
mkdir secrets
```

Create a Gemini key file:

```powershell
notepad secrets\gemini.key
```

Add your Gemini API key as plain text inside the file.

Run with Docker:

```powershell
docker compose up --build
```

Check server health:

```powershell
curl http://localhost:8080/health
```

Check OpenAPI specification:

```powershell
curl http://localhost:8080/openapi.json
```

Run a smoke test:

```powershell
.\tests\receipt\smoke-single.ps1
```

## Running Locally Without Docker

```powershell
go run ./cmd -config config.docker.yaml
```

The server starts on:

```text
http://localhost:8080
```

## Configuration

The application reads YAML configuration through `internal/config`.

Default configuration is provided by code and can be overridden by a YAML file.

The root configuration sections are:

```yaml
server:
api_keys:
receipt_service:
```

Example:

```yaml
server:
  listen_address: "0.0.0.0"
  port: 8080
  read_timeout: 15s
  write_timeout: 15m
  idle_timeout: 60s
  tls:
    enabled: false
    cert_file: ""
    key_file: ""
  cors:
    enabled: true
    allowed_origins:
      - "*"
    allowed_methods:
      - GET
      - POST
      - OPTIONS
    allowed_headers:
      - Authorization
      - Content-Type

api_keys:
  anthropic: "/app/secrets/anthropic.key"
  openai: "/app/secrets/openai.key"
  gemini: "/app/secrets/gemini.key"

receipt_service:
  tax_api:
    base_url: "https://cabinet.tax.gov.ua/ws/api_public/rro/chkAllWeb"
    request_timeout: 20s
    captcha_code: "0"
    receipt_type: "3"

  processing:
    queue:
      single_buffer: 100
      batch_buffer: 100
    workers:
      single_workers: 4
      batch_workers: 2

  enrichment:
    enabled: true
    provider: "gemini"
    model: "gemini-2.5-flash"
    base_url: ""
    system_prompt: ""
    token_budget: 4096
    headers: {}
```

## Secrets

Remote LLM providers use API keys loaded from files or environment variables.

Supported key files:

```text
secrets/
  gemini.key
  openai.key
  anthropic.key
```

Ollama does not require an API key in the default local setup.

If a provider key path is empty, the loader falls back to environment variables and then to provider-specific default files in the user home directory.

Environment variables:

```text
OPENAI_API_KEY
GEMINI_API_KEY
ANTHROPIC_API_KEY
```

## Supported LLM Providers

### Gemini

Recommended default provider.

```yaml
receipt_service:
  enrichment:
    enabled: true
    provider: "gemini"
    model: "gemini-2.5-flash"
    token_budget: 4096
```

### OpenAI

```yaml
receipt_service:
  enrichment:
    enabled: true
    provider: "openai"
    model: "gpt-4.1-mini"
    token_budget: 4096
```

### Ollama

Ollama is used through a local or custom HTTP endpoint.

For Docker Desktop on Windows, if Ollama runs on the host machine:

```yaml
receipt_service:
  enrichment:
    enabled: true
    provider: "ollama"
    model: "llama3.2"
    base_url: "http://host.docker.internal:11434"
```

### Anthropic

Anthropic API key loading exists in the configuration layer, but the Anthropic completion provider is not implemented yet.

Do not use this value yet:

```yaml
provider: "anthropic"
```

## API Endpoints

### System

```text
GET /health
GET /openapi.json
```

### Receipts

```text
POST /receipts/parse
POST /receipts/parse/batch
```

### Receipt Queues

```text
GET /receipts/queues
GET /receipts/queues/single
GET /receipts/queues/batch
GET /receipts/queues/user?user_id=<user-id>
```

## Parse One Receipt

Request:

```http
POST /receipts/parse
Content-Type: application/json
```

```json
{
  "user_id": "user-1",
  "qr_code": "https://cabinet.tax.gov.ua/cashregs/check?date=20260429&time=222006&id=696582&sm=437.40&fn=3000909908"
}
```

Response:

```json
{
  "userId": "user-1",
  "count": 1,
  "results": [
    {
      "status": "success",
      "message": "receipt processed successfully",
      "receiptJson": {},
      "enrichment": {}
    }
  ]
}
```

## Parse Batch Receipts

Request:

```http
POST /receipts/parse/batch
Content-Type: application/json
```

```json
{
  "user_id": "user-1",
  "qr_codes": [
    "https://cabinet.tax.gov.ua/cashregs/check?date=20260429&time=222006&id=696582&sm=437.40&fn=3000909908",
    "https://cabinet.tax.gov.ua/cashregs/check?date=20260429&time=222006&id=696582&sm=437.40&fn=3000909908"
  ]
}
```

## Error Format

All HTTP handlers return structured errors:

```json
{
  "error": {
    "code": "invalid_request",
    "message": "user_id is required"
  }
}
```

Common error codes:

```text
invalid_request
method_not_allowed
request_canceled
request_timeout
receipt_processing_failed
internal_server_error
```

## Testing

Run smoke tests:

```powershell
.\tests\receipt\smoke-single.ps1
.\tests\receipt\smoke-batch.ps1
```

Run sequential load tests:

```powershell
.\tests\receipt\load-single-sequential.ps1 -TotalRequests 5
.\tests\receipt\load-batch-sequential.ps1 -TotalBatches 3 -ReceiptsPerBatch 3
```

Run parallel load tests:

```powershell
.\tests\receipt\load-single-parallel.ps1 -TotalRequests 50
.\tests\receipt\load-batch-parallel.ps1 -TotalBatches 10 -ReceiptsPerBatch 3
```

Monitor queues:

```powershell
.\tests\receipt\monitor-queues.ps1
```

## Development Checks

Run all Go tests:

```powershell
go test ./...
```

Run the server locally:

```powershell
go run ./cmd -config config.docker.yaml
```

## Production Notes

* Do not commit files from `secrets/`.
* Keep `write_timeout` high enough for receipt processing and LLM enrichment.
* Keep CORS wildcard only for local development or public APIs without credentials.
* Use queue stats endpoints to observe pending and active receipt jobs.
* Use `/openapi.json` as the source of truth for external client integration.
