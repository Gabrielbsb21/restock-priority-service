# ADR-003: Publish the contract as a hand-written OpenAPI document, served with an embedded Swagger UI

| Field | Value |
| --- | --- |
| Status | Accepted |
| Date | 2026-07-29 |
| Decision owners | Restock Priority Service team |
| Related specifications | SPEC-001 |
| Supersedes | None |

## Context

SPEC-001 describes the HTTP contract completely and normatively, but only in prose. A
consumer cannot import prose into Postman, generate a client from it, or click through the
endpoints. The specification also listed "OpenAPI" as an implementation task that had been
deferred out of the v1 delivery.

Publishing the contract in a machine-readable form changes how the service is integrated
with, which `docs/sdd/README.md` names as an ADR trigger.

## Decision drivers

- A reviewer should be able to browse the endpoints without reading Go or Markdown.
- Documentation that can silently disagree with the code is worse than no documentation,
  because it is trusted.
- `docs/project-overview.md` requires local execution without external APIs, so the
  documentation must not depend on a CDN being reachable.
- SPEC-001 stays the source of truth for behaviour. A second document must not become a
  competing normative source.

## Considered options

### Option A: Generate the document from handler annotations (`swaggo/swag`)

The conventional Gin approach. Comment blocks above each handler are compiled into a
Swagger document by a CLI, and the result is committed.

Costs: `swag` v1 emits Swagger 2.0 rather than OpenAPI 3. It adds roughly ninety lines of
annotation to the handlers, plus a generated package and a CLI step in the build. It needs
`swaggertype` overrides for `decimal.Decimal` and `uuid.UUID`, whose real shapes render as
`{}` and as an array of bytes.

Most importantly, proximity is not verification. A stale annotation still generates a
document, and nothing fails: annotations describe what the author believed, not what the
handler does.

### Option B: Hand-write the document, and test that it matches the router

Author `api/openapi.yaml` directly, in OpenAPI 3.0, with full control over schemas,
examples, constraints and the error envelope. Embed it and serve it.

Cost: nothing derives the document from the code, so it can drift — unless drift is made to
fail the build, which is what makes this option viable rather than merely tidier.

## Decision

The contract is published as a hand-written OpenAPI 3.0 document at `api/openapi.yaml`,
embedded with `go:embed` and served at `GET /openapi.yaml`, with a Swagger UI at
`GET /docs`. Four constraints hold it together:

1. **Drift fails the build.** `TestOpenAPI_MatchesRegisteredRoutes` compares the router's
   registered routes against the document's operations and fails in **both** directions: a
   route added without documenting it, and an operation documented but not served. Routes
   that serve the documentation itself are excluded through a named list, and
   `TestOpenAPI_UndocumentedRoutesAreAllServed` fails if that list outlives its routes.
2. **The document must be internally sound.** `TestOpenAPI_EveryReferenceResolves` walks
   every `$ref` and fails on one that does not resolve, which would otherwise surface only
   as a broken page. `TestOpenAPI_DeclaresEveryErrorCode` ties the documented `code`
   enumeration to the codes the service actually emits, in both directions.
3. **No network at runtime.** The Swagger UI assets come from `swaggo/files/v2`, which
   embeds them, and the document is embedded too. Nothing is fetched from a CDN and nothing
   is read from disk.
4. **The server URL is relative.** `servers` is `/`, so "Try it out" targets whichever host
   and port served the page. A hardcoded `http://localhost:8080` silently breaks the moment
   the API is published anywhere else.

OpenAPI 3.0 rather than 3.1: 3.1's additions are not needed here, and 3.0 renders in every
version of Swagger UI, Redoc and Postman.

Swagger UI's own `swagger-initializer.js` is overridden. The shipped file points at the
public petstore demo, so serving the assets unmodified would present somebody else's API
under our title.

## Consequences

### Positive

- The contract is browsable, importable, and usable for client generation.
- The document cannot quietly disagree with the router about which endpoints exist, or
  about which error codes are possible.
- Full fidelity to SPEC-001: field constraints, the error envelope, both `404` codes, the
  `Location` header, the correlation header, and the challenge-example discrepancy are all
  described.
- Serving the UI directly from the embedded filesystem removed the `gin-swagger`,
  `swaggo/swag` and `go-openapi/*` dependency trees, which existed only to parse
  annotation-generated documents this service does not produce.

### Negative

- Schemas are maintained by hand. The tests cover the route set and the error codes, not
  whether a property's type matches its DTO field; that remains review's job.
- The embedded Swagger UI assets are about 11 MB, of which roughly 6.8 MB are JavaScript
  source maps that are never usefully served. They cannot be excluded from the upstream
  package's `go:embed`, and vendoring the assets to trim them would put megabytes of
  third-party JavaScript into the repository, which is worse to review. The Docker build
  links with `-s -w`, which recovers about a quarter of the binary.
- The documentation endpoints are unauthenticated, like the whole of v1.

### Follow-up

- If the schemas and DTOs drift in practice, consider a round-trip test that marshals a
  DTO and validates it against the document's schema.

## Reversal strategy

The document is one file plus a `go:embed`, and the routes are confined to
`internal/adapter/http/docs.go`. Moving to generated annotations later means deleting both
and adding the CLI step; the drift tests would be deleted with them, since generation makes
them meaningless. Nothing else in the service imports either.

Reconsider if the hand-written schemas are observed drifting from the DTOs despite review,
or if a consumer needs OpenAPI 3.1 specifically.

## Status history

| Date | Status | Note |
| --- | --- | --- |
| 2026-07-29 | Accepted | Delivers the OpenAPI task that SPEC-001 had deferred |
