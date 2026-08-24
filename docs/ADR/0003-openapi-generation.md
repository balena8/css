# ADR-0003: Generated OpenAPI Specification Exposed by the Server

## Status

Accepted

## Date

2026-06-13

## Context

The service exposes multiple HTTP endpoints for:

* health checks;
* receipt parsing;
* batch receipt parsing;
* queue monitoring;
* user queue status;
* structured error responses.

Clients need a reliable way to understand the API contract.

A manually written static OpenAPI file can become outdated when handlers, request DTOs, response structures, or error formats change. The project already has a strongly typed OpenAPI builder that describes the current API surface in code.

The OpenAPI contract should be available to clients and tools without requiring manual file copying.

## Decision

The service exposes the generated OpenAPI document through:

```text
GET /openapi.json
```

The OpenAPI generation logic is kept separate from HTTP handlers in:

```text
internal/server/openapi
```

The server router imports the OpenAPI package and serves the generated specification as JSON.

The OpenAPI document describes:

* system endpoints;
* receipt parsing endpoints;
* receipt queue endpoints;
* request schemas;
* response schemas;
* enrichment schemas;
* queue schemas;
* structured error schemas.

## Consequences

### Positive

* `/openapi.json` becomes the source of truth for API clients.
* API tools can generate clients from the running server.
* The OpenAPI builder stays close to backend types and routes.
* Documentation can link to the generated API contract instead of duplicating every field manually.
* The server can expose the current contract in every environment.
* Future endpoint changes can be reviewed in code.

### Negative

* OpenAPI generation code must be maintained with route changes.
* Strongly typed OpenAPI structs require updates when new schema features are needed.
* The generated specification is not automatically derived from Go structs; schema builders still need to be updated manually.

## Alternatives Considered

### Static `openapi.yaml`

Rejected for now.

A static file is simple, but it can drift from the actual server implementation.

### No OpenAPI endpoint

Rejected.

Without OpenAPI, integrations would rely only on README examples and manual inspection.

### Reflection-based OpenAPI generation

Deferred.

Reflection-based generation can reduce manual schema writing, but it may produce less controlled contracts. The current explicit builder keeps the public API contract intentional and reviewable.

## OpenAPI Route

```text
GET /openapi.json
```

The route returns:

```json
{
  "openapi": "3.0.3",
  "info": {
    "title": "check-stateless-server",
    "version": "1.0.0"
  },
  "paths": {},
  "components": {}
}
```

## Documentation Policy

Manual documentation should explain API concepts and provide examples.

The full schema contract should come from:

```text
GET /openapi.json
```

This prevents README and API documentation from becoming a second source of truth.

## Related Components

```text
internal/server/router.go
internal/server/openapi/openapi_builder.go
internal/server/openapi/openapi_paths.go
internal/server/openapi/openapi_schemas.go
internal/server/openapi/openapi_types.go
internal/server/openapi/openapi_helpers.go
```
