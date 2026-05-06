## 1. Project Foundation

- [x] 1.1 Choose and document the initial application stack and local development commands.
- [x] 1.2 Scaffold the app, API, worker, and shared domain module layout.
- [x] 1.3 Add environment configuration loading with secret redaction.
- [x] 1.4 Add a checked-in example environment file without real credentials.

## 2. Persistence

- [x] 2.1 Add database configuration and migration tooling.
- [x] 2.2 Create initial tables or models for products, source snapshots, jobs, assets, review states, and analytics references.
- [x] 2.3 Add repository or service boundaries for reading and writing workflow records.

## 3. Jobs and Assets

- [x] 3.1 Implement background job creation, status updates, retries, and failure recording.
- [x] 3.2 Implement an asset storage abstraction for local development and provider-backed storage.
- [x] 3.3 Add audit metadata helpers for external inputs and generated outputs.

## 4. Verification

- [x] 4.1 Add tests for configuration loading and secret redaction.
- [x] 4.2 Add tests for job lifecycle transitions.
- [x] 4.3 Add tests for asset metadata creation and workflow references.
- [x] 4.4 Document the foundation contracts used by later modules.
