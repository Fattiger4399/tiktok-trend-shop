## 1. Go Project Foundation

- [x] 1.1 Add Go module metadata and command layout.
- [x] 1.2 Add runtime configuration for database URL and HTTP address.
- [x] 1.3 Add SQLite connection and migration support for core product tables.

## 2. Product Storage

- [x] 2.1 Define Go product, source snapshot, and detail snapshot models.
- [x] 2.2 Implement product repository upsert and list queries.
- [x] 2.3 Implement latest detail and metrics projection for product discovery.

## 3. CSV Ingestion

- [x] 3.1 Implement CSV import compatible with existing ranking columns.
- [x] 3.2 Implement detail enrichment parsing for product URL, platform, image, price, selling points, specs, and review summary.
- [x] 3.3 Add CLI output for imported rows and product IDs.

## 4. HTTP API

- [x] 4.1 Add a minimal HTTP server command.
- [x] 4.2 Add health and product listing endpoints.
- [x] 4.3 Return JSON errors and stable response shapes.

## 5. Documentation and Verification

- [x] 5.1 Document Go commands and staged migration policy.
- [x] 5.2 Add Go unit tests for migration, CSV import, and product listing.
- [x] 5.3 Run `go test ./...`.
- [x] 5.4 Run `openspec validate migrate-core-backend-to-go`.
