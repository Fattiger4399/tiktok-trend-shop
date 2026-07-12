## Why

The Python prototype has validated the product-trend-to-video workflow, but the long-term product will be maintained by a Go-oriented developer and needs a stable API, worker, and deployment foundation. This change starts a staged migration to Go while keeping the Python implementation as a reference and fallback.

## What Changes

- Add a Go backend module with CLI entry points for migration, CSV import, product listing, and HTTP serving.
- Recreate the core product, source snapshot, and product detail storage needed for the selection workbench.
- Add a minimal HTTP API for health and product discovery.
- Keep the existing Python implementation in place during migration.
- Establish Go project layout, tests, and documentation for future API/workbench/video-worker changes.

## Capabilities

### New Capabilities
- `go-core-backend`: Go runtime foundation, database access, CSV ingestion, product query, and minimal HTTP API.

### Modified Capabilities
- None.

## Impact

- Adds Go module files, Go command binaries, internal packages, and Go tests.
- Adds one SQLite driver dependency for local persistence.
- Does not remove or rewrite existing Python modules in this change.
- Future changes can migrate scoring, creative generation, video rendering, auth, and the product workbench onto the Go foundation.
