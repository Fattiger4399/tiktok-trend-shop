## Why

The product becomes more valuable when approved videos can be published or tracked and their performance can improve future selection and creative decisions. This change adds publishing readiness, publishing status tracking, performance ingestion, and feedback loops.

## What Changes

- Add publishing channel configuration for manual export tracking and official API-backed publishing when available.
- Add scheduling, publish attempt, status, and failure records.
- Add performance metric ingestion for views, engagement, clicks, conversions, and revenue when available.
- Link performance back to product scores, research cards, script versions, and rendered videos.
- Add feedback rules that can inform future product scoring and creative generation.

## Capabilities

### New Capabilities
- `publishing-analytics-loop`: Publishing readiness, publishing status tracking, performance metrics, attribution, and feedback into product and creative decisions.

### Modified Capabilities
- None.

## Impact

- Depends on `app-foundation`, `review-export-workflow`, and provider permissions for publishing or analytics APIs.
- Affects channel configuration, publish jobs, status polling, metrics ingestion, attribution records, and optimization reporting.
