# Task 1 Report: Database Schema, Ent Models, and Migration

## Status

DONE

## Delivered

- Added idempotent migration `backend/migrations/172_zenxiang_liyu.sql` for settings, prizes, user grants, and immutable play records.
- Added Ent schemas for all four Zenxiang Liyu entities with matching table annotations, decimal PostgreSQL column types, timestamps, hard-delete semantics, and required indexes.
- Added the Zenxiang Liyu grant and record edges to `User`, then regenerated `backend/ent`.

## Verification

- `cd backend && GOCACHE=/tmp/sub2api-go-cache go generate ./ent` passed.
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./ent ./internal/repository -run 'TestMigrations|Test.*Schema' -count=1` passed.
- `git diff --check -- backend/ent backend/migrations/172_zenxiang_liyu.sql` passed.

## Commit

- `dc3fc0a29 feat: add zenxiang liyu schema`

## Self-review

- Confirmed `zenxiang_liyu_records.request_id` and `zenxiang_liyu_user_grants.user_id` are unique in both migration and Ent schema.
- Confirmed the migration retains the requested foreign-key delete behavior, check constraints, singleton settings seed, and query indexes.
- No files outside the Task 1 ownership scope will be staged.

## Concerns

None.

---

## Task 1 Schema Review Fixes

### Changes

- Added the singleton settings check constraint and advanced `zenxiang_liyu_settings_id_seq` after the seed row.
- Added matching Ent singleton metadata and explicit foreign-key delete actions for user grants, records, prizes, and grant authors.
- Regenerated `backend/ent` after updating the schemas.

### Verification

- `cd backend && GOCACHE=/tmp/sub2api-go-cache go generate ./ent` passed.
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./ent ./internal/repository -run 'TestMigrations|Test.*Schema' -count=1` passed.
- `git diff --check -- backend/ent backend/migrations/172_zenxiang_liyu.sql .superpowers/sdd/task-1-report.md` passed.

---

## Task 1 Second Review Fixes

### Changes

- Restored committed migration `172_zenxiang_liyu.sql` to the exact content from `dc3fc0a29`.
- Added forward migration `173_zenxiang_liyu_settings_singleton_fix.sql` to idempotently add the settings singleton constraint, align the settings ID sequence, and prevent `UPDATE` and `DELETE` on `zenxiang_liyu_records` with a database trigger.
- Retained the Ent singleton metadata and explicit foreign-key delete actions. Generated Ent update/delete builders remain intentionally available; the database trigger enforces record immutability.

### Verification

- `cd backend && GOCACHE=/tmp/sub2api-go-cache go generate ./ent` passed without generated Ent changes.
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./ent ./internal/repository -run 'TestMigrations|Test.*Schema' -count=1` passed; no matching tests are enabled without build tags.
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go test -tags integration ./internal/repository -run '^TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate$' -count=1` completed successfully but skipped because Docker is unavailable.
- `git diff --check -- backend/ent backend/ent/schema backend/migrations/172_zenxiang_liyu.sql backend/migrations/173_zenxiang_liyu_settings_singleton_fix.sql` passed.

### Concern

- The database-backed integration test did not run in this environment because Docker is unavailable; run it with Docker available to validate the forward migration against PostgreSQL.

---

## Task 1 Third Review Fixes

### Changes

- Added forward migration `174_zenxiang_liyu_record_fk_immutability_fix.sql`; it replaces the records-to-users foreign key with `ON DELETE RESTRICT`, preserving immutable ledger rows by preventing deletion of a referenced user.
- Replaced the record immutability trigger function so only the database-managed prize delete action (`prize_id` non-null to `NULL`, with every other field unchanged) is permitted. Record deletes and all other updates remain rejected.
- Updated the `User` Ent edge for Zenxiang Liyu records to `entsql.Restrict`; generated Ent metadata is regenerated to match the database constraint.

### Verification

- `cd backend && GOCACHE=/tmp/sub2api-go-cache go generate ./ent` passed.
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./ent ./internal/repository -run 'TestMigrations|Test.*Schema' -count=1` passed; no matching unit tests are enabled.
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go test -tags integration ./internal/repository -run '^TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate$' -count=1 -v` completed successfully but skipped because Docker is unavailable.
- `git diff --check` passed.

### Concern

- Run the integration migration test with Docker available to validate the `174` foreign-key replacement and trigger behavior against PostgreSQL.

---

## Task 1 Fourth Review Fix

### Changes

- Restricted the `prize_id` clearing exception in `174_zenxiang_liyu_record_fk_immutability_fix.sql` to nested PostgreSQL trigger execution (`pg_trigger_depth() > 1`), so direct record updates at trigger depth 1 remain rejected. The existing non-null-to-null and no-other-columns-changed checks are retained.

### Verification

- Static inspection confirms the trigger exception now requires `pg_trigger_depth() > 1`, contains no `pg_trigger_depth() > 0` predicate, and retains `TG_OP = 'UPDATE'`; no local PostgreSQL SQL parser or formatter is installed.
- PostgreSQL integration remains pending Docker availability to validate direct update rejection, FK-driven `prize_id` nulling, and delete rejection.
