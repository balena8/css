# ADR-0001: Separate Receipt Processing Pipeline with Single and Batch Queues

## Status

Accepted

## Date

2026-06-13

## Context

`check-stateless-server` processes fiscal receipt QR codes by calling an external tax receipt API, decoding receipt payloads, parsing XML, normalizing receipt data, and optionally enriching products through an LLM provider.

Receipt processing is not a trivial in-memory operation. A single request can involve:

* external network calls;
* external API latency;
* XML parsing;
* base64 decoding;
* optional LLM completion calls;
* retry logic;
* per-receipt result aggregation.

The service supports two different workload types:

1. Single receipt requests.
2. Batch receipt requests.

These workloads have different performance characteristics. Single receipt requests are usually user-facing and latency-sensitive. Batch requests can contain multiple receipts and are more throughput-oriented.

If both workload types use one shared queue, large batch jobs can delay smaller single receipt requests. This creates poor latency for interactive users.

## Decision

The service uses a dedicated `internal/receiptpipeline` package with:

* `ReceiptPipeline`
* `ReceiptQueue`
* `ReceiptWorkerPool`
* single receipt queue
* batch receipt queue
* pending job tracking
* active job tracking
* queue statistics endpoints

Single receipt requests are routed to the single queue.

Batch receipt requests are routed to the batch queue.

The HTTP layer submits jobs through a narrow pipeline interface and waits for the job result. The worker pool owns the actual processing execution.

## Consequences

### Positive

* Single receipt traffic is isolated from large batch traffic.
* Queue behavior is observable through API endpoints.
* Worker counts can be tuned independently for single and batch workloads.
* The HTTP layer stays thin and does not own processing details.
* The processing model is easier to test because queueing and execution are isolated.
* Pending and active jobs can be tracked per user.
* Future background processing or async job APIs can be added without redesigning receipt parsing.

### Negative

* The system is more complex than direct synchronous processing.
* There are more moving parts: queues, jobs, workers, result channels, active state.
* Shutdown logic must correctly stop queues and wait for workers.
* Queue configuration must be tuned for expected traffic.

## Alternatives Considered

### Direct synchronous processing in HTTP handlers

Rejected.

This would be simpler, but it would couple HTTP delivery to long-running processing and make queue visibility impossible. It would also make it harder to isolate single and batch workloads.

### One shared queue for all receipt requests

Rejected.

A shared queue would simplify implementation, but batch jobs could block single receipt requests. This would hurt latency for user-facing flows.

### External queue system

Deferred.

Systems such as Redis, RabbitMQ, or Kafka could be used later, but an in-process queue is sufficient for the current service scope. The current design keeps the queue abstraction isolated enough to replace the implementation later.

## Operational Notes

Queue behavior can be observed through:

```text
GET /receipts/queues
GET /receipts/queues/single
GET /receipts/queues/batch
GET /receipts/queues/user?user_id=<user-id>
```

Recommended configuration should be adjusted based on:

* external tax API latency;
* LLM provider latency;
* provider rate limits;
* number of concurrent clients;
* average batch size;
* server CPU and memory limits.

## Related Components

```text
internal/receiptpipeline/
  config.go
  job.go
  queue.go
  pipeline.go
  worker_pool.go

internal/server/
  receipt_handler.go
```
