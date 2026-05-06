## Context

Trend data can come from multiple sources with different availability, permissions, rate limits, schemas, and freshness. The product should avoid coupling the rest of the workflow to any single source provider.

## Goals / Non-Goals

**Goals:**
- Provide a connector interface for trend and product source providers.
- Normalize products, videos, creators, hashtags, shops, and metrics into a shared snapshot format.
- Support scheduled and manual collection jobs.
- Preserve source evidence for downstream scoring and research.

**Non-Goals:**
- Circumvent TikTok platform permissions or scraping protections.
- Decide whether a product is worth creating content for.
- Generate product research, scripts, or videos.

## Decisions

- Use provider connectors with a common collection contract. Each connector returns source records plus provider metadata, while normalization maps them into shared entities.
- Store immutable source snapshots. Trend scoring depends on change over time, so previous metrics must remain available.
- Deduplicate by provider IDs, canonical product URLs, shop identifiers, and normalized product names where provider IDs are missing.
- Treat manual import as a first-class source. This allows early validation even before official or paid data providers are integrated.
- Record collection windows and freshness. Users need to know whether a trend score is based on recent data or stale snapshots.

## Risks / Trade-offs

- Official APIs may not expose every desired metric -> Support third-party and manual import connectors behind the same contract.
- Provider schemas may be inconsistent -> Preserve raw payload references for debugging and normalize only required fields first.
- Rate limits can block daily refreshes -> Schedule jobs per provider and persist partial collection progress.
- Deduplication may merge unrelated products -> Keep source links and allow later manual correction.
