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

    def get_product(self, product_id: str) -> ProductRecord:
        row = self.conn.execute("SELECT * FROM products WHERE id = ?", (product_id,)).fetchone()
        if row is None:
            raise KeyError(f"Product not found: {product_id}")
        return _product_from_row(row)

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
