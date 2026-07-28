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

The [project overview](project-overview.md) provides context and a non-normative
summary. If it conflicts with the active specification, the specification wins.

## Recommended reading order

1. [Project overview](project-overview.md)
2. [Core service specification](sdd/specs/001-core-service.md)
3. [System design](architecture/system-design.md)
4. [Go engineering guidelines](engineering/go-guidelines.md)
5. [Engineering anti-patterns](engineering/anti-patterns.md)
6. [Spec-Driven Development workflow](sdd/README.md)

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

