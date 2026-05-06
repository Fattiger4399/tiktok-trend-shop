## Context

The video generation module should transform an approved script into a draft video that can be reviewed, exported, and later published. Video work is asynchronous, provider-heavy, and failure-prone, so it needs stage-level tracking and reusable asset references.

## Goals / Non-Goals

**Goals:**
- Generate a storyboard from a structured script.
- Create or select product visuals, overlays, voiceover, subtitles, and render settings.
- Render a 9:16 MP4 draft with preview and asset metadata.
- Track provider, model, prompt, and source asset provenance.

**Non-Goals:**
- Automatically approve final content.
- Automatically publish to TikTok.
- Guarantee that AI-generated visuals exactly match the physical product.

## Decisions

- Model the pipeline as stages: storyboard, asset planning, media generation, voiceover, subtitles, composition, render, and validation. This allows retries without restarting the full workflow.
- Use provider abstractions for image/video generation, text-to-speech, captioning, and rendering. Initial implementation can use local rendering for composition while keeping AI providers replaceable.
- Store every intermediate media item as an asset. Reviewers need provenance, and failed renders need reusable intermediates.
- Validate output dimensions, duration, audio presence, subtitle timing, and required metadata before marking a render complete.

## Risks / Trade-offs

- AI visuals may misrepresent the product -> Prefer source product assets when available and require review of generated visuals.
- Rendering can be slow or expensive -> Use background jobs with stage retries and cache reusable intermediates.
- Provider output formats may vary -> Normalize media metadata before composition.
- Captions can drift from voiceover -> Generate captions from final voiceover timing when possible.
