# Engineering Anti-Patterns

This document records practices that must not be introduced into the Restock
Priority Service without a new, approved architectural decision. It complements
the [Go engineering guidelines](go-guidelines.md).

## Importing another ecosystem's architecture

Do not reproduce a Node.js framework structure by creating controllers, services,
providers, modules, factories, and interfaces that only forward calls. Go favors
small packages, explicit construction, and concrete types until a boundary has a
real reason to exist.

Warning signs:

- One interface for every struct.
- Constructors that only feed a dependency injection container.
- Multiple layers whose methods repeat the same signature and behavior.
- Package names based solely on framework roles rather than business cohesion.

## Speculative abstraction

Do not create generic repositories, factories, plugin systems, or extension
points for possible future requirements. The required database replacement seam
is the application-owned `PartRepository`, not a universal persistence
framework.

Generics must not be used merely to remove a few repeated lines when doing so
obscures domain language or error behavior.

## Business logic in adapters

Do not calculate restock priorities in:

- HTTP handlers or middleware.
- SQL queries or database triggers.
- JSON DTO methods.
- PostgreSQL record mapping.

Adapters translate and perform I/O. The pure domain engine owns the formulas,
eligibility rule, and ordering.

## Leaking boundary models

Do not expose PostgreSQL records directly as HTTP responses or decode request JSON
straight into persistence structs. Doing so couples public contracts to schema
details and makes database replacement unsafe.

Do not expose `pgx`, SQL null wrappers, or transport-specific types through domain
or application interfaces.

## Unsafe numeric choices

Do not use `float32` or `float64` for money or exact priority tie-breaking. Binary
floating-point representation can produce unexpected comparisons and serialized
values.

Do not clamp negative current stock to zero. Negative stock is a supported
business input and must influence projected stock and urgency.

## Hidden global behavior

Do not use mutable package-level variables, global database handles, singleton
service registries, or configuration lookups scattered through the codebase.

Do not use `init` functions for application wiring, database access, migrations,
or hidden registration.

## Incorrect error handling

Do not:

- Ignore returned errors.
- Compare wrapped errors by their message.
- Use `panic` for validation or missing resources.
- Call `log.Fatal` outside the process entry point.
- Log the same failure independently in every layer.
- Return raw database or internal error messages to clients.
- Continue after writing an HTTP error response.

## Broken context propagation

Do not store `context.Context` in services or repositories. Do not create a
background context inside a request path to avoid handling cancellation. A
cancelled request must cancel its database work.

## Unowned concurrency

Do not start a goroutine unless its lifecycle, cancellation, error propagation,
resource bounds, and shutdown behavior are explicit.

Do not add worker pools, channels, parallel ranking, or background refresh loops
without measurements showing a need. Thousands of in-memory calculations do not
justify distributed or concurrent complexity by themselves.

## Premature infrastructure

Do not add cache, event bus, queues, CQRS, event sourcing, microservice-to-
microservice communication, or cloud-specific infrastructure to the first
version. Each would expand failure modes without satisfying an existing
requirement.

Do not run migrations inside request handling. Do not use runtime schema
auto-synchronization as a substitute for reviewed, versioned SQL migrations.

## Dumping-ground packages

Do not create broad packages named `utils`, `common`, `shared`, `helpers`, or
`misc`. Put behavior with the concept that owns it. A reusable helper should
remain local until multiple concrete consumers establish a coherent package.

## Fragile tests

Do not:

- Assert private implementation details instead of public behavior.
- Use arbitrary sleeps to wait for asynchronous behavior.
- Depend on test execution order.
- Share mutable fixtures between parallel tests.
- Mock pure functions or simple domain values.
- Replace every dependency with a generated mock when a small fake is clearer.
- Skip extreme cases required by the active specification.

## Documentation drift

Do not implement a behavior change first and update documentation afterward.
Under the [SDD workflow](../sdd/README.md), the specification changes first and
must reach `Approved` before implementation.

Do not duplicate normative requirements across overview, design, and engineering
documents. Link to the owning source of truth instead.

Do not copy the mathematically inconsistent challenge example into tests. The
written formulas defined by the active specification are authoritative.

