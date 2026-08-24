# Operations

This document explains how to configure, run, test, and troubleshoot `check-stateless-server`.

## Configuration File

The application uses YAML configuration.

Example Docker configuration:

```yaml
server:
  listen_address: "0.0.0.0"
  port: 8080
  read_timeout: 15s
  write_timeout: 15s
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

## Server Settings

```yaml
server:
  listen_address: "0.0.0.0"
  port: 8080
  read_timeout: 15s
  write_timeout: 15m
  idle_timeout: 60s
```

`write_timeout` should be long enough for:

* queue waiting;
* external tax API calls;
* receipt XML parsing;
* optional LLM enrichment.

For this service, `15m` is safer than a short generic HTTP timeout.

## CORS

Local development:

```yaml
cors:
  enabled: true
  allowed_origins:
    - "*"
```

Production with credentials should avoid wildcard origins.

Use explicit origins instead:

```yaml
cors:
  enabled: true
  allowed_origins:
    - "https://example.com"
```

## API Keys

The app can load provider keys from configured files:

```yaml
api_keys:
  openai: "/app/secrets/openai.key"
  gemini: "/app/secrets/gemini.key"
  anthropic: "/app/secrets/anthropic.key"
```

Or from environment variables:

```text
OPENAI_API_KEY
GEMINI_API_KEY
ANTHROPIC_API_KEY
```

Ollama does not require an API key for the default local setup.

## Secrets Directory

Recommended local structure:

```text
secrets/
  gemini.key
  openai.key
  anthropic.key
```

Do not commit secrets.

Recommended `.gitignore` entries:

```gitignore
secrets/*
secret/*
*.key
*.pem
.env
.env.*
```

## LLM Enrichment

Disable enrichment:

```yaml
receipt_service:
  enrichment:
    enabled: false
```

Use Gemini:

```yaml
receipt_service:
  enrichment:
    enabled: true
    provider: "gemini"
    model: "gemini-2.5-flash"
    token_budget: 4096
```

Use OpenAI:

```yaml
receipt_service:
  enrichment:
    enabled: true
    provider: "openai"
    model: "gpt-4.1-mini"
    token_budget: 4096
```

Use Ollama from Docker Desktop on Windows:

```yaml
receipt_service:
  enrichment:
    enabled: true
    provider: "ollama"
    model: "llama3.2"
    base_url: "http://host.docker.internal:11434"
    token_budget: 4096
```

Anthropic is reserved for future support. Do not use it as `receipt_service.enrichment.provider` until `llm/anthropic` is implemented.

## Docker

Dockerfile should build the app from `./cmd`:

```yaml
args:
  MAIN_PACKAGE: "./cmd"
```

Docker Compose should mount the secrets directory:

```yaml
volumes:
  - ./config.docker.yaml:/app/config/config.docker.yaml:ro
  - ./secrets:/app/secrets:ro
```

Run:

```powershell
docker compose up --build
```

Check health:

```powershell
curl http://localhost:8080/health
```

Check OpenAPI:

```powershell
curl http://localhost:8080/openapi.json
```

## Local Run

Run without Docker:

```powershell
go run ./cmd -config config.docker.yaml
```

Run tests:

```powershell
go test ./...
```

## Receipt Test Scripts

Smoke tests:

```powershell
.\tests\receipt\smoke-single.ps1
.\tests\receipt\smoke-batch.ps1
```

Sequential tests:

```powershell
.\tests\receipt\load-single-sequential.ps1 -TotalRequests 5
.\tests\receipt\load-batch-sequential.ps1 -TotalBatches 3 -ReceiptsPerBatch 3
```

Parallel tests:

```powershell
.\tests\receipt\load-single-parallel.ps1 -TotalRequests 50
.\tests\receipt\load-batch-parallel.ps1 -TotalBatches 10 -ReceiptsPerBatch 3
```

Queue monitor:

```powershell
.\tests\receipt\monitor-queues.ps1
```

Test results are written to:

```text
tests/receipt/results/
```

## Queue Monitoring

Use:

```text
GET /receipts/queues
```

For one user:

```text
GET /receipts/queues/user?user_id=user-1
```

Queue stats are useful when:

* requests are waiting too long;
* batch jobs are blocking workers;
* LLM enrichment is slow;
* external tax API is slow;
* worker count is too low;
* queue buffer size is too small.

## Recommended Worker Settings

Local development:

```yaml
workers:
  single_workers: 2
  batch_workers: 1
```

Normal Docker development:

```yaml
workers:
  single_workers: 4
  batch_workers: 2
```

Higher throughput:

```yaml
workers:
  single_workers: 8
  batch_workers: 4
```

Increase worker counts carefully. More workers can increase pressure on:

* the external tax API;
* the configured LLM provider;
* CPU;
* memory;
* network;
* provider rate limits.

## Troubleshooting

### `Gemini API key file not found`

Check that the file exists:

```powershell
dir secrets
```

Check `config.docker.yaml`:

```yaml
api_keys:
  gemini: "/app/secrets/gemini.key"
```

Check Docker Compose volume:

```yaml
volumes:
  - ./secrets:/app/secrets:ro
```

### `unsupported receipt enrichment provider: anthropic`

Anthropic completion provider is not implemented yet.

Use:

```yaml
provider: "gemini"
```

or:

```yaml
provider: "openai"
```

or:

```yaml
provider: "ollama"
```

### `/openapi.json` returns method not allowed

Use GET:

```powershell
curl http://localhost:8080/openapi.json
```

### Request body is rejected because of an unknown field

The server uses strict JSON decoding.

This is intentional. Unknown fields are rejected to prevent silent client-side mistakes.

Example invalid request:

```json
{
  "user_id": "user-1",
  "qr": "..."
}
```

Correct request:

```json
{
  "user_id": "user-1",
  "qr_code": "..."
}
```

### Receipt processing times out

Increase:

```yaml
server:
  write_timeout: 15m
```

Check queue stats:

```powershell
curl http://localhost:8080/receipts/queues
```

Check whether the external tax API or LLM provider is slow.

### Ollama does not work inside Docker

If Ollama runs on the Windows host machine and the app runs in Docker, use:

```yaml
base_url: "http://host.docker.internal:11434"
```

Not:

```yaml
base_url: "http://localhost:11434"
```

Inside a Docker container, `localhost` means the container itself, not the Windows host.

## Pre-Documentation Checklist

Before treating documentation as stable, verify:

```powershell
go test ./...
```

```powershell
go run ./cmd -config config.docker.yaml
```

```powershell
curl http://localhost:8080/health
```

```powershell
curl http://localhost:8080/openapi.json
```

```powershell
.\tests\receipt\smoke-single.ps1
```

If all checks pass, README and operations docs are aligned with the current server behavior.
