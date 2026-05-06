## 1. Connector Contracts

- [ ] 1.1 Define connector interfaces for configured collection inputs and normalized outputs.
- [ ] 1.2 Add source provider configuration records and validation.
- [ ] 1.3 Add a manual import connector for early product and metric ingestion.

## 2. Collection Jobs

- [ ] 2.1 Implement scheduled collection job creation by provider, region, category, and keyword.
- [ ] 2.2 Implement manual collection triggers for product URLs, keywords, and categories.
- [ ] 2.3 Add rate-limit and retry handling for connector jobs.

## 3. Normalization

- [ ] 3.1 Add normalized data models for products, videos, creators, shops, hashtags, and metrics.
- [ ] 3.2 Implement normalization from connector payloads into source snapshots.
- [ ] 3.3 Implement deduplication rules and canonical source references.

## 4. Verification

- [ ] 4.1 Add tests for connector configuration validation.
- [ ] 4.2 Add tests for manual import normalization.
- [ ] 4.3 Add tests for product deduplication across repeated imports.
- [ ] 4.4 Add tests for failed connector health states.
