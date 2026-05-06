## ADDED Requirements

### Requirement: Storyboard Generation
The system SHALL generate a storyboard from an approved script version before media rendering begins.

#### Scenario: Create storyboard
- **WHEN** a user starts video generation from an approved script version
- **THEN** the system creates storyboard scenes with timing, voiceover text, visual direction, overlay text, and required assets

### Requirement: Media Asset Planning
The system SHALL select or generate media assets needed for the storyboard while preserving source and AI provenance.

#### Scenario: Use product image
- **WHEN** a storyboard scene requires a product visual and a source product image is available
- **THEN** the system can reference the source product image and record its origin in the scene asset plan

#### Scenario: Generate AI visual
- **WHEN** a scene requires generated media
- **THEN** the system stores the prompt, provider, model, and output asset reference

### Requirement: Voiceover and Subtitles
The system SHALL generate voiceover audio and synchronized subtitles for approved video drafts.

#### Scenario: Generate voiceover
- **WHEN** a storyboard contains voiceover text
- **THEN** the system creates voiceover audio and subtitle timing assets linked to the render job

### Requirement: Vertical Video Rendering
The system SHALL render approved storyboard and media assets into a 9:16 MP4 video draft.

#### Scenario: Render video draft
- **WHEN** all required storyboard assets are ready
- **THEN** the system renders a vertical MP4 draft and stores the output as an asset

### Requirement: Render Validation
The system SHALL validate rendered video drafts before marking them complete.

#### Scenario: Validate render
- **WHEN** a render job finishes
- **THEN** the system verifies aspect ratio, duration, audio presence, subtitle presence, and output asset metadata

### Requirement: Stage-Level Failure Tracking
The system SHALL track video generation failures by pipeline stage.

#### Scenario: TTS provider fails
- **WHEN** voiceover generation fails
- **THEN** the system marks the voiceover stage failed and allows retrying that stage without recreating completed storyboard assets
