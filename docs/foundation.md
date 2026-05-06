# Foundation Contracts

The `bootstrap-app-foundation` change establishes the contracts that later modules depend on.

## Configuration

`AppConfig.from_env()` loads runtime settings from environment variables and validates enabled providers. Use `config.redacted()` for logs or API responses. Never print raw provider credentials.

Important variables:

- `TTS_DATABASE_URL`: SQLite URL, for example `sqlite:///./data/tiktok_trend_shop.sqlite3`
- `TTS_STORAGE_BACKEND`: `local` or `external`
- `TTS_STORAGE_ROOT`: local asset folder
- `TTS_SOURCE_PROVIDERS`: comma-separated source provider names
- `TTS_AI_PROVIDERS`: comma-separated AI provider names

Provider-specific values use this pattern:

```text
TTS_SOURCE_<NAME>_ENABLED=true
TTS_SOURCE_<NAME>_BASE_URL=https://provider.example
TTS_SOURCE_<NAME>_API_KEY=local-secret
```

## Persistence

SQLite migrations create durable records for:

- `products`
- `source_snapshots`
- `jobs`
- `assets`
- `review_states`
- `analytics_refs`
- `audit_events`

Use `Migrator(conn).migrate()` before repository access.

## Repositories

`WorkflowRepository` owns product workflow records, source snapshots, review states, and analytics references.

`AssetRepository` owns asset metadata. Media bytes are stored through an asset store, not directly in workflow rows.

## Jobs

`JobRepository` tracks long-running work using these statuses:

- `queued`
- `running`
- `succeeded`
- `failed`
- `canceled`

Failed jobs record `last_error` and increment `retry_count`. Retryable jobs can be returned to `queued` with `retry_job()`.

## Assets

`LocalAssetStore` writes bytes to local storage and records metadata with checksum, content type, size, and URI.

`ExternalAssetRegistry` records provider-backed assets that already exist outside the local filesystem.

## Audit

`AuditService` records traceability for external inputs and generated outputs. Later modules should include provider names, source URLs or IDs, prompt versions, models, renderers, and other useful metadata.
