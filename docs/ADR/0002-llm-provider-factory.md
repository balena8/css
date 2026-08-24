# ADR-0002: Provider-Neutral LLM Completion Interface and Factory

## Status

Accepted

## Date

2026-06-13

## Context

Receipt product enrichment can be performed by different LLM providers.

The service currently supports:

* Gemini
* OpenAI
* Ollama

The configuration layer also supports loading an Anthropic API key, but the Anthropic completion provider is not implemented yet.

The enrichment package should not depend directly on provider-specific packages such as `llm/gemini`, `llm/openai`, or `llm/ollama`. If business logic imports concrete provider packages directly, every provider change would affect receipt enrichment code.

The project needs a stable abstraction for LLM completions so the enrichment flow can stay independent from provider-specific HTTP APIs, authentication strategies, request formats, and response formats.

## Decision

The project defines a provider-neutral interface in the `llm` package:

```go
type CompletionProvider interface {
    Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
    CompleteStream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, <-chan error)
    ModelName() string
}
```

Provider-specific implementations live in separate packages:

```text
llm/gemini
llm/openai
llm/ollama
```

Provider creation is centralized in:

```text
llm/factory
```

The factory maps configuration values to concrete provider implementations.

The enrichment layer depends only on the `CompletionProvider` interface.

## Consequences

### Positive

* Enrichment logic is independent from concrete LLM providers.
* Provider-specific authentication remains isolated.
* Provider-specific request and response formats remain isolated.
* New providers can be added without rewriting receipt enrichment.
* Configuration can switch providers without changing business logic.
* Local Ollama can be used without changing the enrichment pipeline.
* OpenAI-compatible gateways can be supported through custom `base_url` and headers.

### Negative

* Provider packages need to normalize different APIs into one internal interface.
* Some provider-specific features may not fit the shared abstraction.
* The factory must be kept aligned with config validation.
* Streaming support exists in the interface even when receipt enrichment currently uses non-streaming completion.

## Supported Providers

### Gemini

Used as the default provider.

Example:

```yaml
receipt_service:
  enrichment:
    enabled: true
    provider: "gemini"
    model: "gemini-2.5-flash"
```

### OpenAI

Example:

```yaml
receipt_service:
  enrichment:
    enabled: true
    provider: "openai"
    model: "gpt-4.1-mini"
```

### Ollama

Ollama does not require an API key in the default local setup.

Example for Docker Desktop on Windows when Ollama runs on the host:

```yaml
receipt_service:
  enrichment:
    enabled: true
    provider: "ollama"
    model: "llama3.2"
    base_url: "http://host.docker.internal:11434"
```

### Anthropic

Anthropic is reserved for future support.

The config layer may load an Anthropic API key, but `llm/anthropic` is not implemented yet. The provider should not be used in `receipt_service.enrichment.provider` until the completion provider exists.

## Alternatives Considered

### Direct provider usage inside enrichment

Rejected.

This would tightly couple enrichment to one provider and make provider switching expensive.

### One generic HTTP client without provider packages

Rejected.

Provider APIs differ in authentication, request shape, response shape, streaming format, and token usage metadata. Dedicated provider packages make these differences explicit and easier to test.

### Implement all providers before introducing abstraction

Rejected.

The abstraction is useful immediately because at least Gemini, OpenAI, and Ollama need a shared completion contract.

## Operational Notes

Provider selection is controlled by:

```yaml
receipt_service:
  enrichment:
    provider: "gemini"
    model: "gemini-2.5-flash"
    base_url: ""
    token_budget: 4096
    headers: {}
```

The API key loader supports file-based and environment-based key loading.

Remote providers usually require keys. Ollama does not require a key by default.

## Related Components

```text
llm/provider.go
llm/factory/factory.go
llm/gemini/
llm/openai/
llm/ollama/
internal/enrichment/
internal/config/
```
