## Why

Scripts need to become usable 9:16 video drafts with voiceover, captions, product visuals, and render metadata. This change adds the AI video generation pipeline that turns approved scripts into reviewable and exportable video assets.

## What Changes

- Add storyboard generation from approved script versions.
- Add media asset selection and generation for product visuals, overlays, voiceover, subtitles, and optional music.
- Add a render pipeline for vertical short-form video drafts.
- Track render jobs, stage failures, provider metadata, and output assets.
- Preserve provenance for AI-generated media and source product assets.

## Capabilities

### New Capabilities
- `ai-video-generation`: Storyboard, media assembly, voiceover, subtitles, rendering, and provenance tracking for AI-generated selling video drafts.

### Modified Capabilities
- None.

## Impact

- Depends on `app-foundation`, `product-intelligence`, and `creative-script-agent`.
- Affects media providers, render workers, asset storage, storyboard data models, and video preview or download surfaces.
