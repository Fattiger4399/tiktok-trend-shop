## ADDED Requirements

### Requirement: Structured Script Generation
The system SHALL generate structured TikTok selling script variants from an approved product research card.

#### Scenario: Generate script variants
- **WHEN** a user requests scripts for an approved product research card
- **THEN** the system creates one or more script variants with hook, scene beats, voiceover, captions, hashtags, CTA, and compliance notes

### Requirement: Creative Parameter Control
The system SHALL allow script generation to be controlled by duration, target audience, language, tone, hook style, and output count.

#### Scenario: Generate by duration and tone
- **WHEN** a user requests a 20-second energetic script variant
- **THEN** the system generates a script whose estimated voiceover and scene beats fit the requested duration and tone

### Requirement: Claim Grounding
The system SHALL validate generated scripts against approved product research claims and evidence references.

#### Scenario: Unsupported script claim
- **WHEN** a generated script includes a product claim not present in approved research evidence
- **THEN** the system flags the script variant and prevents it from being marked ready for video generation

### Requirement: Script Versioning
The system SHALL preserve generated script variants and user revisions as versioned records.

#### Scenario: Revise script
- **WHEN** a user edits a generated script
- **THEN** the system creates a new version linked to the original variant and preserves the prior version

### Requirement: Video-Ready Output
The system SHALL expose approved script versions in a structure that can be consumed by the video generation module.

#### Scenario: Select script for video
- **WHEN** a script version passes validation and is selected for video generation
- **THEN** the system provides all required storyboard input fields to the video generation workflow
