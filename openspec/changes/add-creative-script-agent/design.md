## Context

AI-generated scripts are only useful if they are structured, grounded, editable, and reusable by video generation. The script agent should produce creative variety while respecting product evidence and risk flags.

## Goals / Non-Goals

**Goals:**
- Generate multiple structured script variants from a product research card.
- Keep claims grounded in approved product research.
- Support versioning, revisions, and variant comparison.
- Produce outputs that the video generation module can consume directly.

**Non-Goals:**
- Render final videos.
- Automatically publish generated scripts.
- Approve risky claims without human review.

## Decisions

- Use a strict script schema instead of free-form text. The video module needs predictable fields such as hook, scene beats, voiceover, captions, CTA, hashtags, and disclaimers.
- Separate creative parameters from prompt templates. Duration, audience, language, tone, and hook style should be configurable per generation request.
- Validate scripts after generation. Validation checks source-backed claims, banned wording, missing CTA, duration fit, and risk flags.
- Keep revisions as new versions. Users should be able to compare generated and edited scripts and trace which version produced a video.

## Risks / Trade-offs

- Strong validation can reduce creative variety -> Allow regeneration with different creative parameters while preserving claim rules.
- Scripts may exceed target duration -> Estimate spoken word count and scene timing during validation.
- Hashtags can become stale -> Generate hashtags from current source and trend data, then mark them with generation time.
- LLM outputs may be malformed -> Use structured output parsing and retry with repair prompts when needed.
