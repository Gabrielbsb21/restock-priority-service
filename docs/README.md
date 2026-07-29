# Restock Priority Service Documentation

This directory contains the product specification, system design, and engineering
standards for the Restock Priority Service.

## Sources of truth

Each kind of decision has one authoritative document:

| Subject | Source of truth |
| --- | --- |
| Product behavior, API contracts, business rules, and acceptance criteria | [Core service specification](sdd/specs/001-core-service.md) |
| Architecture, dependency direction, and runtime design | [System design](architecture/system-design.md) |
| Go implementation standards | [Go engineering guidelines](engineering/go-guidelines.md) |
| Prohibited practices and common failure modes | [Engineering anti-patterns](engineering/anti-patterns.md) |
| Specification lifecycle and change process | [Spec-Driven Development workflow](sdd/README.md) |
| Durable technology choices and their constraints | [Architecture decision records](sdd/decisions/) |
| Machine-readable HTTP contract, for tooling | [OpenAPI document](../api/openapi.yaml), served at `/openapi.yaml` with a Swagger UI at `/docs` |

The [project overview](project-overview.md) provides context and a non-normative
summary. If it conflicts with the active specification, the specification wins.

## Recommended reading order

1. [Project overview](project-overview.md)
2. [Core service specification](sdd/specs/001-core-service.md)
3. [System design](architecture/system-design.md)
4. [Go engineering guidelines](engineering/go-guidelines.md)
5. [Engineering anti-patterns](engineering/anti-patterns.md)
6. [Spec-Driven Development workflow](sdd/README.md)

## Accepted decisions

| ADR | Decision |
| --- | --- |
| [ADR-001](sdd/decisions/001-http-framework-gin.md) | Gin is the HTTP transport adapter, with Gin types confined to it |
| [ADR-002](sdd/decisions/002-persistence-gorm.md) | GORM is the PostgreSQL adapter, with explicit mapping, explicit update columns, and SQL migrations |
| [ADR-003](sdd/decisions/003-openapi-documentation.md) | The contract is a hand-written OpenAPI document kept in step with the router by test, served with an embedded Swagger UI |

## Starting a change

Behavior changes begin with a specification:

1. Copy the [feature specification template](sdd/templates/feature-spec.md).
2. Give it the next sequential identifier and a descriptive kebab-case name.
3. Keep it in `Draft` until requirements, edge cases, and acceptance criteria are
   reviewable.
4. Move it through the lifecycle defined by the
   [SDD workflow](sdd/README.md).

Use the [ADR template](sdd/templates/adr.md) when a change introduces a durable,
cross-cutting architectural decision that is not already covered by the system
design.

