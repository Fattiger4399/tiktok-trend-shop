## Why

The workflow needs a reliable way to discover and refresh TikTok product trend signals before it can score products or generate content. This change introduces normalized source ingestion for product, video, creator, hashtag, and metric snapshots.

## What Changes

- Add configurable trend source connectors for official APIs, approved third-party providers, and manual imports.
- Add scheduled collection by region, category, keyword, product URL, and source provider.
- Normalize raw source responses into reusable trend snapshots.
- Deduplicate products and source entities across providers.
- Track source health, rate limits, collection errors, and freshness.

## Capabilities

### New Capabilities
- `trend-source-ingestion`: Collection, normalization, deduplication, and freshness tracking for TikTok trend and product source data.

### Modified Capabilities
- None.

## Impact

- Depends on `app-foundation` for provider configuration, persistence, jobs, asset references, and audit metadata.
- Affects data ingestion services, scheduler jobs, source connector interfaces, and normalized snapshot storage.
