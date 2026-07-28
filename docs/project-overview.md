# Project Overview

## Context

An automotive parts distributor must decide which parts should be replenished
first. Available stock and working capital are limited, while parts have different
sales patterns, supplier lead times, and operational criticality.

The Restock Priority Service manages the inventory inputs required for that
decision and produces a deterministic ranking of parts that need replenishment.

This document is a non-normative introduction. The
[core service specification](sdd/specs/001-core-service.md) is the source of truth
for behavior and contracts.

## Problem statement

Inventory operators need a consistent way to identify parts that are expected to
fall below their desired minimum stock before suppliers can replenish them.
Manual prioritization is slow, difficult to audit, and prone to inconsistent
judgment.

The service solves this problem by applying an explicit formula to every part and
returning only the parts that need restocking, ordered by urgency.

## Goals

- Manage the inventory and replenishment attributes of automotive parts.
- Identify parts whose projected stock falls below their minimum stock.
- Rank those parts using deterministic business rules.
- Keep business calculations independent from HTTP and persistence concerns.
- Support hundreds or thousands of parts without unnecessary infrastructure.
- Make a future database replacement possible through an application-owned
  repository boundary.
- Provide a codebase that is easy to test, review, and evolve through
  Spec-Driven Development.

## Stakeholders

- **Inventory operator:** maintains part information and consumes the priority
  ranking.
- **Operations manager:** expects the ranking to be explainable and repeatable.
- **Engineering team:** maintains the service and evolves its rules.
- **Technical evaluator:** verifies domain modeling, architecture, tests, and
  delivery quality.

## Scope summary

The first version provides:

- Create, read, update, delete, and list operations for parts.
- Optional filtering of the part list by category.
- A restock priority endpoint.
- Validation for inventory, sales, lead-time, cost, and criticality inputs.
- PostgreSQL persistence behind a repository port.
- Health and readiness signals for local and containerized operation.
- Automated domain, HTTP, application, and persistence tests.

## Non-functional expectations

- Deterministic results for identical persisted data.
- Exact decimal arithmetic where business values may be fractional.
- Clear separation between transport, application, domain, and persistence.
- Predictable behavior for negative stock and unusually high lead times.
- Structured diagnostics without exposing internal failures to API consumers.
- Graceful startup and shutdown.
- Local execution without external APIs.

## Out of scope for the first version

- Authentication and authorization.
- Supplier management or purchase-order creation.
- A maximum restocking budget or capital allocation algorithm.
- Recommended purchase quantities.
- Reservations, warehouses, or stock movements.
- Cache, event bus, message queues, CQRS, or background ranking jobs.
- Webhooks or integrations with third-party services.
- Cloud infrastructure or production deployment automation.

## Business terminology

| Term | Meaning |
| --- | --- |
| Part | An automotive inventory item that can be replenished. |
| Current stock | The quantity currently available. It may be negative when recorded demand exceeds available inventory. |
| Minimum stock | The desired lower stock boundary. |
| Average daily sales | The expected number of units sold per day. |
| Lead time | The number of days a supplier needs to deliver a replenishment. |
| Expected consumption | Estimated sales during the supplier lead time. |
| Projected stock | Expected remaining stock at the end of the lead time. |
| Criticality | An integer from 1 to 5 representing operational importance. |
| Urgency score | The shortage relative to minimum stock, weighted by criticality. |

## Challenge example discrepancy

The challenge includes an example with a part whose current stock is `15`, average
daily sales is `4`, lead time is `5`, minimum stock is `20`, and criticality is
`3`. Applying the stated formulas produces:

```text
expectedConsumption = 4 × 5 = 20
projectedStock = 15 - 20 = -5
urgencyScore = (20 - (-5)) × 3 = 75
```

The example response instead shows a projected stock of `5` and an urgency score
of `45`. The written formulas are authoritative. The implementation and tests
must therefore produce `-5` and `75` for those inputs. This discrepancy must not
be silently reproduced in code.

