## ADDED Requirements

### Requirement: Go Runtime Foundation
The system SHALL provide a Go backend foundation with command entry points, runtime configuration, and package boundaries suitable for future API and worker development.

#### Scenario: Run Go CLI
- **WHEN** an operator runs the Go command
- **THEN** the system exposes documented subcommands for migration, CSV import, product listing, and HTTP serving

### Requirement: Go Database Migration
The system SHALL initialize local SQLite tables needed for product discovery and enrichment.

#### Scenario: Run migration
- **WHEN** an operator runs the Go migration command
- **THEN** the system creates product, source snapshot, product detail snapshot, and migration tables if they do not already exist

### Requirement: Go CSV Product Import
The system SHALL import product ranking CSV rows through the Go backend.

#### Scenario: Import ranking CSV
- **WHEN** a CSV contains title, region, category, metrics, and source identifiers
- **THEN** the system upserts products, stores source snapshots, and returns imported product IDs

#### Scenario: Import detail columns
- **WHEN** a CSV row contains product URL, platform, image URL, price, selling points, specs, or review summary
- **THEN** the system stores a product detail snapshot linked to the imported product

### Requirement: Go Product Query
The system SHALL list product candidates with latest metrics and detail completeness.

#### Scenario: List products
- **WHEN** an operator requests product results
- **THEN** the system returns products with IDs, title, region, category, workflow state, metrics, product URL, platform, price, and detail completeness

### Requirement: Minimal Go HTTP API
The system SHALL expose a minimal HTTP API for health checks and product discovery.

#### Scenario: Health check
- **WHEN** a client requests the health endpoint
- **THEN** the API returns a successful JSON health response

#### Scenario: Products endpoint
- **WHEN** a client requests the products endpoint
- **THEN** the API returns a JSON list of product candidates using the same query behavior as the Go CLI
