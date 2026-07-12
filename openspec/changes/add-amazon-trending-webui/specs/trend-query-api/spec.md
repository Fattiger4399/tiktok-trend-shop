## ADDED Requirements

### Requirement: Versioned trend listing API
The system SHALL expose a versioned API that returns paginated trend results with validated filtering and sorting.

#### Scenario: Client requests default trend results
- **WHEN** a client requests `GET /api/v1/trends` without optional filters
- **THEN** the system returns a bounded first page, default time window, default sort order, and pagination metadata

#### Scenario: Client requests filtered trend results
- **WHEN** a client provides supported marketplace, category, time-window, search, sort, direction, page, and page-size parameters
- **THEN** the system returns matching results and normalized effective query metadata

#### Scenario: Client provides invalid query values
- **WHEN** a query parameter is unsupported or outside its allowed range
- **THEN** the system returns a structured validation error without executing an unbounded query

### Requirement: Amazon-compatible product identity
The system SHALL support optional Amazon marketplace and ASIN identity while preserving non-Amazon and legacy product records.

#### Scenario: Product is imported with marketplace and ASIN
- **WHEN** a normalized row contains both a marketplace and ASIN
- **THEN** the system associates the row with the unique product identified by that marketplace and ASIN

#### Scenario: Existing product has no ASIN
- **WHEN** a legacy product is queried without Amazon identity
- **THEN** the system returns the product using its internal and source identities without failing the request

### Requirement: Product detail and history APIs
The system SHALL expose product detail and timestamped metric history independently from the list projection.

#### Scenario: Client requests product detail
- **WHEN** a client requests `GET /api/v1/products/{id}` for an existing product
- **THEN** the system returns normalized identity, latest detail, canonical classification, latest score, provenance, and data completeness

#### Scenario: Client requests product metric history
- **WHEN** a client requests `GET /api/v1/products/{id}/metrics` with a supported time window
- **THEN** the system returns chronological observations with metric values, provider, and captured time

#### Scenario: Product does not exist
- **WHEN** a client requests detail or metrics for an unknown product ID
- **THEN** the system returns a structured not-found response

### Requirement: Category-relative hotspot scores
The system SHALL produce versioned hotspot score snapshots using available metrics normalized within marketplace, canonical category, and time window.

#### Scenario: Product has sufficient historical evidence
- **WHEN** demand, acceleration, review growth, price signal, freshness, and completeness inputs are available
- **THEN** the system stores and returns a total score, component values, model version, comparison window, confidence, and evidence references

#### Scenario: Product has incomplete historical evidence
- **WHEN** one or more score inputs are unavailable
- **THEN** the system excludes unsupported evidence, reports missing components, and lowers confidence instead of manufacturing metric values

#### Scenario: Score comparison crosses categories
- **WHEN** hotspot scores are computed for products in different canonical categories
- **THEN** each product is normalized against its own marketplace and canonical category comparison group

### Requirement: Source provenance and freshness
The system SHALL include sufficient provenance for clients to distinguish observed, estimated, and derived values.

#### Scenario: API returns a provider metric
- **WHEN** a product result contains a provider-observed or provider-estimated metric
- **THEN** the response identifies the provider, captured time, metric kind, and whether the value is estimated

#### Scenario: Latest data is stale
- **WHEN** the latest observation exceeds the configured freshness threshold
- **THEN** the API marks the product data as stale and returns the last captured time

### Requirement: Existing API compatibility
The system SHALL preserve the existing health and product-listing behavior while clients migrate to `/api/v1`.

#### Scenario: Existing client requests legacy products endpoint
- **WHEN** a client requests `GET /products` using the current supported parameters
- **THEN** the system returns the existing response shape and does not require Amazon-specific fields

