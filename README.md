# TikTok Trend Shop

Foundation services for a TikTok hot-product-to-AI-video workflow.

## Stack

- Python 3.11+
- Standard library only for the foundation layer
- SQLite for local durable workflow records
- Local filesystem asset storage by default
- OpenSpec for change management

This stack keeps the first implementation portable and easy to test while later modules add provider integrations, UI, video rendering, or queue backends.

## Local Commands

```powershell
$env:PYTHONPATH = "src"
python -m tiktok_trend_shop.cli config
python -m tiktok_trend_shop.cli migrate
python -m unittest discover -s tests
openspec validate bootstrap-app-foundation
```

## Configuration

Copy `.env.example` to `.env` for local development and replace values locally. Real API keys must stay out of source control.

The foundation reads environment variables directly. Provider names are listed in `TTS_SOURCE_PROVIDERS` and `TTS_AI_PROVIDERS`. A provider marked enabled requires its matching API key unless it is a local/manual provider that does not need credentials.

## Modules

- `tiktok_trend_shop.config`: environment loading and secret redaction
- `tiktok_trend_shop.db`: SQLite connection and migrations
- `tiktok_trend_shop.repositories`: workflow record persistence
- `tiktok_trend_shop.jobs`: background job lifecycle
- `tiktok_trend_shop.assets`: local and external asset references
- `tiktok_trend_shop.audit`: source and generated output audit events
- `tiktok_trend_shop.api`: minimal API scaffold
- `tiktok_trend_shop.worker`: worker scaffold
