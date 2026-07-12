## ADDED Requirements

### Requirement: Canonical category hierarchy
The system SHALL maintain a stable platform-owned category hierarchy separately from source category names and identifiers.

#### Scenario: Client lists categories
- **WHEN** a client requests canonical categories
- **THEN** the system returns category IDs, names, parent relationships, active state, and display order

#### Scenario: Source category name changes
- **WHEN** a provider changes a category label while retaining or replacing its source identifier
- **THEN** the canonical category remains stable and the source mapping can be updated independently

### Requirement: Source category mappings
The system SHALL map provider and marketplace category identifiers to canonical categories using explicit mapping records.

#### Scenario: Exact mapping exists
- **WHEN** an imported product contains a provider category identifier with an active exact mapping
- **THEN** the system assigns the mapped canonical category and records the mapping method and confidence

#### Scenario: Mapping does not exist
- **WHEN** an imported source category has no exact mapping
- **THEN** the system preserves the source category and continues to the configured fallback classification process

### Requirement: Confidence-based fallback classification
The system SHALL support deterministic fallback classification and route uncertain results to manual review.

#### Scenario: Keyword rule produces high confidence
- **WHEN** no exact mapping exists and a deterministic rule exceeds the automatic-assignment threshold
- **THEN** the system assigns the canonical category and records the rule, confidence, and assignment time

#### Scenario: Classification confidence is low
- **WHEN** no exact mapping or fallback rule reaches the automatic-assignment threshold
- **THEN** the system marks the product as requiring category review and does not silently assign a confident category

### Requirement: Manual category review
The system SHALL provide a workbench queue for reviewing unclassified and low-confidence products.

#### Scenario: Reviewer assigns a category
- **WHEN** a reviewer selects a canonical category for a queued product
- **THEN** the system saves a manual assignment, removes the product from the unresolved queue, and records an audit event

#### Scenario: Automatic classification reruns after manual assignment
- **WHEN** a product already has a manual category assignment
- **THEN** an automatic mapping or rule does not overwrite the manual assignment

### Requirement: Category filtering uses canonical assignments
The system SHALL use canonical category IDs for workbench filtering while retaining source categories for display and audit.

#### Scenario: User filters by canonical category
- **WHEN** a trend query specifies a canonical category ID
- **THEN** the system returns products assigned to that category regardless of the provider-specific category label

