## ADDED Requirements

### Requirement: Review Queue
The system SHALL provide a review queue for product research cards, scripts, storyboards, and rendered videos.

#### Scenario: Item enters review
- **WHEN** a generated artifact is marked ready for review
- **THEN** the system adds it to the review queue with type, product, version, status, and owner metadata

### Requirement: Approval Gate
The system SHALL require approval before a generated video can be exported or marked publishing-ready.

#### Scenario: Export blocked before approval
- **WHEN** a user attempts to export an unapproved video
- **THEN** the system prevents export and shows the missing approval requirement

### Requirement: Compliance Checklist
The system SHALL record checklist results for claim support, policy-sensitive category, copyright, AI disclosure, and quality review.

#### Scenario: Complete checklist
- **WHEN** a reviewer approves a generated video
- **THEN** the system records completed checklist items and approval metadata

### Requirement: Feedback and Revision Routing
The system SHALL capture reviewer feedback and route requested changes to the relevant upstream artifact.

#### Scenario: Request script revision
- **WHEN** a reviewer rejects a video because of script wording
- **THEN** the system records the reason and links the change request to the script version

### Requirement: Export Package
The system SHALL create export packages that include the approved MP4, caption, hashtags, product references, source evidence, and generation metadata.

#### Scenario: Create export package
- **WHEN** an approved video is exported
- **THEN** the system creates an export package from immutable approved versions and stores it as an asset reference
