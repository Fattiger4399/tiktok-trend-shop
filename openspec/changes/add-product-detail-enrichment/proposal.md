## Why

Trend ranking rows only tell us that a product may be hot; they do not provide enough verified product facts to produce grounded scripts or videos. This change turns a bare product name into a reusable product dossier with links, pricing, images, selling points, reviews, source evidence, and missing-data warnings.

## What Changes

- Add a product detail enrichment capability for manually supplied, CSV-supplied, or later API-supplied product facts.
- Store versioned product detail snapshots with source URL, platform, shop, brand, price, specs, images, selling points, review summary, and evidence metadata.
- Add completeness and risk checks so sparse or policy-sensitive product facts are visible before creative generation.
- Extend CSV import and local workflow output to use enriched product facts when available.
- Add CLI visibility for enriched product records and latest generated outputs.

## Capabilities

### New Capabilities
- `product-detail-enrichment`: Product fact collection, normalized detail storage, completeness checks, and evidence-backed product dossiers.

### Modified Capabilities
- `product-intelligence`: Research cards must prefer enriched product details and expose missing-detail warnings.

## Impact

- Affects SQLite migrations, repositories, CSV import, product intelligence evidence extraction, workflow output, CLI commands, and tests.
- Does not require live Taobao, Douyin, TikTok, or search API credentials in this change.
- Keeps enrichment provider-agnostic so Taobao affiliate APIs, search APIs, and paid data platforms can be added later.
