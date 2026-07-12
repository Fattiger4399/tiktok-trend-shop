## 1. Database and Domain Foundation

- [x] 1.1 Add additive SQLite migrations for Amazon marketplace and ASIN identity, canonical categories, source-category mappings, product category assignments, hotspot score snapshots, and import jobs.
- [x] 1.2 Extend Go product models and repository projections with optional marketplace, ASIN, source category, canonical category, provenance, freshness, and score summary fields.
- [x] 1.3 Enforce unique marketplace-plus-ASIN identity when both values are present while preserving legacy products without Amazon identity.
- [x] 1.4 Add repository tests covering migration compatibility, legacy records, Amazon identity deduplication, and latest-detail projection.

## 2. Catalog Classification

- [x] 2.1 Seed the initial canonical category hierarchy with stable IDs, parent relationships, active state, and display order.
- [x] 2.2 Implement repositories for canonical categories, source-category mappings, product assignments, and unresolved review queries.
- [x] 2.3 Implement classification order for exact source mapping, deterministic keyword fallback, confidence thresholds, and manual-review status.
- [x] 2.4 Implement manual category assignment with automatic-assignment override protection and audit recording.
- [x] 2.5 Add classification tests for exact mappings, fallback rules, low-confidence review, canonical filtering, and manual overrides.

## 3. Trend History and Hotspot Scoring

- [x] 3.1 Extend snapshot normalization to distinguish observed, estimated, and derived metrics and retain provider and captured-time provenance.
- [x] 3.2 Implement chronological metric-history repository queries for configurable time windows.
- [x] 3.3 Implement the versioned category-relative hotspot model with demand, acceleration, review growth, price signal, freshness, and completeness components.
- [x] 3.4 Persist hotspot score snapshots with model version, comparison group, time window, confidence, missing components, and evidence references.
- [x] 3.5 Add scoring tests for complete history, single-snapshot fallback, missing metrics, stale data, and cross-category normalization.

## 4. Import Job Workflow

- [x] 4.1 Refactor reusable CSV header aliasing, numeric parsing, normalization, and row import logic out of the CLI-specific importer path.
- [x] 4.2 Implement import job persistence with pending, running, completed, partially completed, and failed states plus summary counts.
- [x] 4.3 Implement bounded multipart CSV upload validation for file type, file size, row count, required identity, and row-level errors.
- [x] 4.4 Implement marketplace-plus-ASIN and provider-plus-source-ID deduplication while appending valid metric observations.
- [x] 4.5 Implement import idempotency and safe error-report values for spreadsheet formulas and untrusted markup.
- [x] 4.6 Add import workflow tests for valid files, invalid headers, mixed row outcomes, duplicates, retries, limits, and unsafe content.

## 5. Versioned Go API

- [x] 5.1 Add shared `/api/v1` JSON response, pagination, validation-error, not-found, and internal-error helpers.
- [x] 5.2 Implement `GET /api/v1/trends` with marketplace, canonical category, time window, search, sort, direction, page, and page-size handling.
- [x] 5.3 Implement `GET /api/v1/products/{id}` and `GET /api/v1/products/{id}/metrics` with provenance, completeness, classification, and score details.
- [x] 5.4 Implement category listing, source mapping listing, unresolved classification listing, and manual product-category update endpoints.
- [x] 5.5 Implement CSV import creation, import history, and import detail endpoints.
- [x] 5.6 Preserve `GET /healthz` and legacy `GET /products` behavior and add contract-focused `httptest` coverage for all new routes.
- [x] 5.7 Add development proxy support and configurable production static-file serving without exposing arbitrary filesystem paths.

## 6. Frontend Foundation

- [x] 6.1 Create the `web/` React, TypeScript, and Vite application with development, test, lint, and production build commands.
- [x] 6.2 Add React Router, TanStack Query, Recharts, Lucide, Vitest, and React Testing Library with a typed API client and normalized error handling.
- [x] 6.3 Build the responsive workbench shell with desktop sidebar, compact header, mobile navigation, route titles, and stable content dimensions.
- [x] 6.4 Define project-owned design tokens and CSS for typography, spacing, neutral surfaces, semantic status colors, tables, forms, focus states, and responsive behavior.
- [x] 6.5 Implement reusable loading, empty, stale, partial-data, and retryable error states with accessible labels and keyboard behavior.

## 7. Trending Product Workbench

- [x] 7.1 Build the Trending Products route with marketplace, category, time-window, search, sort, direction, and clear-filter controls synchronized to the URL.
- [x] 7.2 Build the desktop trend table and mobile product rows with stable image, identity, price, trend, score, source, freshness, and missing-data presentation.
- [x] 7.3 Implement pagination and query caching without losing active filter state or presenting stale results as freshly updated.
- [x] 7.4 Build the product detail route with normalized details, source provenance, category assignment, hotspot component explanation, confidence, and missing evidence.
- [x] 7.5 Add responsive price, rank, review, and demand history charts with insufficient-history states and readable tooltips.
- [x] 7.6 Add frontend tests for route state, filters, sorting, pagination, missing values, estimated labels, stale data, loading, empty, and error behavior.

## 8. Category and Import Workbenches

- [x] 8.1 Build the Categories route with canonical hierarchy, source mappings, unresolved products, confidence display, and manual assignment controls.
- [x] 8.2 Build the Imports route with bounded CSV selection, upload progress, validation feedback, import history, summary counts, and job details.
- [x] 8.3 Escape imported text throughout the UI and verify unsafe CSV values are never rendered as executable markup.
- [x] 8.4 Add frontend tests for category review, manual override behavior, CSV upload validation, partial imports, duplicate summaries, and job detail rendering.

## 9. Documentation and End-to-End Verification

- [x] 9.1 Document frontend prerequisites, Go API startup, Vite development startup, database setup, sample CSV import, production build, and static serving configuration.
- [x] 9.2 Add representative multi-snapshot and Amazon-identity fixtures for local development and browser verification.
- [x] 9.3 Run Go formatting, `go test ./...`, frontend lint, frontend unit tests, and frontend production build.
- [x] 9.4 Verify the complete dashboard, product detail, category review, and CSV import flows in a real browser at desktop and mobile viewports.
- [x] 9.5 Verify charts render nonblank, controls and text do not overlap, navigation is keyboard accessible, and loading or dynamic content does not shift fixed-format layouts.
- [x] 9.6 Run `openspec validate add-amazon-trending-webui` and resolve all validation findings.
