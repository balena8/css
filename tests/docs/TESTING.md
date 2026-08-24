# Testing

This document explains how to run and understand the test scripts for `check-stateless-server`.

The project contains PowerShell-based smoke, load, and queue watching scripts for the receipt processing API.

## Test Directory

```text
tests/
    helpers/
    results/
    smoke-single.ps1
    smoke-batch.ps1
    load-single-sequential.ps1
    load-single-parallel.ps1
    load-batch-sequential.ps1
    load-batch-parallel.ps1
    watcher-queues.ps1
```

## Requirements

Before running receipt tests, the server must be running.

Using Docker:

```powershell
docker compose up --build
```

Or locally:

```powershell
go run ./cmd -config config.docker.yaml
```

Verify that the server is available:

```powershell
curl http://localhost:8080/health
```

Expected response:

```json
{
  "status": "ok"
}
```

Also verify that the OpenAPI contract is available:

```powershell
curl http://localhost:8080/openapi.json
```

## Test Categories

The receipt test suite is split into four categories:

```text
1. Smoke tests
2. Sequential load tests
3. Parallel load tests
4. Queue watching
```

## Smoke Tests

Smoke tests verify that the main API flow works with a small number of requests.

They are the first tests to run after starting the server.

### Single Receipt Smoke Test

```powershell
.\tests\smoke-single.ps1
```

This test sends one request to:

```text
POST /receipts/parse
```

It checks that the server can:

* accept a single receipt request;
* validate request JSON;
* submit the job to the receipt pipeline;
* process the receipt;
* return a structured response.

### Batch Receipt Smoke Test

```powershell
.\tests\smoke-batch.ps1
```

This test sends one batch request to:

```text
POST /receipts/parse/batch
```

It checks that the server can:

* accept multiple receipt QR codes;
* submit a batch job;
* process multiple receipts;
* return per-receipt results.

## Sequential Load Tests

Sequential tests send requests one after another.

They are useful for checking correctness and average processing time without concurrency pressure.

### Single Receipt Sequential Test

```powershell
.\tests\load-single-sequential.ps1 -TotalRequests 5
```

This sends several single receipt requests sequentially.

Use it to check:

* stable processing behavior;
* repeated single receipt processing;
* average request duration;
* whether any request fails after repeated usage.

### Batch Sequential Test

```powershell
.\tests\load-batch-sequential.ps1 -TotalBatches 3 -ReceiptsPerBatch 3
```

This sends several batch requests sequentially.

Use it to check:

* batch processing stability;
* per-batch duration;
* behavior with multiple receipts per request.

## Parallel Load Tests

Parallel tests send multiple requests concurrently.

They are useful for checking queue behavior, worker pool behavior, and system stability under load.

### Single Receipt Parallel Test

```powershell
.\tests\load-single-parallel.ps1 -TotalRequests 50
```

This sends multiple single receipt requests in parallel.

Use it to check:

* single queue behavior;
* worker pool concurrency;
* queue waiting time;
* external tax API pressure;
* LLM provider pressure if enrichment is enabled.

### Batch Parallel Test

```powershell
.\tests\load-batch-parallel.ps1 -TotalBatches 10 -ReceiptsPerBatch 3
```

This sends multiple batch requests in parallel.

Use it to check:

* batch queue behavior;
* batch worker throughput;
* whether batch processing affects server stability;
* how the service behaves when many receipts are submitted at once.

## Queue Watching

The queue watcher script periodically checks receipt queue endpoints.

```powershell
.\tests\watcher-queues.ps1
```

It is useful while load tests are running.

Queue watching uses these endpoints:

```text
GET /receipts/queues
GET /receipts/queues/single
GET /receipts/queues/batch
GET /receipts/queues/user?user_id=<user-id>
```

The watcher helps observe:

* pending jobs;
* active jobs;
* queue capacity;
* active workers;
* user-specific queue status.

## Recommended Testing Order

Use this order when validating the project:

```powershell
go test ./...
```

```powershell
docker compose up --build
```

```powershell
curl http://localhost:8080/health
```

```powershell
curl http://localhost:8080/openapi.json
```

```powershell
.\tests\smoke-single.ps1
```

```powershell
.\tests\smoke-batch.ps1
```

```powershell
.\tests\load-single-sequential.ps1 -TotalRequests 5
```

```powershell
.\tests\load-batch-sequential.ps1 -TotalBatches 3 -ReceiptsPerBatch 3
```

```powershell
.\tests\load-single-parallel.ps1 -TotalRequests 50
```

```powershell
.\tests\load-batch-parallel.ps1 -TotalBatches 10 -ReceiptsPerBatch 3
```

## Results Directory

Test output should be saved under:

```text
tests/results/
```

The result files are useful for comparing runs and debugging performance changes.

Typical result data may include:

```text
total requests
successful requests
failed requests
duration
average request time
response status codes
error messages
```

## How to Interpret Results

### Successful Smoke Test

A successful smoke test means the basic API flow works.

It does not prove that the system is ready for high load.

### Failed Smoke Test

If a smoke test fails, do not run load tests yet.

First check:

```powershell
curl http://localhost:8080/health
```

Then check logs:

```powershell
docker compose logs -f check-stateless-server
```

Common causes:

* server is not running;
* wrong port;
* invalid config;
* missing Gemini/OpenAI key;
* external tax API failure;
* LLM provider failure;
* invalid test QR code.

### Sequential Load Test Success

A successful sequential test means the service can process repeated requests consistently.

This is useful for finding problems with:

* repeated external API calls;
* repeated enrichment calls;
* memory leaks;
* response mapping;
* result aggregation.

### Parallel Load Test Success

A successful parallel test means the queue and worker pool can handle concurrent traffic.

This is useful for validating:

* queue capacity;
* worker count;
* request timeout settings;
* concurrency behavior;
* queue watching endpoints.

### Queue Growth

If queue pending count grows and does not decrease, possible causes are:

* not enough workers;
* external tax API is slow;
* LLM provider is slow;
* provider rate limits are reached;
* request timeout is too short;
* batch size is too large.

## Testing With Enrichment Disabled

For faster receipt pipeline testing, enrichment can be disabled:

```yaml
receipt_service:
  enrichment:
    enabled: false
```

This is useful when testing:

* queue behavior;
* receipt parsing;
* tax API integration;
* handler behavior;
* request validation.

## Testing With Gemini

For full production-like behavior, use Gemini enrichment:

```yaml
receipt_service:
  enrichment:
    enabled: true
    provider: "gemini"
    model: "gemini-2.5-flash"
    token_budget: 4096
```

Make sure the key exists:

```text
secrets/gemini.key
```

Inside Docker, it should be mounted as:

```text
/app/secrets/gemini.key
```

## Testing With Ollama

When testing Ollama from Docker Desktop on Windows, use:

```yaml
receipt_service:
  enrichment:
    enabled: true
    provider: "ollama"
    model: "llama3.2"
    base_url: "http://host.docker.internal:11434"
```

Do not use `localhost` inside Docker for host Ollama.

Inside the container, `localhost` means the container itself.

## Troubleshooting

### Server is not reachable

Check Docker:

```powershell
docker compose ps
```

Check logs:

```powershell
docker compose logs -f check-stateless-server
```

Check health:

```powershell
curl http://localhost:8080/health
```

### OpenAPI endpoint does not work

Use GET:

```powershell
curl http://localhost:8080/openapi.json
```

The endpoint should return a JSON document with:

```json
{
  "openapi": "3.0.3"
}
```

### Missing API key

For Gemini:

```text
Gemini API key file not found
```

Check:

```powershell
dir secrets
```

Make sure this file exists:

```text
secrets/gemini.key
```

### Request timeout

If processing times out, increase:

```yaml
server:
  write_timeout: 15m
```

Also check whether enrichment is enabled. LLM calls can make processing slower.

### Too many failed requests in load tests

Check:

* server logs;
* queue stats;
* external tax API availability;
* LLM provider limits;
* request timeout;
* worker count;
* batch size.

## Pre-Merge Testing Checklist

Before merging important backend changes, run:

```powershell
go test ./...
```

```powershell
docker compose up --build
```

```powershell
curl http://localhost:8080/health
```

```powershell
curl http://localhost:8080/openapi.json
```

```powershell
.\tests\smoke-single.ps1
```

```powershell
.\tests\smoke-batch.ps1
```

At minimum, smoke tests and Go tests should pass before considering the change stable.

For queue or worker changes, also run:

```powershell
.\tests\load-single-parallel.ps1 -TotalRequests 50
```

```powershell
.\tests\load-batch-parallel.ps1 -TotalBatches 10 -ReceiptsPerBatch 3
```
