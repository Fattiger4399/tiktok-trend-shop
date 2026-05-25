## ADDED Requirements

### Requirement: Enriched Detail Research Inputs
The system SHALL prefer enriched product detail snapshots when generating product research cards.

#### Scenario: Detail snapshot available
- **WHEN** a product has a latest detail snapshot with selling points, specs, review summary, price, platform, or shop facts
- **THEN** the generated research card includes evidence-backed selling points and proof points from those details

#### Scenario: Detail snapshot incomplete
- **WHEN** a product detail snapshot is missing required fields
- **THEN** the generated research card exposes missing-detail warnings for human review

### Requirement: Enrichment-Aware Risk Detection
The system SHALL scan enriched product detail text for unsupported or policy-sensitive claims.

#### Scenario: Risk term in enriched detail
- **WHEN** enriched product details contain medical, exaggerated, financial, children, food safety, or restricted efficacy claim terms
- **THEN** the system records risk flags and marks the product as requiring review before approval
