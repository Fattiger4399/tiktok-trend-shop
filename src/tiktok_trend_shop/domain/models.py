from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class ProductRecord:
    id: str
    title: str
    region: str
    category: str | None
    workflow_state: str
    canonical_url: str | None
    metadata: dict[str, object]
    created_at: str
    updated_at: str


@dataclass(frozen=True)
class JobRecord:
    id: str
    job_type: str
    status: str
    input_ref_type: str | None
    input_ref_id: str | None
    payload: dict[str, object]
    retry_count: int
    max_retries: int
    last_error: str | None
    created_at: str
    updated_at: str
    started_at: str | None
    completed_at: str | None


@dataclass(frozen=True)
class AssetRecord:
    id: str
    kind: str
    backend: str
    uri: str
    content_type: str | None
    byte_size: int | None
    checksum: str | None
    metadata: dict[str, object]
    created_at: str


@dataclass(frozen=True)
class ProductDetailSnapshot:
    id: str
    product_id: str
    provider: str
    platform: str | None
    source_id: str | None
    source_url: str | None
    product_url: str | None
    title: str | None
    shop_name: str | None
    brand: str | None
    price: float | None
    currency: str | None
    image_url: str | None
    specs: dict[str, object]
    selling_points: tuple[str, ...]
    review_summary: str | None
    review_highlights: tuple[str, ...]
    warnings: tuple[str, ...]
    missing_fields: tuple[str, ...]
    completeness_status: str
    raw: dict[str, object]
    metadata: dict[str, object]
    captured_at: str
    created_at: str


@dataclass(frozen=True)
class AuditEvent:
    id: str
    subject_type: str
    subject_id: str
    event_type: str
    actor: str | None
    provider: str | None
    source_url: str | None
    source_id: str | None
    prompt_version: str | None
    model: str | None
    renderer: str | None
    metadata: dict[str, object]
    created_at: str
