## ADDED Requirements

### Requirement: Configurable Source Connectors
The system SHALL allow trend source connectors to be configured by provider, region, category, keyword, and collection mode.

#### Scenario: Enable connector
- **WHEN** an operator enables a source connector with valid configuration
- **THEN** the system can schedule collection jobs for that connector

#### Scenario: Disabled connector
- **WHEN** a connector is disabled
- **THEN** the system does not schedule new collection jobs for that connector

### Requirement: Scheduled Trend Collection
The system SHALL collect trend candidates and related metrics through scheduled or manually triggered jobs.

#### Scenario: Scheduled collection runs
- **WHEN** a collection schedule reaches its run time
- **THEN** the system starts a background job for the configured source, region, category, and time window

#### Scenario: Manual collection runs
- **WHEN** a user triggers collection for a product URL, keyword, or category
- **THEN** the system collects matching source data and stores the resulting snapshots

### Requirement: Normalized Source Snapshots
The system SHALL normalize collected product, video, creator, hashtag, shop, and metric data into durable source snapshots.

#### Scenario: Normalize provider response
- **WHEN** a connector returns raw source data
- **THEN** the system stores normalized fields and preserves provider metadata needed for traceability

### Requirement: Entity Deduplication
The system SHALL deduplicate products and source entities across repeated collections and multiple providers.

#### Scenario: Existing product found
- **WHEN** a collected product matches an existing product by provider ID, canonical URL, or deduplication rule
- **THEN** the system attaches the new snapshot to the existing product record instead of creating a duplicate product

### Requirement: Source Health Tracking
The system SHALL track connector freshness, rate-limit status, errors, and last successful collection time.

#### Scenario: Provider error
- **WHEN** a connector fails due to authentication, rate limit, or provider error
- **THEN** the system records the failure and exposes the connector as degraded until a later successful run
