# ADR-0006: Separate Prompt Construction from Receipt Enrichment Execution

## Status

Accepted

## Date

2026-06-13

## Context

Receipt product enrichment uses an LLM to add structured information to products parsed from fiscal receipts.

The enrichment flow needs to:

* select enrichment options;
* build option profiles;
* include normalized receipt JSON;
* include formatting rules;
* include the expected response structure;
* send the prompt to an LLM provider;
* parse the returned JSON;
* validate the enrichment response;
* retry transient provider failures.

Prompt construction changes more often than receipt parsing or queueing logic.

If prompt templates are mixed directly into receipt processing or provider code, the system becomes harder to maintain. Prompt changes would risk breaking receipt parsing, queue processing, or provider integrations.

The project needs a clear boundary between:

1. building prompts;
2. executing LLM completions;
3. parsing and validating enrichment responses.

## Decision

Prompt construction is isolated in:

```text
internal/prompt
```

Enrichment execution is isolated in:

```text
internal/enrichment
```

The prompt package is responsible for:

* building prompt text;
* rendering selected options;
* including receipt data;
* including rules and requirements;
* including the expected JSON response shape.

The enrichment package is responsible for:

* selecting enrichment options;
* creating option profiles;
* calling the configured `llm.CompletionProvider`;
* parsing LLM output;
* cleaning JSON from markdown/code fences;
* validating the response;
* retrying transient errors.

The receipt processor can call enrichment after the normalized receipt JSON is built, but it does not own prompt rendering details.

## Consequences

### Positive

* Prompt templates can evolve without modifying receipt parsing.
* Enrichment validation is centralized.
* Receipt parsing remains independent from prompt engineering details.
* LLM provider packages remain independent from business-specific prompt logic.
* The prompt package can be tested separately.
* The enrichment package can be tested with fake completion providers.
* Default prompt behavior belongs to the enrichment/prompt subsystem, not to config validation.

### Negative

* More packages exist.
* Developers need to understand the boundary between prompt and enrichment.
* Adding new enrichment options requires updating both option profiles and prompt rendering.

## Boundary Rules

### `internal/prompt`

Should contain:

* prompt builder;
* prompt template;
* default prompt text;
* expected response example/schema;
* rendering logic.

Should not contain:

* LLM HTTP calls;
* provider-specific logic;
* retry logic;
* receipt API fetching;
* queueing logic.

### `internal/enrichment`

Should contain:

* taxonomy;
* enrichment options;
* LLM call orchestration;
* retry policy;
* response parsing;
* response validation.

Should not contain:

* HTTP route handlers;
* queue worker code;
* external tax API fetching;
* provider-specific HTTP clients.

### `llm`

Should contain:

* provider-neutral request/response types;
* provider-specific adapters;
* shared completion interface.

Should not contain:

* receipt-specific taxonomy;
* receipt prompt templates;
* product enrichment validation rules.

## Default System Prompt

The `system_prompt` configuration field is optional.

When it is empty, the enrichment package uses its own default system prompt.

This avoids duplicating prompt defaults in configuration and keeps prompt ownership inside the prompt/enrichment subsystem.

## Retry Policy

The enrichment layer owns retry decisions because it has enough context to understand which failures are safe to retry.

Retryable cases include transient provider or network failures such as:

* timeout;
* rate limit;
* temporary provider unavailability;
* gateway errors;
* empty or incomplete provider response;
* malformed JSON caused by transient generation failure.

Validation errors for unsupported categories, invalid product indexes, or invalid response shape should not be hidden. They should fail clearly so prompt or schema issues can be fixed.

## Alternatives Considered

### Build prompts directly inside the receipt processor

Rejected.

This would couple receipt parsing with prompt engineering and LLM response handling.

### Build prompts inside provider packages

Rejected.

Provider packages should know how to call Gemini, OpenAI, or Ollama. They should not know receipt-specific business logic.

### Put prompt defaults in config validation

Rejected.

Config should validate user-provided values, not own prompt behavior. The prompt/enrichment subsystem should own prompt defaults.

## Related Components

```text
internal/prompt/
  defaults.go
  prompt.go
  renderer.go
  schema.go
  template.go

internal/enrichment/
  models.go
  options.go
  receipt_llm_enricher.go
  taxonomy.go

llm/
  provider.go
```
