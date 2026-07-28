# Go Engineering Guidelines

These rules define how Go code is expected to be written in the Restock Priority
Service. The [system design](../architecture/system-design.md) remains
authoritative for architecture.

Normative words such as **must**, **should**, and **may** express requirement
strength.

## Optimize for clarity

- Code must favor direct, idiomatic Go over patterns imported from other
  ecosystems.
- A new abstraction must solve a current problem, not a hypothetical one.
- The standard library should be preferred when it provides a clear solution.
- Every external dependency must have a specific benefit and a small, understood
  surface area.
- Package names must be short, lower-case, and describe one cohesive concept.

## Package and dependency design

- Executable entry points belong under `cmd`.
- Application code that is not a public library belongs under `internal`.
- Domain packages must not import transport, SQL, logging, or runtime
  configuration packages.
- Adapters may depend on application ports and domain types; the reverse is not
  allowed.
- Import cycles must never be resolved by moving unrelated code into a generic
  shared package.
- DTOs, domain values, and database records must be distinct when their
  responsibilities differ.

## Interfaces

- Define an interface in the package that consumes it.
- Introduce an interface only when it establishes a meaningful boundary or allows
  a real alternative in tests or production.
- Keep interfaces small and use-case oriented.
- Return concrete types from constructors unless callers require polymorphism.
- Do not create a matching interface for every concrete type.
- Do not use a generic repository interface when use cases need domain-specific
  operations.

## Construction and state

- Dependencies must be passed explicitly through constructors.
- Constructors must return initialized values that are ready to use.
- Mutable package-level state is prohibited.
- Runtime configuration must be loaded once, validated, and injected.
- The composition root is responsible for choosing concrete adapters.
- Avoid service locators and hidden dependency lookups.

## Context

- Functions that perform I/O must accept `context.Context` as their first
  parameter.
- Do not store a context in a struct.
- Propagate request cancellation and deadlines to database calls.
- Do not replace a request context with `context.Background()`.
- Use `context.Background()` only at deliberate process-level boundaries, such as
  controlled startup or shutdown.

## Errors

- Every returned error must be handled or deliberately propagated.
- Add useful context with `fmt.Errorf("operation: %w", err)`.
- Preserve error identity with `%w` and inspect it using `errors.Is` or
  `errors.As`.
- Use stable domain or application error categories for expected outcomes such as
  validation failures and missing resources.
- Translate errors at boundaries: PostgreSQL errors in the repository,
  application errors in HTTP handlers.
- Do not log and return the same error at every layer. Log unexpected errors once
  at the boundary that has enough request context.
- `panic` is not normal error handling.

## Domain and validation

- The domain must own business invariants and priority rules.
- HTTP decoding may validate syntax and shape, but a valid DTO must not bypass
  domain validation.
- Database constraints must mirror important invariants without becoming the
  primary implementation of business behavior.
- The priority engine should be pure: identical input produces identical output
  without I/O or hidden time dependencies.
- Stock and day quantities use integers.
- Money and fractional business values use an exact decimal representation.
- `float64` must not represent money.
- Sorting must include all normative tie-breakers and the documented deterministic
  fallback.

## HTTP

- Use `net/http` unless an approved specification establishes a framework need.
- Keep handlers small: decode, invoke a use case, and encode.
- Limit request body size and reject unknown JSON fields.
- Write a response status once and return immediately after failures.
- Use a consistent JSON error envelope and stable error codes.
- Never expose internal error messages, SQL, credentials, or stack traces.
- Configure read, header, write, idle, and shutdown timeouts.
- Middleware must have one clear responsibility.

## PostgreSQL

- Use parameterized SQL for every value.
- Pass the caller's context to every query.
- Close rows and check iteration errors.
- Keep transactions bounded and always arrange rollback before attempting work.
- Repositories translate records to domain values rather than returning database
  models.
- Schema changes require versioned migrations committed with the behavior that
  needs them.
- Queries used by core flows must be covered by integration tests against
  PostgreSQL.

## Concurrency

- Prefer synchronous code until concurrent work has a measured benefit.
- Every goroutine must have an owner, a cancellation path, bounded resource use,
  and a shutdown strategy.
- Do not share mutable memory when message passing or sequential execution is
  simpler.
- Run tests with the race detector in CI.

## Logging

- Use `log/slog` structured attributes rather than formatted log strings.
- Include request ID, operation, status, and duration where relevant.
- Logs should explain what failed and where without duplicating the complete error
  chain at every layer.
- Do not log secrets, database URLs, authorization data, or full request bodies.
- Libraries and domain code should generally return errors instead of logging.

## Testing

- Write table-driven tests for business rule combinations.
- Name test cases after behavior, not implementation details.
- Use `t.Run` and use `t.Parallel` only when state is truly isolated.
- Pure domain tests should not use mocks.
- Application tests should use small, explicit fakes owned by the consuming test
  package.
- HTTP tests should use `httptest` and assert public contracts.
- Persistence tests should exercise a real PostgreSQL instance.
- Tests must not depend on execution order, arbitrary sleeps, or the developer's
  local data.
- Every acceptance criterion in an approved specification must map to one or more
  automated tests or an explicitly documented verification step.

## Tooling and quality gates

Before a change is complete:

```text
gofmt or gofmt verification
go vet ./...
lint
go test ./...
go test -race ./...
integration tests
```

Generated files, if introduced later, must be reproducible and clearly marked.
Formatting or generated output must not hide unrelated changes in a feature pull
request.

## Documentation

- Comments should explain intent, constraints, or non-obvious trade-offs.
- Avoid comments that merely restate code.
- Exported APIs should have useful Go documentation when their purpose is not
  evident from a narrow internal scope.
- Behavior changes must update their specification before implementation begins.
- Durable architectural changes must update the system design and may require an
  ADR.

