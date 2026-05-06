## Why

The product needs a stable application foundation before trend discovery, AI generation, review, and publishing can be implemented safely. This change establishes shared persistence, configuration, job tracking, asset storage, and auditability so later modules can build on one contract.

## What Changes

- Add the first application foundation for the TikTok trend-to-video workflow.
- Add runtime configuration for regions, categories, source providers, AI providers, storage, and feature flags.
- Add persistence contracts for products, source snapshots, generation jobs, assets, review state, and analytics references.
- Add background job tracking for long-running collection and generation work.
- Add secret-handling rules so API keys and provider credentials are never committed to the repository.
- Add audit metadata for source data and generated outputs.

## Capabilities

### New Capabilities
- `app-foundation`: Shared application, configuration, persistence, job, asset, and audit foundation for all workflow modules.

### Modified Capabilities
- None.

## Impact

- Affects the initial app structure, database schema, environment configuration, storage integration, background worker architecture, and module boundaries.
- Later changes depend on this foundation for durable records, job execution, asset references, and provider configuration.
