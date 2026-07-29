# ADR-001: Use Gin for the HTTP transport adapter

| Field | Value |
| --- | --- |
| Status | Accepted |
| Date | 2026-07-28 |
| Decision owners | Restock Priority Service team |
| Related specifications | SPEC-001 |
| Supersedes | None |

## Context

`docs/architecture/system-design.md` names `github.com/gin-gonic/gin` in the technology
baseline, while `docs/engineering/go-guidelines.md` states that the service must "use
`net/http` unless an approved specification establishes a framework need". SPEC-001
never mentions Gin, so no approved specification established that need, and no ADR
recorded the choice. The contradiction was therefore unresolved in writing even though
the implementation already depended on Gin.

`docs/sdd/README.md` requires an ADR when a change "introduces a durable infrastructure
or data-storage choice", and the system design requires one when a framework choice has
durable or cross-cutting consequences. This decision closes that gap rather than
changing behaviour.

## Decision drivers

- The contradiction between two normative documents must be resolved explicitly, not
  left for a reader to discover.
- The v1 surface is nine routes with no streaming, no websockets and no content
  negotiation, so framework capability is not the deciding factor.
- Replacing the transport adapter later must not touch the domain or the application
  layer.

## Considered options

### Option A: Keep Gin

Routing, parameter binding, grouping, `NoRoute`/`NoMethod` hooks and panic recovery are
provided. The adapter stays small, and Gin is a widely recognised choice that a reviewer
can read without introducing project-specific abstractions.

Costs: a dependency whose behaviour must be understood. Two defects in this service came
from exactly that — `gin.New()` leaves `HandleMethodNotAllowed` false, which made the
`NoMethod` handler dead code and turned a wrong method into a 404, and `gin.SetMode`
mutates package-level state, which constrains test parallelism.

### Option B: Move to `net/http`

Go 1.22's `ServeMux` supports method and wildcard patterns (`GET /parts/{id}`) and
returns 405 for a known path with an unknown method, so the framework's main
contributions are now in the standard library. This is what `go-guidelines.md` asks for
by default and would remove a dependency.

Cost: the change touches every handler, the middleware chain, the error helpers and the
whole HTTP test suite, without altering a single documented behaviour. Evaluated against
the same criteria, it buys strict alignment with the guideline at the price of rewriting
a layer that already meets its contract.

## Decision

The HTTP adapter uses Gin. `docs/engineering/go-guidelines.md` references this ADR as
the approved exception to its `net/http` default.

The following constraints bound the decision:

- Gin types appear only inside `internal/adapter/http`. `gin.Context` must not reach the
  application or domain layers, and no application or domain signature may mention Gin.
- The router configures Gin explicitly rather than relying on its defaults, since those
  defaults have already produced one contract defect.
- Binding is not delegated to Gin's validator. Request shape is decoded with
  `encoding/json` under `DisallowUnknownFields`, and every domain invariant is enforced
  by `domain.Part.Validate`.

## Consequences

### Positive

- Two normative documents now agree, and the exception is discoverable from the guideline
  that it contradicts.
- The transport layer stays thin: handlers decode, invoke a use case and encode.
- No behaviour change and no rewrite of a working, tested layer.

### Negative

- One production dependency whose defaults must be reviewed rather than trusted.
- `gin.SetMode` writes package-level state, so HTTP tests construct routers
  sequentially. This is documented in `internal/adapter/http/handler_test.go`.

### Follow-up

- If a future requirement needs streaming, content negotiation or per-route middleware
  that Gin makes awkward, revisit rather than work around it.

## Reversal strategy

Replacing Gin means rewriting `internal/adapter/http` only: `router.go`, the handlers,
the middleware and the error helpers. `NewRouter` already accepts nothing but
application ports, and the HTTP tests assert public behaviour over `http.Handler` rather
than Gin internals, so they carry over unchanged and would demonstrate that the
replacement preserves the contract.

Reconsider this decision if Gin's defaults cause a further contract defect, if the
dependency falls out of maintenance, or if the v1 surface grows into territory where the
standard library is clearly sufficient and the dependency is pure cost.

## Status history

| Date | Status | Note |
| --- | --- | --- |
| 2026-07-28 | Accepted | Records a choice the implementation already depended on |
