## ADDED Requirements

### Requirement: Browser CSV upload
The system SHALL allow a user to upload a bounded CSV file through the web workbench and create a traceable import job.

#### Scenario: User uploads a valid CSV file
- **WHEN** a user submits a CSV file within the configured size and row limits
- **THEN** the system creates an import job, validates the file, imports valid rows, and returns the job identifier

#### Scenario: User uploads an unsupported file
- **WHEN** a user submits a non-CSV file or a file exceeding configured limits
- **THEN** the system rejects the upload with a structured validation error and does not create product records

### Requirement: Header and row validation
The system SHALL validate required identity fields and report row-level normalization errors without hiding successful rows.

#### Scenario: CSV is missing required identity columns
- **WHEN** the header cannot provide a title or supported product identity
- **THEN** the import job fails validation before product rows are written

#### Scenario: Some rows are invalid
- **WHEN** a valid CSV contains both valid and invalid rows
- **THEN** the system imports valid rows and records row number, field, and reason for each rejected row

### Requirement: Import deduplication and history preservation
The system SHALL deduplicate normalized products while preserving each valid metric observation as historical evidence.

#### Scenario: Amazon product already exists
- **WHEN** an imported row has a marketplace and ASIN matching an existing product
- **THEN** the system updates normalized product details and appends a new source snapshot instead of creating a duplicate product

#### Scenario: Non-Amazon source product already exists
- **WHEN** an imported row matches an existing provider and source identifier
- **THEN** the system reuses the existing product and appends the new observation

#### Scenario: Same file row is retried
- **WHEN** an identical import file or row is submitted again
- **THEN** the job reports the duplicate outcome according to the import idempotency key and does not create duplicate product identities

### Requirement: Import job history
The system SHALL provide import job summaries and detailed outcomes through the API and web workbench.

#### Scenario: User opens import history
- **WHEN** a user visits the Imports route
- **THEN** the system displays each job's source filename, status, start time, completion time, total rows, imported rows, updated rows, duplicates, and rejected rows

#### Scenario: User opens a completed import job
- **WHEN** a user selects an import job
- **THEN** the system displays its validation errors, row outcomes, and created or updated product identifiers

### Requirement: Imported content safety
The system SHALL treat uploaded values as untrusted data.

#### Scenario: CSV contains markup or spreadsheet formula text
- **WHEN** an uploaded field begins with executable spreadsheet syntax or contains HTML markup
- **THEN** the system stores the source value for audit but renders it as escaped text and neutralizes it in any downloaded error report

