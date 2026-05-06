## Context

The product should not treat generated content as publishable by default. Review must cover factual claims, policy risks, copyrighted or unlicensed assets, AI-generated media disclosure, and basic content quality before export or publishing.

## Goals / Non-Goals

**Goals:**
- Provide a review queue for generated product materials and videos.
- Enforce an approval state before export or publishing readiness.
- Capture reviewer feedback and route revisions to the correct upstream module.
- Produce export packages that include media and publishing copy.

**Non-Goals:**
- Provide legal advice or guarantee platform approval.
- Automatically fix every compliance issue.
- Publish videos directly to TikTok.

## Decisions

- Use explicit review states: draft, needs review, changes requested, approved, rejected, and exported. This gives later publishing logic a clean readiness signal.
- Review at artifact level and package level. Product research, scripts, storyboards, and renders can each have issues, while export readiness depends on the final package.
- Store checklist results as structured data. This allows filtering, analytics, and later automation without losing reviewer context.
- Route feedback to upstream artifact versions. A script issue should create a new script version, while a render issue should trigger a video regeneration path.

## Risks / Trade-offs

- Too much review slows output -> Default checklist should be compact and focused on real risk.
- Reviewers may skip evidence checks -> Require evidence status before approval for claim-heavy content.
- Export packages can drift from approved assets -> Package creation must use immutable approved version references.
- Platform policy changes -> Keep checklist items configurable and versioned.
