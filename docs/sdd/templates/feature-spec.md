# SPEC-NNN: Feature Name

| Field | Value |
| --- | --- |
| Status | Draft |
| Authors | Name or team |
| Created | YYYY-MM-DD |
| Updated | YYYY-MM-DD |
| Target version | Version or milestone |
| Related ADRs | None |
| Supersedes | None |

## Context

Describe the relevant current behavior and why a change is being considered.
Link to existing specifications instead of copying their normative requirements.

## Problem statement

State the observable problem from the stakeholder's perspective. Explain who is
affected and why the current behavior is insufficient.

## Goals

- Define the outcomes this specification must achieve.

## Non-goals

- Define adjacent behavior that is intentionally excluded.

## Functional requirements

- **FR-001:** Write one observable, testable requirement.
- **FR-002:** Use **must** for mandatory behavior and avoid implementation detail
  unless it is part of the contract.

## Business rules

- **BR-001:** State a deterministic rule, formula, invariant, or ordering.
- **BR-002:** Define boundary values and tie behavior explicitly.

## API contracts

Describe affected methods, paths, query parameters, request bodies, successful
responses, error responses, status codes, and compatibility constraints.

Use examples to clarify a contract, not to replace normative rules.

## Data model

Describe new or changed domain fields, types, invariants, persistence constraints,
indexes, and migration requirements.

Write `No data-model changes` when this section is not applicable.

## Data flow and architecture

Describe affected components and dependency boundaries. Link to the
[system design](../../architecture/system-design.md) and identify any required
ADR.

## Failure modes and edge cases

| Case | Expected behavior |
| --- | --- |
| Invalid input | Define the client-visible result. |
| Dependency failure | Define classification, response, and diagnostics. |
| Boundary value | Define inclusive or exclusive behavior. |

## Acceptance criteria

- **AC-001:** Given a defined initial state, when an action occurs, then an
  observable result follows.
- **AC-002:** Include negative cases and boundary behavior.

## Test strategy

Describe required unit, application, HTTP, integration, and manual verification.
Prefer the lowest test level that proves each behavior reliably.

## Verification matrix

| Requirement | Acceptance criteria | Planned verification |
| --- | --- | --- |
| FR-001 | AC-001 | Test level and intended test name |

Replace planned verification with actual evidence before changing the status to
`Verified`.

## Implementation tasks

- [ ] Domain changes and unit tests.
- [ ] Application changes and tests.
- [ ] Adapter or migration changes and integration tests.
- [ ] Contract and documentation updates.
- [ ] Quality gates and acceptance verification.

Tasks describe delivery order; they must not introduce behavior absent from the
approved requirements.

## Rollout and compatibility

Describe deployment order, backward compatibility, migration safety, rollback,
and observability. Write `No special rollout requirements` when appropriate.

## Open questions

- List unresolved decisions that affect scope or observable behavior.
- Write `None` before approval.

## Change log

| Date | Change | Author |
| --- | --- | --- |
| YYYY-MM-DD | Initial draft | Name or team |

