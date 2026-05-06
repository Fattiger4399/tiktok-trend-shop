## Context

The repository currently contains OpenSpec scaffolding but no application implementation. The target product is an automated workflow for discovering TikTok hot products, generating product research, producing AI-assisted selling videos, reviewing outputs, and optionally publishing and measuring performance.

## Goals / Non-Goals

**Goals:**
- Define the shared application foundation required by all later modules.
- Keep secrets, source metadata, generation jobs, and generated assets traceable.
- Support asynchronous jobs for data collection and media generation.
- Leave room for multiple external providers without hardcoding a single vendor into domain logic.

**Non-Goals:**
- Implement a specific trend source connector.
- Generate scripts or videos.
- Publish to TikTok or collect performance metrics.
- Build a multi-tenant billing system.

## Decisions

- Use a modular application boundary: separate domain modules for ingestion, product intelligence, creative scripting, video generation, review/export, and publishing analytics. This keeps each OpenSpec change independently implementable.
- Use environment-based secrets and provider configuration. Credentials MUST live outside committed files because the workflow depends on sensitive API keys and platform credentials.
- Use durable job records for long-running work. Trend collection, LLM generation, media rendering, and publishing checks are asynchronous and need retries, statuses, and error traces.
- Use asset references rather than embedding media bytes in domain rows. Product images, generated voiceovers, subtitles, renders, and export packages should be stored through a storage abstraction and referenced by ID or URL.
- Use audit metadata on every external input and generated output. Later review and analytics workflows need to know which source, prompt version, provider, and model produced each artifact.

## Risks / Trade-offs

- Early foundation work can become overbuilt -> Keep contracts minimal and add fields only when a later module needs them.
- Provider APIs may change -> Isolate provider-specific code behind connectors and store provider response metadata for debugging.
- Long-running jobs can fail halfway -> Track stage-level status, retry count, and last error.
- Secrets may leak through logs or fixtures -> Redact sensitive values at config load, logging, and task output boundaries.
