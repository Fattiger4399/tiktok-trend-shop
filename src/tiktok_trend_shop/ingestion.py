from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Iterable, Protocol
import sqlite3

from .audit import AuditService
from .config import ProviderConfig
from .jobs import JobRepository
from .repositories import SourceHealthRepository, WorkflowRepository, new_id, to_json, utc_now


class ConnectorError(RuntimeError):
    """Raised when a source connector fails."""


class RateLimitError(ConnectorError):
    def __init__(self, message: str, *, retry_after: str | None = None) -> None:
        super().__init__(message)
        self.retry_after = retry_after


@dataclass(frozen=True)
class CollectionRequest:
    provider: str
    region: str
    category: str | None = None
    keyword: str | None = None
    product_url: str | None = None
    mode: str = "manual"
    window_start: str | None = None
    window_end: str | None = None
    metadata: dict[str, Any] = field(default_factory=dict)


@dataclass(frozen=True)
class CollectedRecord:
    provider: str
    product: dict[str, Any]
    metrics: dict[str, Any] = field(default_factory=dict)
    videos: tuple[dict[str, Any], ...] = ()
    creators: tuple[dict[str, Any], ...] = ()
    shops: tuple[dict[str, Any], ...] = ()
    hashtags: tuple[dict[str, Any], ...] = ()
    raw_ref: str | None = None
    captured_at: str | None = None


@dataclass(frozen=True)
class NormalizedProduct:
    title: str
    region: str
    category: str | None = None
    canonical_url: str | None = None
    source_id: str | None = None
    source_url: str | None = None
    metadata: dict[str, Any] = field(default_factory=dict)


@dataclass(frozen=True)
class NormalizedCollection:
    product: NormalizedProduct
    metrics: dict[str, Any]
    entities: tuple[dict[str, Any], ...]
    raw_ref: str | None
    captured_at: str | None


class TrendSourceConnector(Protocol):
    name: str

    def collect(self, request: CollectionRequest) -> Iterable[CollectedRecord]:
        ...


class SourceProviderRegistry:
    def __init__(self, providers: Iterable[ProviderConfig]) -> None:
        self.providers = {provider.name: provider for provider in providers}

    def validate_enabled(self, provider_name: str) -> ProviderConfig:
        provider = self.providers.get(provider_name)
        if provider is None:
            raise ValueError(f"Unknown source provider: {provider_name}")
        provider.validate()
        if not provider.enabled:
            raise ValueError(f"Source provider is disabled: {provider_name}")
        return provider


class ManualImportConnector:
    name = "manual"

    def __init__(self, records: Iterable[dict[str, Any]] | None = None) -> None:
        self.records = tuple(records or ())

    def collect(self, request: CollectionRequest) -> Iterable[CollectedRecord]:
        for raw in self.records:
            if request.product_url and raw.get("canonical_url") != request.product_url:
                continue
            if request.keyword and request.keyword.lower() not in raw.get("title", "").lower():
                continue
            if request.category and raw.get("category") not in {None, request.category}:
                continue
            yield CollectedRecord(
                provider=self.name,
                product={
                    "title": raw["title"],
                    "region": raw.get("region", request.region),
                    "category": raw.get("category", request.category),
                    "canonical_url": raw.get("canonical_url") or request.product_url,
                    "source_id": raw.get("source_id"),
                    "source_url": raw.get("source_url") or raw.get("canonical_url"),
                    "metadata": raw.get("metadata", {}),
                },
                metrics=raw.get("metrics", {}),
                videos=tuple(raw.get("videos", ())),
                creators=tuple(raw.get("creators", ())),
                shops=tuple(raw.get("shops", ())),
                hashtags=tuple(raw.get("hashtags", ())),
                raw_ref=raw.get("raw_ref"),
                captured_at=raw.get("captured_at"),
            )


def normalize_record(record: CollectedRecord, request: CollectionRequest) -> NormalizedCollection:
    product = record.product
    normalized_product = NormalizedProduct(
        title=str(product["title"]).strip(),
        region=str(product.get("region") or request.region),
        category=product.get("category") or request.category,
        canonical_url=product.get("canonical_url"),
        source_id=product.get("source_id"),
        source_url=product.get("source_url") or product.get("canonical_url"),
        metadata=dict(product.get("metadata") or {}),
    )
    entities: list[dict[str, Any]] = []
    for entity_type, rows in (
        ("video", record.videos),
        ("creator", record.creators),
        ("shop", record.shops),
        ("hashtag", record.hashtags),
    ):
        for row in rows:
            entities.append(
                {
                    "entity_type": entity_type,
                    "provider": record.provider,
                    "source_id": row.get("source_id") or row.get("id"),
                    "source_url": row.get("source_url") or row.get("url"),
                    "name": row.get("name") or row.get("title") or row.get("handle"),
                    "metrics": row.get("metrics", {}),
                    "metadata": {k: v for k, v in row.items() if k not in {"metrics"}},
                }
            )
    return NormalizedCollection(
        product=normalized_product,
        metrics=dict(record.metrics),
        entities=tuple(entities),
        raw_ref=record.raw_ref,
        captured_at=record.captured_at,
    )


class CollectionScheduler:
    def __init__(self, conn: sqlite3.Connection, jobs: JobRepository) -> None:
        self.conn = conn
        self.jobs = jobs

    def create_schedule(
        self,
        *,
        provider: str,
        region: str,
        category: str | None = None,
        keyword: str | None = None,
        mode: str = "scheduled",
        interval_minutes: int = 1440,
        next_run_at: str | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> str:
        schedule_id = new_id("schedule")
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                INSERT INTO collection_schedules(
                    id, provider, region, category, keyword, mode, interval_minutes,
                    next_run_at, metadata_json, created_at, updated_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    schedule_id,
                    provider,
                    region,
                    category,
                    keyword,
                    mode,
                    interval_minutes,
                    next_run_at,
                    to_json(metadata),
                    now,
                    now,
                ),
            )
        return schedule_id

    def schedule_due_collection(self, schedule_id: str) -> str:
        row = self.conn.execute(
            "SELECT * FROM collection_schedules WHERE id = ? AND enabled = 1",
            (schedule_id,),
        ).fetchone()
        if row is None:
            raise KeyError(f"Enabled schedule not found: {schedule_id}")
        job = self.jobs.create_job(
            job_type="trend_collection",
            input_ref_type="collection_schedule",
            input_ref_id=schedule_id,
            payload={
                "provider": row["provider"],
                "region": row["region"],
                "category": row["category"],
                "keyword": row["keyword"],
                "mode": row["mode"],
            },
            max_retries=2,
        )
        return job.id


class IngestionService:
    def __init__(
        self,
        *,
        workflow: WorkflowRepository,
        jobs: JobRepository,
        health: SourceHealthRepository,
        audit: AuditService,
        connectors: Iterable[TrendSourceConnector],
        registry: SourceProviderRegistry | None = None,
    ) -> None:
        self.workflow = workflow
        self.jobs = jobs
        self.health = health
        self.audit = audit
        self.connectors = {connector.name: connector for connector in connectors}
        self.registry = registry

    def trigger_manual_collection(self, request: CollectionRequest) -> str:
        job = self.jobs.create_job(
            job_type="manual_trend_collection",
            input_ref_type="manual_request",
            input_ref_id=request.product_url or request.keyword or request.category,
            payload={
                "provider": request.provider,
                "region": request.region,
                "category": request.category,
                "keyword": request.keyword,
                "product_url": request.product_url,
                "mode": request.mode,
                "metadata": request.metadata,
            },
            max_retries=1,
        )
        return job.id

    def collect(self, request: CollectionRequest) -> list[str]:
        if self.registry:
            self.registry.validate_enabled(request.provider)
        connector = self.connectors.get(request.provider)
        if connector is None:
            raise ValueError(f"No connector registered for provider: {request.provider}")
        product_ids: list[str] = []
        try:
            for record in connector.collect(request):
                normalized = normalize_record(record, request)
                product = self.workflow.upsert_product_candidate(
                    title=normalized.product.title,
                    region=normalized.product.region,
                    category=normalized.product.category,
                    canonical_url=normalized.product.canonical_url,
                    provider=record.provider,
                    source_id=normalized.product.source_id,
                    metadata={
                        **normalized.product.metadata,
                        "source_provider": record.provider,
                    },
                )
                snapshot_id = self.workflow.add_source_snapshot(
                    product_id=product.id,
                    provider=record.provider,
                    source_type="product",
                    source_id=normalized.product.source_id,
                    source_url=normalized.product.source_url,
                    metrics=normalized.metrics,
                    raw_ref=normalized.raw_ref,
                    metadata={
                        "category": normalized.product.category,
                        "canonical_url": normalized.product.canonical_url,
                    },
                    captured_at=normalized.captured_at,
                )
                self.audit.record_external_input(
                    subject_type="source_snapshot",
                    subject_id=snapshot_id,
                    provider=record.provider,
                    source_url=normalized.product.source_url,
                    source_id=normalized.product.source_id,
                )
                for entity in normalized.entities:
                    self.workflow.add_normalized_entity(product_id=product.id, **entity)
                product_ids.append(product.id)
            self.health.mark_success(request.provider)
        except RateLimitError as exc:
            self.health.mark_error(
                request.provider, str(exc), rate_limited_until=exc.retry_after
            )
            raise
        except Exception as exc:
            self.health.mark_error(request.provider, str(exc))
            raise
        return product_ids
