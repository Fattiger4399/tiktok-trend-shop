## Context

The current pipeline can rank imported products and generate local scripts, but product facts are mostly limited to the title, category, URL, metadata, and trend metrics. Domestic Douyin workflows need a middle layer that can accept product facts from CSV exports, manual research, affiliate APIs, or search APIs without depending on any single provider.

## Goals / Non-Goals

**Goals:**
- Store normalized product detail snapshots separately from trend snapshots.
- Preserve source evidence and capture time for each enrichment pass.
- Support CSV/manual enrichment immediately, with a provider-agnostic interface for later Taobao, search, or paid-platform connectors.
- Surface missing fields and policy-sensitive product facts before creative generation.
- Feed enriched facts into research cards and CLI summaries.

**Non-Goals:**
- Build a Taobao, Douyin, TikTok, or search API connector in this change.
- Scrape third-party pages automatically.
- Guarantee that supplied product claims are legally approved.
- Replace human review for cosmetics, food, children, health, supplement, or medical claims.

## Decisions

- Store product details in a new `product_detail_snapshots` table instead of expanding `products`. Product details are versioned evidence, while `products` remains the canonical candidate record.
- Represent details as normalized columns plus JSON arrays. Fields needed for sorting and display, such as platform, shop, brand, price, currency, product URL, and image URL, get columns; variable fields such as specs, selling points, review summary, and warnings stay in JSON.
- Add `ProductDetailEnrichmentService` with a manual payload model. The first connector is CSV/manual, but the service shape can accept API-backed results later.
- Compute completeness deterministically. Missing URL, price, image, selling points, or review summary should be visible to the user and downstream modules.
- Treat product detail text as evidence for research cards, but keep risky claims flagged. Enriched data improves grounding; it does not automatically approve claims.

## Risks / Trade-offs

- Sparse manual data can still produce weak scripts -> mark completeness status and list missing fields.
- CSV column names may vary across platforms -> support common aliases and preserve raw rows for traceability.
- Product title matching can select the wrong marketplace item -> prefer explicit product URLs and keep source metadata.
- More detail fields increase storage complexity -> keep a single snapshot table and avoid provider-specific schemas until real APIs are added.

## Migration Plan

- Add migration `008_product_detail_enrichment` with a snapshot table and indexes.
- Existing databases migrate in place without changing current product rows.
- Existing workflows continue to run when no detail snapshots exist.
- Rollback is manual for local SQLite: delete the migration row and table only if no enriched detail data is needed.
