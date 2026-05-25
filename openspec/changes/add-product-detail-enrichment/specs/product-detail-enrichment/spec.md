## ADDED Requirements

### Requirement: Product Detail Snapshot Storage
The system SHALL store product detail enrichment results as versioned snapshots linked to a product.

#### Scenario: Store supplied product details
- **WHEN** an operator supplies product details for a product
- **THEN** the system stores a product detail snapshot with platform, product URL, shop, brand, price, image URL, specs, selling points, review summary, source metadata, and capture time

#### Scenario: Preserve repeated enrichment
- **WHEN** a product is enriched more than once
- **THEN** the system preserves each snapshot and exposes the latest snapshot as the current detail source

### Requirement: Manual and CSV Detail Enrichment
The system SHALL allow product details to be enriched from manually supplied records and CSV columns.

#### Scenario: Import CSV with product detail columns
- **WHEN** a CSV row contains detail fields such as product URL, shop, brand, price, image URL, selling points, specs, or review summary
- **THEN** the system imports those fields into a product detail snapshot linked to the imported product

#### Scenario: Enrich existing product by title
- **WHEN** a manual detail record references an existing product by title and region
- **THEN** the system attaches the detail snapshot to that product instead of creating a duplicate product

### Requirement: Detail Completeness Checks
The system SHALL calculate completeness status for each product detail snapshot.

#### Scenario: Required fields missing
- **WHEN** product URL, image URL, price, selling points, or review summary are missing
- **THEN** the system records missing fields and marks the detail snapshot as incomplete

#### Scenario: Required fields present
- **WHEN** all required detail fields are present
- **THEN** the system marks the detail snapshot as complete

### Requirement: Detail Evidence Exposure
The system SHALL expose enriched product details as evidence for downstream product intelligence and creative generation.

#### Scenario: Research uses enriched facts
- **WHEN** a product has an enriched detail snapshot
- **THEN** research generation can use its selling points, specs, reviews, platform, shop, brand, price, and source URL as evidence

### Requirement: Enrichment Result Visibility
The system SHALL provide CLI visibility into latest product details and generated workflow outputs.

#### Scenario: Show product results
- **WHEN** an operator asks to show local results
- **THEN** the system lists products with latest score, detail completeness, product URL, script caption, hashtags, render URI, and export package ID
