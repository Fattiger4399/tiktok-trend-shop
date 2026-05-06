## Context

The system needs to decide which products are worth turning into videos. This requires both quantitative signals from trend snapshots and qualitative synthesis from product details, reviews, competitor creatives, and policy-sensitive claims.

## Goals / Non-Goals

**Goals:**
- Produce ranked product opportunities with explainable score components.
- Generate product research cards that can ground scripts and videos.
- Preserve evidence links and confidence levels.
- Flag claims and product categories that need human review.

**Non-Goals:**
- Guarantee sales performance.
- Replace legal or platform policy review.
- Generate final ad scripts or rendered videos.

## Decisions

- Use a versioned scoring model. Score weights will change as data improves, so each score must include a model version and component breakdown.
- Combine deterministic metrics with AI-assisted synthesis. Numeric scoring handles trend and economics, while the research card uses structured generation for audience, pain points, objections, and creative angles.
- Require evidence references for research claims. Each selling point should be traceable to product details, reviews, creator videos, or manually entered evidence.
- Store confidence and risk flags separately from opportunity score. A product can be hot but risky, and downstream modules must be able to filter or require review.

## Risks / Trade-offs

- Scores can create false precision -> Show component breakdowns and confidence instead of only a single number.
- AI-generated research may invent claims -> Require source-backed fields and validation before creative use.
- Sparse data can bias results -> Mark low-data products and allow manual enrichment.
- Some product categories are policy-sensitive -> Add risk categories early and route them to review.
