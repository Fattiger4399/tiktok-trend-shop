## Context

The repository currently contains a Python standard-library implementation that covers ingestion, scoring, enrichment, creative scripts, placeholder video generation, review, export, and analytics. The next product direction requires a customer-facing selection workbench and long-running background tasks, and the maintainer prefers Go.

## Goals / Non-Goals

**Goals:**
- Introduce Go as the production backend foundation without deleting Python.
- Preserve the existing SQLite-oriented local workflow during migration.
- Provide a small but real Go path for migration, CSV import, product query, and HTTP product listing.
- Keep package boundaries aligned with the future SaaS architecture.

**Non-Goals:**
- Rebuild the full AI script generator in Go in this change.
- Rebuild video rendering in Go in this change.
- Add auth, tenancy, billing, or a React workbench in this change.
- Change the existing Python CLI behavior.

## Decisions

- Use a staged migration. A big-bang rewrite would erase useful prototype behavior and slow down product discovery.
- Use a Go modular monolith layout under `cmd/` and `internal/`. This keeps deployment simple while preserving package boundaries.
- Start with SQLite compatibility. PostgreSQL will be the production target, but SQLite keeps local validation and existing datasets easy to inspect.
- Use `database/sql` with a SQLite driver. Repository APIs should hide the driver so PostgreSQL can be introduced later.
- Use the Go standard `net/http` package for the first HTTP API. Framework selection can wait until route complexity justifies it.
- Keep CSV import compatible with existing Python CSV columns and the new detail enrichment columns.

## Risks / Trade-offs

- Maintaining Python and Go temporarily duplicates behavior -> keep the Go first slice small and focused on product discovery only.
- SQLite driver dependency adds build complexity -> use a pure-Go driver to avoid local CGO setup on Windows.
- Go product import may diverge from Python scoring/video behavior -> do not migrate scoring/video until the selection workbench API stabilizes.
- Existing SQLite files may have Python-created schemas -> use `CREATE TABLE IF NOT EXISTS` and compatible column names.

## Migration Plan

1. Add Go module, config, store, product repository, CSV importer, CLI, HTTP API, and tests.
2. Use Go against a separate local database by default.
3. Allow `TTS_DATABASE_URL` to point at existing SQLite files for read/query validation.
4. Migrate product selection workbench to the Go API in a later change.
5. Migrate scoring, creative generation, and video workers after the Go API is stable.
