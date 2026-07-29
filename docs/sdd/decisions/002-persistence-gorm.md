# ADR-002: Use GORM for the PostgreSQL adapter, with explicit mapping and SQL migrations

| Field | Value |
| --- | --- |
| Status | Accepted |
| Date | 2026-07-28 |
| Decision owners | Restock Priority Service team |
| Related specifications | SPEC-001 |
| Supersedes | None |

## Context

`docs/architecture/system-design.md` names `gorm.io/gorm` and `gorm.io/driver/postgres`
in the technology baseline. Several normative rules sit uneasily with an ORM's defaults:

- `go-guidelines.md` requires "parameterized SQL for every value" and that
  "repositories translate records to domain values rather than returning database
  models".
- `anti-patterns.md` forbids "runtime schema auto-synchronization as a substitute for
  reviewed, versioned SQL migrations", and forbids exposing SQL null wrappers or
  driver types through the ports.
- SPEC-001 requires migrations to be "ordered, versioned, and reproducible", and
  requires schema constraints mirroring the domain invariants.

The implementation had drifted from all of these in one place: startup called
`db.AutoMigrate(&PartModel{})`, and `PartModel` carries no `check:` tags, so the live
table was created without any of the five CHECK constraints the specification requires.
The reviewed SQL migration in `migrations/` was never executed — it carried goose
annotations while goose was not a dependency.

A second defect came from an ORM default rather than from the schema: `Updates` with a
struct skips zero-valued fields, so a full replacement setting `currentStock`,
`minimumStock` or `leadTimeDays` to `0` silently kept the previous value while the
handler returned `200` describing the requested state.

No ADR recorded the persistence choice, though `docs/sdd/README.md` requires one for a
"durable infrastructure or data-storage choice".

## Decision drivers

- The database must be replaceable, which SPEC-001 places behind the application-owned
  `PartRepository` port rather than behind an ORM abstraction.
- Schema shape must come from reviewed, versioned SQL, not from struct tags interpreted
  at runtime.
- An ORM default must never be able to change the meaning of a documented operation.

## Considered options

### Option A: Keep GORM, constrain its use

Keep GORM for connection handling, scanning and query building, and remove the two
places where its defaults conflicted with the contract: replace `AutoMigrate` with a
migration runner over the reviewed SQL, and name the mutable columns explicitly on
update so zero values are written.

Explicit `PartModel.ToDomain` and `FromDomain` mappers already exist, so no database type
crosses the port.

### Option B: Replace GORM with `database/sql`

Hand-written parameterized SQL in the repository, roughly 150 lines. This removes the
zero-value trap and the statement-reuse footgun by construction, and matches
`go-guidelines.md` most literally.

Cost: rewriting a repository that, once the two defects above are fixed, satisfies its
contract and is covered by tests. Evaluated against the same criteria, it trades a
working layer for the removal of a class of default-driven surprises.

## Decision

The PostgreSQL adapter uses GORM, bounded by four constraints that are not optional:

1. **Schema comes from SQL.** `AutoMigrate` is not used. `cmd/migrate` applies the
   ordered files in `migrations/` with `github.com/pressly/goose/v3`, embedded into the
   binary. Migrations run as their own step before the API accepts traffic — never at
   startup and never during request handling. The API process does not alter the schema.
2. **Updates name their columns.** `Update` passes an explicit column list to `Select`,
   because `Updates` with a struct skips zero-valued fields. `id` and `created_at` are
   excluded; `updated_at` is written by `autoUpdateTime`.
3. **Types do not cross the port.** `PartModel` stays inside
   `internal/adapter/postgres`. `ToDomain` and `FromDomain` are the only translation, and
   the port speaks `domain.Part` and `uuid.UUID` only.
4. **Each finisher gets a fresh statement.** `List` builds its query twice rather than
   reusing one `*gorm.DB` across `Count` and `Find`, so conditions cannot leak between
   them.

## Consequences

### Positive

- The live schema now carries the five CHECK constraints as the documented second line
  of defence, verified by inserting invalid rows directly.
- `current_stock` remains unconstrained, so BR-010 holds at the database level too.
- Migrations are ordered, versioned and reproducible, and the same binary applies them
  locally and in the container.
- Full replacement writes zero values, so the response describes what was stored.

### Negative

- GORM's defaults must be reviewed per operation rather than trusted. Both defects above
  were defaults, not misuse.
- The dependency is heavier than `database/sql` for a single-table schema.
- Constraint 2 means adding a mutable column requires updating `mutableColumns`. The
  comment on that variable says so.

### Follow-up

- Constraint 2 has an automated regression gate that needs no database:
  `internal/adapter/postgres/part_repository_test.go` builds the update statement under
  GORM's `DryRun` and asserts that every mutable column derived from `PartModel` appears
  in the `SET` clause, including the zero-valued ones. Removing the explicit column list
  fails it. That test also fails if `PartModel` grows a field the adapter does not write,
  which is the drift risk named above.
- Row-level behaviour against a real instance is still proven by documented manual
  verification. Automated integration tests remain the gap to close, and are the natural
  home for regression cover on constraint 4.
- Connection pool limits are not yet configured.

## Reversal strategy

Replacing GORM means rewriting `internal/adapter/postgres` only. The seam is
`application.PartRepository`, which mentions no database type, and
`internal/adapter/memory` already demonstrates a second implementation of it. The SQL
migrations are driver-independent and carry over as they are, so a replacement inherits
the schema and its constraints unchanged.

Reconsider if a further ORM default alters documented behaviour, if the query shapes
outgrow the builder, or once integration tests make a rewrite cheap to validate.

## Status history

| Date | Status | Note |
| --- | --- | --- |
| 2026-07-28 | Accepted | Records the choice and the constraints that make it safe |
