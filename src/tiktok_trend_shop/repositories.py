from __future__ import annotations

from datetime import UTC, datetime
import json
import sqlite3
from typing import Any
from uuid import uuid4

from .domain.models import AssetRecord, ProductRecord


def utc_now() -> str:
    return datetime.now(UTC).isoformat(timespec="seconds")


def new_id(prefix: str) -> str:
    return f"{prefix}_{uuid4().hex}"


def to_json(value: dict[str, Any] | None) -> str:
    return json.dumps(value or {}, sort_keys=True, separators=(",", ":"))


def from_json(value: str | None) -> dict[str, Any]:
    if not value:
        return {}
    return json.loads(value)


class WorkflowRepository:
    def __init__(self, conn: sqlite3.Connection) -> None:
        self.conn = conn

    def create_product(
        self,
        *,
        title: str,
        region: str,
        category: str | None = None,
        canonical_url: str | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> ProductRecord:
        now = utc_now()
        product_id = new_id("prod")
        with self.conn:
            self.conn.execute(
                """
                INSERT INTO products(
                    id, canonical_url, title, region, category, workflow_state,
                    metadata_json, created_at, updated_at
                )
                VALUES (?, ?, ?, ?, ?, 'candidate', ?, ?, ?)
                """,
                (
                    product_id,
                    canonical_url,
                    title,
                    region,
                    category,
                    to_json(metadata),
                    now,
                    now,
                ),
            )
        return self.get_product(product_id)

    def find_product_by_canonical_url(self, canonical_url: str) -> ProductRecord | None:
        row = self.conn.execute(
            "SELECT * FROM products WHERE canonical_url = ? ORDER BY created_at LIMIT 1",
            (canonical_url,),
        ).fetchone()
        return _product_from_row(row) if row else None

    def find_product_by_title_region(self, title: str, region: str) -> ProductRecord | None:
        row = self.conn.execute(
            """
            SELECT * FROM products
            WHERE lower(title) = lower(?) AND region = ?
            ORDER BY created_at
            LIMIT 1
            """,
            (title.strip(), region),
        ).fetchone()
        return _product_from_row(row) if row else None

    def find_product_by_source(self, provider: str, source_id: str) -> ProductRecord | None:
        row = self.conn.execute(
            """
            SELECT p.*
            FROM products p
            JOIN source_snapshots s ON s.product_id = p.id
            WHERE s.provider = ? AND s.source_id = ?
            ORDER BY s.created_at
            LIMIT 1
            """,
            (provider, source_id),
        ).fetchone()
        return _product_from_row(row) if row else None

    def upsert_product_candidate(
        self,
        *,
        title: str,
        region: str,
        category: str | None = None,
        canonical_url: str | None = None,
        provider: str | None = None,
        source_id: str | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> ProductRecord:
        existing: ProductRecord | None = None
        if provider and source_id:
            existing = self.find_product_by_source(provider, source_id)
        if existing is None and canonical_url:
            existing = self.find_product_by_canonical_url(canonical_url)
        if existing is None:
            existing = self.find_product_by_title_region(title, region)
        if existing is not None:
            return existing
        return self.create_product(
            title=title,
            region=region,
            category=category,
            canonical_url=canonical_url,
            metadata=metadata,
        )

    def get_product(self, product_id: str) -> ProductRecord:
        row = self.conn.execute("SELECT * FROM products WHERE id = ?", (product_id,)).fetchone()
        if row is None:
            raise KeyError(f"Product not found: {product_id}")
        return _product_from_row(row)

    def list_products(self, *, limit: int | None = None) -> list[ProductRecord]:
        sql = "SELECT * FROM products ORDER BY created_at"
        params: tuple[Any, ...] = ()
        if limit is not None:
            sql += " LIMIT ?"
            params = (limit,)
        rows = self.conn.execute(sql, params).fetchall()
        return [_product_from_row(row) for row in rows]

    def update_product_state(self, product_id: str, state: str) -> ProductRecord:
        now = utc_now()
        with self.conn:
            self.conn.execute(
                "UPDATE products SET workflow_state = ?, updated_at = ? WHERE id = ?",
                (state, now, product_id),
            )
        return self.get_product(product_id)

    def add_source_snapshot(
        self,
        *,
        product_id: str,
        provider: str,
        source_type: str,
        source_id: str | None = None,
        source_url: str | None = None,
        metrics: dict[str, Any] | None = None,
        raw_ref: str | None = None,
        metadata: dict[str, Any] | None = None,
        captured_at: str | None = None,
    ) -> str:
        snapshot_id = new_id("snap")
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                INSERT INTO source_snapshots(
                    id, product_id, provider, source_type, source_id, source_url,
                    captured_at, metrics_json, raw_ref, metadata_json, created_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    snapshot_id,
                    product_id,
                    provider,
                    source_type,
                    source_id,
                    source_url,
                    captured_at or now,
                    to_json(metrics),
                    raw_ref,
                    to_json(metadata),
                    now,
                ),
            )
        return snapshot_id

    def create_review_state(
        self,
        *,
        artifact_type: str,
        artifact_id: str,
        state: str,
        product_id: str | None = None,
        reviewer: str | None = None,
        notes: str | None = None,
        checklist: dict[str, Any] | None = None,
    ) -> str:
        review_id = new_id("review")
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                INSERT INTO review_states(
                    id, artifact_type, artifact_id, product_id, state, reviewer,
                    notes, checklist_json, created_at, updated_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    review_id,
                    artifact_type,
                    artifact_id,
                    product_id,
                    state,
                    reviewer,
                    notes,
                    to_json(checklist),
                    now,
                    now,
                ),
            )
        return review_id

    def create_analytics_ref(
        self,
        *,
        product_id: str,
        channel: str,
        external_id: str | None = None,
        metrics: dict[str, Any] | None = None,
        collected_at: str | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> str:
        analytics_id = new_id("metric")
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                INSERT INTO analytics_refs(
                    id, product_id, channel, external_id, metrics_json,
                    collected_at, metadata_json, created_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    analytics_id,
                    product_id,
                    channel,
                    external_id,
                    to_json(metrics),
                    collected_at or now,
                    to_json(metadata),
                    now,
                ),
            )
        return analytics_id

    def add_normalized_entity(
        self,
        *,
        entity_type: str,
        provider: str,
        product_id: str | None = None,
        source_id: str | None = None,
        source_url: str | None = None,
        name: str | None = None,
        metrics: dict[str, Any] | None = None,
        metadata: dict[str, Any] | None = None,
        captured_at: str | None = None,
    ) -> str:
        entity_id = new_id("entity")
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                INSERT INTO normalized_entities(
                    id, product_id, entity_type, provider, source_id, source_url,
                    name, metrics_json, metadata_json, captured_at, created_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    entity_id,
                    product_id,
                    entity_type,
                    provider,
                    source_id,
                    source_url,
                    name,
                    to_json(metrics),
                    to_json(metadata),
                    captured_at or now,
                    now,
                ),
            )
        return entity_id


class SourceHealthRepository:
    def __init__(self, conn: sqlite3.Connection) -> None:
        self.conn = conn

    def configure_provider(
        self, provider: str, *, enabled: bool = True, metadata: dict[str, Any] | None = None
    ) -> dict[str, Any]:
        now = utc_now()
        existing = self.get_provider_state(provider)
        with self.conn:
            if existing:
                self.conn.execute(
                    """
                    UPDATE source_provider_states
                    SET enabled = ?, metadata_json = ?, updated_at = ?
                    WHERE provider = ?
                    """,
                    (int(enabled), to_json(metadata), now, provider),
                )
            else:
                self.conn.execute(
                    """
                    INSERT INTO source_provider_states(
                        provider, enabled, status, metadata_json, created_at, updated_at
                    )
                    VALUES (?, ?, 'healthy', ?, ?, ?)
                    """,
                    (provider, int(enabled), to_json(metadata), now, now),
                )
        return self.get_provider_state(provider) or {}

    def mark_success(self, provider: str) -> dict[str, Any]:
        now = utc_now()
        self.configure_provider(provider)
        with self.conn:
            self.conn.execute(
                """
                UPDATE source_provider_states
                SET status = 'healthy', last_success_at = ?, last_error = NULL,
                    rate_limited_until = NULL, updated_at = ?
                WHERE provider = ?
                """,
                (now, now, provider),
            )
        return self.get_provider_state(provider) or {}

    def mark_error(
        self, provider: str, error: str, *, rate_limited_until: str | None = None
    ) -> dict[str, Any]:
        now = utc_now()
        self.configure_provider(provider)
        status = "rate_limited" if rate_limited_until else "degraded"
        with self.conn:
            self.conn.execute(
                """
                UPDATE source_provider_states
                SET status = ?, last_error = ?, rate_limited_until = ?, updated_at = ?
                WHERE provider = ?
                """,
                (status, error, rate_limited_until, now, provider),
            )
        return self.get_provider_state(provider) or {}

    def get_provider_state(self, provider: str) -> dict[str, Any] | None:
        row = self.conn.execute(
            "SELECT * FROM source_provider_states WHERE provider = ?", (provider,)
        ).fetchone()
        if row is None:
            return None
        return {
            "provider": row["provider"],
            "enabled": bool(row["enabled"]),
            "status": row["status"],
            "last_success_at": row["last_success_at"],
            "last_error": row["last_error"],
            "rate_limited_until": row["rate_limited_until"],
            "metadata": from_json(row["metadata_json"]),
            "created_at": row["created_at"],
            "updated_at": row["updated_at"],
        }


class AssetRepository:
    def __init__(self, conn: sqlite3.Connection) -> None:
        self.conn = conn

    def create_asset(
        self,
        *,
        kind: str,
        backend: str,
        uri: str,
        content_type: str | None = None,
        byte_size: int | None = None,
        checksum: str | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> AssetRecord:
        asset_id = new_id("asset")
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                INSERT INTO assets(
                    id, kind, backend, uri, content_type, byte_size,
                    checksum, metadata_json, created_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    asset_id,
                    kind,
                    backend,
                    uri,
                    content_type,
                    byte_size,
                    checksum,
                    to_json(metadata),
                    now,
                ),
            )
        return self.get_asset(asset_id)

    def get_asset(self, asset_id: str) -> AssetRecord:
        row = self.conn.execute("SELECT * FROM assets WHERE id = ?", (asset_id,)).fetchone()
        if row is None:
            raise KeyError(f"Asset not found: {asset_id}")
        return _asset_from_row(row)


def _product_from_row(row: sqlite3.Row) -> ProductRecord:
    return ProductRecord(
        id=row["id"],
        title=row["title"],
        region=row["region"],
        category=row["category"],
        workflow_state=row["workflow_state"],
        canonical_url=row["canonical_url"],
        metadata=from_json(row["metadata_json"]),
        created_at=row["created_at"],
        updated_at=row["updated_at"],
    )


def _asset_from_row(row: sqlite3.Row) -> AssetRecord:
    return AssetRecord(
        id=row["id"],
        kind=row["kind"],
        backend=row["backend"],
        uri=row["uri"],
        content_type=row["content_type"],
        byte_size=row["byte_size"],
        checksum=row["checksum"],
        metadata=from_json(row["metadata_json"]),
        created_at=row["created_at"],
    )
