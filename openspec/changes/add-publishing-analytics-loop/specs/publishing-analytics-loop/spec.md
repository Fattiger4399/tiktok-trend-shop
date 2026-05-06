## ADDED Requirements

### Requirement: Publishing Channel Configuration
The system SHALL support publishing channel configuration for manual tracking and approved API-backed integrations.

#### Scenario: Configure manual channel
- **WHEN** a user creates a manual publishing channel
- **THEN** the system can associate exported packages and performance metrics with that channel

### Requirement: Publishing Readiness Gate
The system SHALL allow only approved export packages to be scheduled or marked for publishing.

#### Scenario: Schedule approved package
- **WHEN** a user schedules an approved export package
- **THEN** the system creates a publish attempt with channel, scheduled time, status, and package reference

#### Scenario: Block unapproved package
- **WHEN** a user attempts to publish an unapproved package
- **THEN** the system rejects the action and reports the missing approval state

### Requirement: Publish Status Tracking
The system SHALL track publish attempts, provider responses, statuses, errors, and retries.

#### Scenario: Provider status update
- **WHEN** an API-backed publishing provider returns a status update
- **THEN** the system records the provider status and maps it to the internal publish attempt state

### Requirement: Performance Metrics
The system SHALL ingest or manually record performance metrics for published videos.

#### Scenario: Import metrics snapshot
- **WHEN** metrics are imported for a published video
- **THEN** the system stores views, engagement, clicks, conversions, revenue, source, and collection time when available

### Requirement: Attribution and Feedback
The system SHALL attribute performance metrics to product, research card, script version, rendered video, export package, and publishing channel when those references exist.

#### Scenario: View performance attribution
- **WHEN** a user views performance for a product or video
- **THEN** the system shows linked metrics and the creative versions that produced the published output

#### Scenario: Generate feedback signal
- **WHEN** enough performance metrics exist for a product or script pattern
- **THEN** the system can produce advisory feedback for future product scoring and script generation
