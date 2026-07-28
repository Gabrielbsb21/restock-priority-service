# Spec-Driven Development Workflow

Spec-Driven Development (SDD) makes an approved specification the input to
implementation rather than a retrospective description of it. Every behavior
change begins as a reviewable contract containing requirements, edge cases, and
acceptance criteria.

## Principles

1. **Specify before implementing.** Production code does not begin while behavior
   is undefined.
2. **One source of truth.** A requirement is normative in one specification and
   referenced elsewhere.
3. **Make behavior verifiable.** Requirements use stable IDs and map to acceptance
   criteria and tests.
4. **Resolve ambiguity deliberately.** Open questions remain visible and block
   approval when they affect observable behavior.
5. **Keep specifications current.** A behavior change modifies its specification
   first.
6. **Record durable decisions.** Cross-cutting architectural choices use an
   Architecture Decision Record (ADR).

## Specification lifecycle

```text
Draft → In Review → Approved → Implementing → Verified
```

### Draft

The author defines the problem, scope, requirements, business rules, contracts,
failure behavior, and acceptance criteria. Open questions are allowed and must be
listed explicitly.

Exit conditions:

- Goals and non-goals are clear.
- Functional requirements and business rules have stable IDs.
- Public contracts and affected data are described.
- Important edge cases are covered.
- Acceptance criteria are observable.
- Material open questions are ready for review.

### In Review

Reviewers examine product intent, domain behavior, interfaces, data flow,
operational impact, testability, and consistency with existing specifications.
Implementation must not begin.

Exit conditions:

- Material open questions are resolved.
- Conflicts with active specifications are resolved.
- Acceptance criteria cover the intended behavior.
- Architectural consequences are captured in the system design or an ADR.
- Review feedback is incorporated.

### Approved

The specification is decision-complete. An implementer can execute it without
making product or architectural decisions.

Moving to `Approved` is an explicit review action. Approval means scope and
behavior are stable enough for implementation, not that wording can never
improve.

### Implementing

Code, migrations, tests, and operational changes are developed against the
approved specification.

Rules:

- Pull requests and commits reference the specification ID.
- Test names or test documentation identify the acceptance criteria they verify.
- New ambiguity pauses implementation and moves the specification back to
  `Draft` or `In Review`.
- Scope must not grow silently during implementation.

### Verified

The implemented behavior satisfies all acceptance criteria, automated quality
checks pass, documentation is current, and any manual verification evidence is
recorded.

A verified specification remains active until superseded or deprecated by a
later approved specification.

## Starting a feature

1. Copy [the feature specification template](templates/feature-spec.md).
2. Store it in `specs/` using `<sequence>-<descriptive-name>.md`.
3. Use the next available three-digit sequence, for example
   `002-priority-pagination.md`.
4. Set its status to `Draft`.
5. Assign stable requirement IDs within that specification:
   - `FR-###` for functional requirements.
   - `BR-###` for business rules.
   - `AC-###` for acceptance criteria.
6. Complete review and approval before modifying production behavior.

IDs are local to a specification. When referencing them elsewhere, include the
specification ID, for example `SPEC-001/BR-004`.

## Changing an active specification

For a clarification that does not change observable behavior, update the document
and record the clarification in its change log.

For an observable behavior change:

1. Create a new specification that references the superseded behavior, or move
   the active specification back to `Draft` when implementation has not begun.
2. Describe compatibility and migration implications.
3. Review and approve the change.
4. Implement only after approval.
5. Update affected architecture and engineering documentation by reference.

Do not rewrite the history of an already verified decision in a way that hides
why the system behaved as it did.

## Architecture Decision Records

Use the [ADR template](templates/adr.md) when a decision:

- Changes dependency direction or a system boundary.
- Introduces a durable infrastructure or data-storage choice.
- Changes a public integration style.
- Has difficult or costly reversal consequences.
- Resolves a recurring architectural disagreement.

An ADR does not replace a feature specification. Specifications define required
behavior; ADRs explain important architectural choices and consequences.

ADR states are `Proposed`, `Accepted`, `Superseded`, and `Rejected`. Store future
ADRs under `docs/sdd/decisions/` using `<sequence>-<descriptive-name>.md`.

## Traceability

Every approved specification must provide a verification matrix:

| Requirement | Acceptance criteria | Verification |
| --- | --- | --- |
| `FR-001` | `AC-001` | Test name, command, or manual check |

During `Draft` and `In Review`, the verification column may describe the intended
test level. Before moving to `Verified`, it must identify the actual automated
test or documented manual evidence.

## Definition of done

A specification may move to `Verified` only when:

- Every acceptance criterion has evidence.
- Domain rules have focused unit tests.
- Public contracts have HTTP-level tests.
- Persistence behavior has integration coverage where applicable.
- Formatting, static analysis, race detection, and test suites pass.
- Migrations and operational configuration are documented and reproducible.
- No implementation behavior contradicts the specification.
- The specification, system design, and engineering documentation contain no
  unresolved drift.

## Current specification

[SPEC-001: Core Service](specs/001-core-service.md) defines the first version of
the Restock Priority Service.

