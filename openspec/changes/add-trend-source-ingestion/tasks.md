## 1. Connector Contracts

- [x] 1.1 Define connector interfaces for configured collection inputs and normalized outputs.
- [x] 1.2 Add source provider configuration records and validation.
- [x] 1.3 Add a manual import connector for early product and metric ingestion.

## 2. Collection Jobs

- [x] 2.1 Implement scheduled collection job creation by provider, region, category, and keyword.
- [x] 2.2 Implement manual collection triggers for product URLs, keywords, and categories.
- [x] 2.3 Add rate-limit and retry handling for connector jobs.

## 3. Normalization

- [x] 3.1 Add normalized data models for products, videos, creators, shops, hashtags, and metrics.
- [x] 3.2 Implement normalization from connector payloads into source snapshots.
- [x] 3.3 Implement deduplication rules and canonical source references.

## 4. Verification

- [x] 4.1 Add tests for connector configuration validation.
- [x] 4.2 Add tests for manual import normalization.
- [x] 4.3 Add tests for product deduplication across repeated imports.
- [x] 4.4 Add tests for failed connector health states.
