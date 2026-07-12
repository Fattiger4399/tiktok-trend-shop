## Context

The repository has a Go modular monolith that migrates SQLite data, imports CSV rows, projects the latest product metrics, and exposes `GET /healthz` and `GET /products`. The existing product records are primarily manual or CSV sourced and do not consistently include Amazon marketplace identity, historical trend presentation, canonical categories, or import job state. There is no frontend application.

This change creates the first customer-facing product selection workbench. It must be useful with the existing local dataset, remain compatible with the current CLI and Python workflow, and establish stable boundaries for a future licensed Amazon data provider. The initial deployment target is local development and single-user evaluation; production authentication, tenancy, and provider contracts are separate work.

## Goals / Non-Goals

**Goals:**

- Deliver a responsive, operational Web UI for scanning and comparing trending products.
- Expose stable versioned APIs for filtering, pagination, product details, metric history, categories, and imports.
- Preserve historical source snapshots and clearly label metric provenance, freshness, and confidence.
- Represent Amazon products by marketplace and ASIN without requiring all existing products to have an ASIN.
- Provide deterministic category-relative hotspot scoring that works with incomplete sample data.
- Provide canonical category mapping and a manual review path for uncertain classifications.
- Reuse the existing Go CSV parsing and repository boundaries where practical.

**Non-Goals:**

- Integrating Amazon SP-API, Product Advertising API, Keepa, or another live provider.
- Claiming that estimated sales metrics are verified Amazon sales.
- Adding authentication, multi-tenancy, billing, or user-specific saved lists.
- Migrating Python scoring, script generation, video generation, or publishing to Go.
- Building an SEO-oriented storefront or marketing landing page.
- Implementing automatic AI classification in the first release.

## Decisions

### Frontend stack and application boundary

Create a `web/` application using React, TypeScript, and Vite. Use React Router for route state, TanStack Query for server-state caching and request lifecycle handling, Recharts for metric history, and Lucide for interface icons. Use project-owned CSS rather than introducing a component framework so the interface can remain dense and workbench-oriented.

Vite is preferred over Next.js because the workbench does not need server-side rendering or SEO. A server-rendered framework would add a second backend runtime without solving a current requirement. The production build SHALL be deployable as static files, and the Go server MAY serve a configured build directory so local and single-host deployments can remain same-origin. During development, Vite SHALL proxy `/api` to the Go service.

### Workbench information architecture

Use a persistent desktop sidebar with Trending Products, Categories, and Imports destinations. The trending route contains a compact header, filter toolbar, summary indicators, and a dense sortable table. Mobile layouts replace the table with stable product rows while retaining the same information priority. Product details use a dedicated route rather than an oversized modal so URLs are shareable and charts have sufficient space.

The UI SHALL expose loading, empty, partial-data, stale-data, and error states. Missing price, ASIN, image, score components, or history SHALL be shown as unavailable data rather than converted to zero.

### Versioned API contract

Add routes under `/api/v1` and retain the existing `/healthz` and `/products` endpoints for compatibility. List responses use a stable envelope with `items` and pagination metadata. Filters are passed as query parameters and validated by the server. The first routes are:

- `GET /api/v1/trends`
- `GET /api/v1/products/{id}`
- `GET /api/v1/products/{id}/metrics`
- `GET /api/v1/categories`
- `GET /api/v1/category-mappings`
- `PATCH /api/v1/products/{id}/category`
- `POST /api/v1/imports/csv`
- `GET /api/v1/imports`
- `GET /api/v1/imports/{id}`

The frontend uses a small typed API client and does not access SQLite or decode provider-specific raw payloads.

### Product identity and source history

Keep the internal product ID as the primary key. Add optional `marketplace` and `asin` fields with a uniqueness constraint when both are present. Existing products without Amazon identity remain valid and display their current region/provider identity.

Continue treating source snapshots as append-only metric observations. Imports update normalized product fields and append snapshots instead of overwriting historical metrics. Product detail responses project the latest normalized values while metric history returns timestamped observations.

### Hotspot score

Store score snapshots rather than only calculating a mutable score in the browser. Scores are normalized within marketplace, canonical category, and requested time window so unrelated category scales are not compared directly. The initial deterministic model combines available demand, acceleration, review growth, price signal, freshness, and data completeness components. Component weights and a model version are stored with each score.

When historical data is insufficient, the system SHALL calculate only supported components, reduce confidence, and identify missing evidence. A high raw sales estimate from a single observation must not be presented as a high-confidence trend.

### Classification model

Create a stable canonical category hierarchy separate from provider categories and product tags. Store source category identifiers and mappings to canonical categories. Classification order is exact source mapping, deterministic keyword rule, then manual review. Each assignment records method, confidence, and update time.

Automatic AI classification is deferred. The schema retains a method field so an AI classifier can be added without changing the workbench contract. Manual assignments override automatic results and are audit recorded.

### CSV import workflow

The browser uploads CSV data as multipart form data to the Go API. The API enforces a configurable size limit, stores an import job record, validates headers and rows, reuses shared normalization helpers, and records row-level outcomes. The response returns the job ID rather than holding the request open for future long-running provider imports; the first local implementation MAY complete the job synchronously.

Deduplicate Amazon rows by `marketplace + ASIN`. Rows without that identity use the existing provider/source identifier strategy. Repeated observations update normalized product details and append a metric snapshot. Raw row values remain available for audit without being rendered as trusted HTML.

### Testing and verification

Go repository and HTTP behavior SHALL use unit and `httptest` coverage. Frontend data transformation and state behavior SHALL use Vitest and React Testing Library. The implemented workbench SHALL receive browser-level verification for desktop and mobile layouts, filter state, navigation, loading/empty/error behavior, chart rendering, and CSV upload.

## Risks / Trade-offs

- [The sample dataset has only one observation per product] -> Show low confidence and partial trend components; seed tests with multi-snapshot fixtures.
- [Amazon sales estimates can be misunderstood as real sales] -> Display source, timestamp, confidence, and explicit estimated labels at every relevant surface.
- [SQLite has limited write concurrency] -> Keep import writes transactional and scoped; retain repository boundaries for a later PostgreSQL migration.
- [Unauthenticated import and classification endpoints are unsafe on a public deployment] -> Document the first release as local/single-user and require authentication before public exposure.
- [Category-relative normalization can change scores when the comparison set changes] -> Store score time, model version, window, and components so changes remain explainable.
- [Frontend and API types can drift] -> Keep explicit TypeScript response types and contract-focused HTTP tests; consider generated clients only when route volume justifies them.
- [Serving static files from Go can complicate development] -> Use the Vite proxy in development and make static build serving configuration-based.

## Migration Plan

1. Add additive SQLite migrations for Amazon identity, canonical categories, mappings, assignments, score snapshots, and import jobs.
2. Backfill existing records with their current region and category where possible; leave ASIN and marketplace nullable.
3. Add `/api/v1` read endpoints while preserving existing endpoints and CLI behavior.
4. Add classification and import mutation endpoints.
5. Add the frontend application and connect it to fixture-backed API responses, then the live Go API.
6. Verify existing Go and Python tests, new API tests, frontend tests, and browser workflows.

Rollback consists of stopping use of the new frontend and `/api/v1` routes. Database changes are additive, so the prior CLI and endpoints continue to operate. New tables and columns do not need to be destructively removed.

## Open Questions

- Which canonical category seed list should become the long-term product taxonomy after the initial twelve top-level categories?
- Should the Go server serve the production frontend build by default, or should production deployment keep the static frontend on a separate host?
- What licensed Amazon-oriented provider will be selected for the subsequent live-ingestion change?

