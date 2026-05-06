## ADDED Requirements

### Requirement: Runtime Configuration
The system SHALL manage runtime configuration for regions, categories, source providers, AI providers, storage, and feature flags without hardcoding secrets in source files.

#### Scenario: Load provider configuration
- **WHEN** the application starts with configured provider environment variables
- **THEN** the system loads provider settings and redacts secret values from logs and responses

#### Scenario: Missing required configuration
- **WHEN** a required provider setting is missing for an enabled module
- **THEN** the system reports a clear configuration error and prevents that module from running

### Requirement: Durable Domain Records
The system SHALL persist product records, source snapshots, generated assets, background jobs, review states, and analytics references with stable identifiers and timestamps.

#### Scenario: Create product workflow record
- **WHEN** a product candidate enters the workflow
- **THEN** the system stores a durable product workflow record that later modules can reference

#### Scenario: Update workflow state
- **WHEN** a later module updates the product, generated content, or review status
- **THEN** the system preserves the previous timestamps and records the latest state change

### Requirement: Background Job Tracking
The system SHALL execute long-running collection, generation, rendering, and integration tasks through trackable background jobs.

#### Scenario: Start background job
- **WHEN** a module schedules a long-running task
- **THEN** the system creates a job record with status, input reference, retry count, and timestamps

#### Scenario: Job failure
- **WHEN** a background task fails
- **THEN** the system records the failure reason, marks the job failed, and allows a retry when the task is retryable

### Requirement: Asset References
The system SHALL store media and export artifacts through an asset abstraction and reference those assets from workflow records.

#### Scenario: Store generated asset
- **WHEN** a module produces an image, audio file, subtitle file, or rendered video
- **THEN** the system stores the asset metadata and returns a reference that can be reused by downstream modules

### Requirement: Audit Metadata
The system SHALL attach audit metadata to external data and generated outputs, including source provider, source URL or ID, prompt version, model or renderer, and creation time when applicable.

#### Scenario: Trace generated output
- **WHEN** a user inspects a generated script, voiceover, or video
- **THEN** the system can show the source product record and generation metadata used to create it
