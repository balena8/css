# ADR-0004: Passive Configuration and Startup Validation

## Status

Accepted

## Date

2026-06-13

## Context

`check-stateless-server` has multiple runtime subsystems:

* HTTP server;
* CORS middleware;
* external tax receipt API client;
* receipt processing pipeline;
* single and batch queues;
* worker pool;
* optional LLM enrichment;
* provider-specific LLM clients;
* API key loading.

These subsystems need configuration, but configuration itself should not create runtime objects.

If the configuration package starts creating HTTP clients, workers, queues, processors, or LLM providers, it becomes a hidden application bootstrap layer. That makes startup harder to reason about and makes tests more difficult because loading configuration would also create runtime dependencies.

The project needs configuration to stay predictable, testable, and safe.

## Decision

The `internal/config` package stays passive.

It is responsible for:

* defining configuration structs;
* providing default values;
* loading YAML files;
* validating configuration values;
* loading API keys from files or environment variables.

It is not responsible for creating:

* HTTP servers;
* routers;
* queues;
* worker pools;
* receipt processors;
* LLM providers;
* loggers;
* application services.

Runtime objects are created in the application bootstrap layer, currently from `cmd/main.go`.

Configuration validation runs during startup before the server starts accepting traffic.

## Consequences

### Positive

* Configuration stays easy to test.
* Startup behavior is explicit.
* Runtime dependencies are created in one place.
* Invalid configuration fails fast.
* Missing required API keys fail before the server starts accepting requests.
* Config loading does not have hidden side effects.
* The same configuration structs can be reused in tests, Docker, local development, and production.

### Negative

* Application bootstrap code is slightly more verbose.
* Adding a new subsystem requires updating both config structs and bootstrap wiring.
* Validation logic must stay aligned with runtime factory behavior.

## Validation Rules

The configuration layer validates:

* server address;
* server port;
* HTTP timeouts;
* TLS certificate paths when TLS is enabled;
* external tax API settings;
* queue buffer sizes;
* worker counts;
* enrichment provider;
* enrichment model;
* enrichment token budget;
* enrichment headers map.

The `system_prompt` field is optional.

If it is empty, the enrichment package uses its internal default prompt.

This keeps prompt defaults close to prompt logic instead of duplicating prompt behavior in config validation.

## Provider Validation

Supported receipt enrichment providers are:

```text
gemini
openai
ollama
```

Anthropic key loading exists, but the Anthropic completion provider is not implemented yet.

Therefore this value is intentionally rejected for receipt enrichment:

```yaml
receipt_service:
  enrichment:
    provider: "anthropic"
```

This prevents the server from starting with a provider that cannot be created by the LLM factory.

## Alternatives Considered

### Create runtime objects inside config

Rejected.

This would make configuration loading perform hidden work and couple configuration to application runtime.

### Skip startup validation and fail during requests

Rejected.

Failing during user requests would produce worse operational behavior. Missing keys, invalid provider names, and invalid queue settings should fail before the server starts.

### Store all defaults only in YAML

Rejected.

Code-level defaults are useful for tests and local development. YAML files should override defaults, not become the only source required for application startup.

## Related Components

```text
internal/config/
  apikeys.go
  config.go
  loader.go
  validation.go

cmd/
  main.go
```
