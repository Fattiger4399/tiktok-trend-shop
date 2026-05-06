## ADDED Requirements

### Requirement: Product Opportunity Scoring
The system SHALL calculate an opportunity score for product candidates using source trend signals, demand indicators, competition signals, economics, content potential, and risk factors.

#### Scenario: Score product candidate
- **WHEN** a product has sufficient source snapshots
- **THEN** the system calculates an opportunity score with component scores and a scoring model version

#### Scenario: Insufficient data
- **WHEN** a product has insufficient source data for one or more score components
- **THEN** the system marks those components as low confidence instead of fabricating values

### Requirement: Explainable Score Breakdown
The system SHALL expose the score components and evidence used to rank each product candidate.

#### Scenario: View score details
- **WHEN** a user opens a scored product
- **THEN** the system shows component scores, confidence levels, and source references used by the score

### Requirement: Product Research Card
The system SHALL generate a structured product research card containing target audience, core selling points, pain points, objections, proof points, creative angles, and recommended positioning.

#### Scenario: Generate research card
- **WHEN** a product candidate is selected for research
- **THEN** the system creates a product research card grounded in collected source snapshots and product details

### Requirement: Evidence-Backed Claims
The system SHALL attach evidence references to product claims used in product research cards.

#### Scenario: Unsupported claim detected
- **WHEN** a generated research claim lacks source evidence
- **THEN** the system flags the claim as unsupported and prevents it from being used as approved creative input

### Requirement: Risk and Compliance Warnings
The system SHALL identify policy-sensitive categories, risky claims, and content warnings that require human review.

#### Scenario: Risky claim found
- **WHEN** a product research card includes medical, financial, safety, or exaggerated performance claims
- **THEN** the system marks the product as requiring compliance review before script or video approval
