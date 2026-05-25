from __future__ import annotations

from dataclasses import dataclass, field
import sqlite3
from typing import Any

from .audit import AuditService
from .domain.models import ProductDetailSnapshot
from .repositories import WorkflowRepository, from_json, new_id, to_json, utc_now


REQUIRED_DETAIL_FIELDS = (
    "product_url",
    "image_url",
    "price",
    "selling_points",
    "review_summary",
)


@dataclass(frozen=True)
class ProductDetailPayload:
    provider: str = "manual"
    platform: str | None = None
    source_id: str | None = None
    source_url: str | None = None
    product_url: str | None = None
    title: str | None = None
    shop_name: str | None = None
    brand: str | None = None
    price: float | None = None
    currency: str | None = None
    image_url: str | None = None
    specs: dict[str, Any] = field(default_factory=dict)
    selling_points: tuple[str, ...] = ()
    review_summary: str | None = None
    review_highlights: tuple[str, ...] = ()
    warnings: tuple[str, ...] = ()
    raw: dict[str, Any] = field(default_factory=dict)
    metadata: dict[str, Any] = field(default_factory=dict)
    captured_at: str | None = None


class ProductDetailRepository:
    def __init__(self, conn: sqlite3.Connection) -> None:
        self.conn = conn

    def save_snapshot(
        self,
        *,
        product_id: str,
        payload: ProductDetailPayload,
        missing_fields: tuple[str, ...],
        completeness_status: str,
    ) -> ProductDetailSnapshot:
        snapshot_id = new_id("detail")
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                INSERT INTO product_detail_snapshots(
                    id, product_id, provider, platform, source_id, source_url,
                    product_url, title, shop_name, brand, price, currency, image_url,
                    specs_json, selling_points_json, review_summary,
                    review_highlights_json, warnings_json, missing_fields_json,
                    completeness_status, raw_json, metadata_json, captured_at, created_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    snapshot_id,
                    product_id,
                    payload.provider,
                    payload.platform,
                    payload.source_id,
                    payload.source_url,
                    payload.product_url,
                    payload.title,
                    payload.shop_name,
                    payload.brand,
                    payload.price,
                    payload.currency,
                    payload.image_url,
                    to_json(payload.specs),
                    to_json({"items": list(payload.selling_points)}),
                    payload.review_summary,
                    to_json({"items": list(payload.review_highlights)}),
                    to_json({"items": list(payload.warnings)}),
                    to_json({"items": list(missing_fields)}),
                    completeness_status,
                    to_json(payload.raw),
                    to_json(payload.metadata),
                    payload.captured_at or now,
                    now,
                ),
            )
        return self.get_snapshot(snapshot_id)

    def get_snapshot(self, snapshot_id: str) -> ProductDetailSnapshot:
        row = self.conn.execute(
            "SELECT * FROM product_detail_snapshots WHERE id = ?", (snapshot_id,)
        ).fetchone()
        if row is None:
            raise KeyError(f"Product detail snapshot not found: {snapshot_id}")
        return _detail_from_row(row)

    def get_latest_snapshot(self, product_id: str) -> ProductDetailSnapshot | None:
        row = self.conn.execute(
            """
            SELECT *
            FROM product_detail_snapshots
            WHERE product_id = ?
            ORDER BY captured_at DESC, created_at DESC
            LIMIT 1
            """,
            (product_id,),
        ).fetchone()
        return _detail_from_row(row) if row else None

    def list_snapshots(self, product_id: str) -> list[ProductDetailSnapshot]:
        rows = self.conn.execute(
            """
            SELECT *
            FROM product_detail_snapshots
            WHERE product_id = ?
            ORDER BY captured_at DESC, created_at DESC
            """,
            (product_id,),
        ).fetchall()
        return [_detail_from_row(row) for row in rows]


class ProductDetailEnrichmentService:
    def __init__(
        self,
        *,
        details: ProductDetailRepository,
        workflow: WorkflowRepository,
        audit: AuditService,
    ) -> None:
        self.details = details
        self.workflow = workflow
        self.audit = audit

    def enrich_product(
        self, product_id: str, payload: ProductDetailPayload
    ) -> ProductDetailSnapshot:
        self.workflow.get_product(product_id)
        missing_fields = compute_missing_detail_fields(payload)
        status = "complete" if not missing_fields else "incomplete"
        snapshot = self.details.save_snapshot(
            product_id=product_id,
            payload=payload,
            missing_fields=missing_fields,
            completeness_status=status,
        )
        self.audit.record_external_input(
            subject_type="product_detail_snapshot",
            subject_id=snapshot.id,
            provider=payload.provider,
            source_url=payload.source_url or payload.product_url,
            source_id=payload.source_id,
            metadata={
                "product_id": product_id,
                "platform": payload.platform,
                "completeness_status": status,
                "missing_fields": list(missing_fields),
            },
        )
        return snapshot


def compute_missing_detail_fields(payload: ProductDetailPayload) -> tuple[str, ...]:
    missing: list[str] = []
    if not payload.product_url:
        missing.append("product_url")
    if not payload.image_url:
        missing.append("image_url")
    if payload.price is None:
        missing.append("price")
    if not payload.selling_points:
        missing.append("selling_points")
    if not payload.review_summary:
        missing.append("review_summary")
    return tuple(missing)


def product_detail_texts(snapshot: ProductDetailSnapshot | None) -> tuple[str, ...]:
    if snapshot is None:
        return ()
    texts: list[str] = []
    for value in (
        snapshot.title,
        snapshot.shop_name,
        snapshot.brand,
        snapshot.review_summary,
        snapshot.product_url,
    ):
        if value:
            texts.append(value)
    texts.extend(snapshot.selling_points)
    texts.extend(snapshot.review_highlights)
    texts.extend(snapshot.warnings)
    for key, value in snapshot.specs.items():
        texts.append(f"{key}: {value}")
    return tuple(texts)


def payload_from_csv_row(
    row: dict[str, str], *, default_provider: str = "manual-csv"
) -> ProductDetailPayload | None:
    normalized = {_normalize_key(key): value for key, value in row.items()}
    product_url = _pick(normalized, "product_url", "canonical_url", "url", "item_url", "goods_url")
    image_url = _pick(normalized, "image_url", "main_image_url", "product_image", "image", "pic_url")
    price_text = _pick(normalized, "price", "sale_price", "current_price", "券后价")
    selling_points = _parse_list(
        _pick(normalized, "selling_points", "selling_point", "卖点", "商品卖点", "product_points")
    )
    specs = _parse_specs(_pick(normalized, "specs", "spec", "sku", "规格", "参数"))
    review_summary = _pick(normalized, "review_summary", "reviews", "评价摘要", "评价")
    review_highlights = _parse_list(
        _pick(normalized, "review_highlights", "review_points", "好评点", "评价亮点")
    )
    warnings = _parse_list(_pick(normalized, "warnings", "risk_warnings", "合规提示", "风险提示"))
    has_detail = any(
        (
            product_url,
            image_url,
            price_text,
            selling_points,
            specs,
            review_summary,
            review_highlights,
            warnings,
            _pick(normalized, "shop_name", "shop", "店铺", "店铺名"),
            _pick(normalized, "brand", "品牌"),
            _pick(normalized, "platform", "平台"),
        )
    )
    if not has_detail:
        return None
    return ProductDetailPayload(
        provider=_pick(normalized, "detail_provider", "provider") or default_provider,
        platform=_pick(normalized, "platform", "平台"),
        source_id=_pick(normalized, "item_id", "goods_id", "source_id", "商品id"),
        source_url=_pick(normalized, "source_url", "detail_source_url") or product_url,
        product_url=product_url,
        title=_pick(normalized, "detail_title", "product_title", "title", "商品名"),
        shop_name=_pick(normalized, "shop_name", "shop", "店铺", "店铺名"),
        brand=_pick(normalized, "brand", "品牌"),
        price=_parse_price(price_text),
        currency=_pick(normalized, "currency", "币种") or "CNY",
        image_url=image_url,
        specs=specs,
        selling_points=tuple(selling_points),
        review_summary=review_summary,
        review_highlights=tuple(review_highlights),
        warnings=tuple(warnings),
        raw={key: value for key, value in row.items() if value},
        metadata={"csv_detail": True},
    )


def _detail_from_row(row: sqlite3.Row) -> ProductDetailSnapshot:
    return ProductDetailSnapshot(
        id=row["id"],
        product_id=row["product_id"],
        provider=row["provider"],
        platform=row["platform"],
        source_id=row["source_id"],
        source_url=row["source_url"],
        product_url=row["product_url"],
        title=row["title"],
        shop_name=row["shop_name"],
        brand=row["brand"],
        price=float(row["price"]) if row["price"] is not None else None,
        currency=row["currency"],
        image_url=row["image_url"],
        specs=from_json(row["specs_json"]),
        selling_points=tuple(from_json(row["selling_points_json"]).get("items", ())),
        review_summary=row["review_summary"],
        review_highlights=tuple(from_json(row["review_highlights_json"]).get("items", ())),
        warnings=tuple(from_json(row["warnings_json"]).get("items", ())),
        missing_fields=tuple(from_json(row["missing_fields_json"]).get("items", ())),
        completeness_status=row["completeness_status"],
        raw=from_json(row["raw_json"]),
        metadata=from_json(row["metadata_json"]),
        captured_at=row["captured_at"],
        created_at=row["created_at"],
    )


def _normalize_key(value: str) -> str:
    return value.strip().lower().replace(" ", "_").replace("-", "_")


def _pick(row: dict[str, str], *keys: str) -> str | None:
    for key in keys:
        value = (row.get(_normalize_key(key)) or "").strip()
        if value:
            return value
    return None


def _parse_list(value: str | None) -> list[str]:
    if not value:
        return []
    text = value.replace("\r\n", "\n").replace("\r", "\n")
    for sep in ("；", ";", "|", "、", "\n"):
        text = text.replace(sep, ",")
    return [item.strip() for item in text.split(",") if item.strip()]


def _parse_specs(value: str | None) -> dict[str, Any]:
    items = _parse_list(value)
    if not items:
        return {}
    specs: dict[str, Any] = {}
    loose: list[str] = []
    for item in items:
        if ":" in item:
            key, raw = item.split(":", 1)
            specs[key.strip()] = raw.strip()
        elif "：" in item:
            key, raw = item.split("：", 1)
            specs[key.strip()] = raw.strip()
        else:
            loose.append(item)
    if loose:
        specs["items"] = loose
    return specs


def _parse_price(value: str | None) -> float | None:
    if not value:
        return None
    text = (
        value.strip()
        .replace(",", "")
        .replace("￥", "")
        .replace("¥", "")
        .replace("元", "")
        .replace("CNY", "")
        .replace("cny", "")
    )
    if "~" in text:
        left, right = text.split("~", 1)
        left_price = _parse_price(left)
        right_price = _parse_price(right)
        if left_price is not None and right_price is not None:
            return round((left_price + right_price) / 2, 2)
    try:
        return float(text)
    except ValueError:
        return None
