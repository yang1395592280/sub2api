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
