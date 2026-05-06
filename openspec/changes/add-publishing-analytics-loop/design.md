## Context

Publishing is platform-sensitive and permission-dependent. The first implementation should support manual export tracking and leave a clear path for official API publishing once app permissions, OAuth, and platform review are in place.

## Goals / Non-Goals

**Goals:**
- Track whether approved export packages were published manually or through an integration.
- Store publish attempts, schedules, statuses, and errors.
- Collect or import performance metrics and attribute them to products, scripts, and videos.
- Feed performance signals back into scoring and creative decisions.

**Non-Goals:**
- Bypass TikTok official API permissions or posting review.
- Publish unapproved videos.
- Build a full ads manager or affiliate payout system.

## Decisions

- Support manual publishing records first. This makes analytics useful before official TikTok publishing permissions are available.
- Model publishing channels separately from export packages. A single approved package may be posted to multiple channels or accounts.
- Use status polling jobs for API-backed publishing. Provider status can lag, fail, or require retries.
- Store metrics as time-series snapshots. Trend analysis and feedback loops require changes over time, not only latest totals.
- Link every metric to product, script, video, export package, channel, and publish attempt when possible.

## Risks / Trade-offs

- API publishing may be delayed by platform approval -> Manual publishing records keep the workflow usable.
- Metrics may be incomplete or imported late -> Store source and freshness for every metric snapshot.
- Attribution can be ambiguous across reposts -> Require channel and publish attempt references where possible.
- Feedback loops can overfit early data -> Treat performance signals as advisory until enough volume exists.
