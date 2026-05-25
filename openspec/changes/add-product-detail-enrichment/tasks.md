## 1. Data Model

- [x] 1.1 Add a SQLite migration for product detail snapshots and indexes.
- [x] 1.2 Define product detail dataclasses and repository methods for saving, fetching latest, and listing snapshots.

## 2. Enrichment Service

- [x] 2.1 Implement normalized manual detail payload parsing, required-field completeness checks, and risk text collection.
- [x] 2.2 Add CSV row mapping for common marketplace detail columns and attach details during product CSV import.
- [x] 2.3 Preserve product-detail source metadata and audit events for traceability.

## 3. Intelligence Integration

- [x] 3.1 Add enriched detail evidence extraction for selling points, specs, reviews, price, shop, brand, and product URL.
- [x] 3.2 Update research card generation to prefer enriched details and include missing-detail warnings.
- [x] 3.3 Extend risk detection to scan enriched detail text for sensitive claims.

## 4. CLI and Output

- [x] 4.1 Add a CLI command to show latest product results with score, detail completeness, URL, caption, render URI, and export package.
- [x] 4.2 Include detail completeness and product URL in workflow result payloads.
- [x] 4.3 Document the enriched CSV columns and next-step usage commands.

## 5. Verification

- [x] 5.1 Add tests for detail snapshot storage and latest snapshot lookup.
- [x] 5.2 Add tests for CSV detail enrichment and completeness warnings.
- [x] 5.3 Add tests for research card use of enriched evidence and risk scanning.
- [x] 5.4 Add tests for the show-results CLI command.
- [x] 5.5 Run unit tests and `openspec validate add-product-detail-enrichment`.
