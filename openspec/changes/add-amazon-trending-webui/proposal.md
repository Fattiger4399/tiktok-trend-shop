## Why

The Go backend can import and list product records, but the project has no customer-facing interface for discovering, comparing, or validating trending products. A focused web workbench is needed now so the product discovery workflow can be tested with existing CSV data before committing to a live Amazon data provider.

## What Changes

- Add a React and TypeScript web application for an Amazon-oriented trending product workbench.
- Add a dense hotspot dashboard with marketplace, category, time-window, search, and sorting controls.
- Add product detail views with source provenance, metric history, trend indicators, and category information.
- Extend the Go API with paginated trend queries, product detail, metric history, category, and import-management endpoints.
- Add Amazon-compatible product identity and marketplace fields while continuing to support existing manual and CSV sources.
- Add a canonical category tree, source-category mappings, confidence-based classification, and a manual review queue.
- Add CSV upload, import history, validation results, and duplicate/error reporting in the web workbench.
- Define a category-relative hotspot score and expose its components, confidence, source, and update time.
- Keep live Amazon, Keepa, or other licensed provider integrations outside this first change; the ingestion boundary will allow those providers to be added later.

## Capabilities

### New Capabilities

- `amazon-trend-workbench`: Browser-based hotspot dashboard and product detail experience for discovering and comparing Amazon-oriented product trends.
- `trend-query-api`: Paginated and filterable product trend APIs, metric history, hotspot scoring, and source provenance required by the workbench.
- `catalog-classification`: Canonical product categories, source-category mappings, confidence handling, and manual classification review.
- `data-import-workbench`: Browser-based CSV import, validation feedback, import history, and duplicate/error reporting.

### Modified Capabilities

- None.

## Impact

- Adds a new frontend application and frontend build dependencies.
- Extends the Go HTTP API, product repository, SQLite schema, migrations, and tests.
- Adds Amazon-compatible fields such as marketplace and ASIN without requiring a live Amazon integration.
- Adds metric history, hotspot scores, category mappings, and import job records.
- Preserves the Python workflow and existing Go CLI behavior.
- Requires development proxy or CORS handling between the frontend development server and Go API.
