from __future__ import annotations

from pathlib import Path
import csv
import sqlite3
from typing import Any

from .assets import LocalAssetStore
from .audit import AuditService
from .creative import ScriptGenerationRequest, ScriptGenerator, ScriptRepository
from .db import Migrator
from .detail import (
    ProductDetailEnrichmentService,
    ProductDetailRepository,
    payload_from_csv_row,
)
from .ingestion import CollectionRequest, IngestionService, ManualImportConnector
from .intelligence import (
    EvidenceExtractor,
    OpportunityScorer,
    ProductIntelligenceRepository,
    ResearchCardGenerator,
)
from .jobs import JobRepository
from .publishing import MetricsSnapshot, PublishingRepository, PublishingService
from .repositories import (
    AssetRepository,
    SourceHealthRepository,
    WorkflowRepository,
    from_json,
)
from .review import (
    CHECKLIST_ITEMS,
    CHECKLIST_VERSION,
    ExportService,
    ReviewChecklist,
    ReviewRepository,
)
from .video import VideoGenerationService, VideoRepository


def run_manual_demo_pipeline(
    *,
    conn: sqlite3.Connection,
    storage_root: Path,
    title: str,
    region: str = "US",
    category: str | None = None,
    canonical_url: str | None = None,
    views: int = 1800,
    sales: int = 0,
    engagement: int = 120,
    clicks: int = 20,
    conversions: int = 1,
    revenue: float = 0.0,
) -> dict[str, Any]:
    Migrator(conn).migrate()
    workflow = WorkflowRepository(conn)
    jobs = JobRepository(conn)
    health = SourceHealthRepository(conn)
    audit = AuditService(conn)

    product_id = IngestionService(
        workflow=workflow,
        jobs=jobs,
        health=health,
        audit=audit,
        connectors=(
            ManualImportConnector(
                [
                    {
                        "title": title,
                        "region": region,
                        "category": category,
                        "canonical_url": canonical_url,
                        "source_id": canonical_url or title.lower().replace(" ", "-"),
                        "metrics": {
                            "views": views,
                            "sales": sales,
                            "engagement": engagement,
                        },
                    }
                ]
            ),
        ),
    ).collect(CollectionRequest(provider="manual", region=region, category=category))[0]

    return run_product_workflow(
        conn=conn,
        storage_root=storage_root,
        product_id=product_id,
        publishing_metrics={
            "views": views,
            "engagement": engagement,
            "clicks": clicks,
            "conversions": conversions,
            "revenue": revenue,
        },
    )


def import_products_csv(
    *,
    conn: sqlite3.Connection,
    csv_path: Path,
    default_region: str = "CN",
) -> dict[str, Any]:
    Migrator(conn).migrate()
    workflow = WorkflowRepository(conn)
    jobs = JobRepository(conn)
    health = SourceHealthRepository(conn)
    audit = AuditService(conn)
    rows = _read_csv(csv_path)
    records = [_row_to_manual_record(row, default_region=default_region) for row in rows]

    product_ids = IngestionService(
        workflow=workflow,
        jobs=jobs,
        health=health,
        audit=audit,
        connectors=(ManualImportConnector(records),),
    ).collect(CollectionRequest(provider="manual", region=default_region))
    detail_service = ProductDetailEnrichmentService(
        details=ProductDetailRepository(conn),
        workflow=workflow,
        audit=audit,
    )
    detail_snapshot_ids: list[str] = []
    for product_id, row in zip(product_ids, rows, strict=False):
        payload = payload_from_csv_row(row)
        if payload is None:
            continue
        detail_snapshot_ids.append(detail_service.enrich_product(product_id, payload).id)

    return {
        "file": str(csv_path),
        "imported_rows": len(records),
        "product_ids": product_ids,
        "detail_snapshot_ids": detail_snapshot_ids,
    }


def run_batch_workflow(
    *,
    conn: sqlite3.Connection,
    storage_root: Path,
    limit: int = 10,
) -> dict[str, Any]:
    Migrator(conn).migrate()
    workflow = WorkflowRepository(conn)
    results = [
        run_product_workflow(
            conn=conn,
            storage_root=storage_root,
            product_id=product.id,
            publishing_metrics=_latest_metrics(conn, product.id),
        )
        for product in workflow.list_products(limit=limit)
    ]
    return {"count": len(results), "results": results}


def run_product_workflow(
    *,
    conn: sqlite3.Connection,
    storage_root: Path,
    product_id: str,
    publishing_metrics: dict[str, Any] | None = None,
) -> dict[str, Any]:
    Migrator(conn).migrate()

    workflow = WorkflowRepository(conn)
    jobs = JobRepository(conn)
    audit = AuditService(conn)
    assets = AssetRepository(conn)
    details = ProductDetailRepository(conn)
    store = LocalAssetStore(storage_root, assets)
    intelligence = ProductIntelligenceRepository(conn)
    scripts = ScriptRepository(conn)
    videos = VideoRepository(conn)
    reviews = ReviewRepository(conn)
    publishing = PublishingRepository(conn)

    product = workflow.get_product(product_id)
    detail = details.get_latest_snapshot(product_id)
    metrics = publishing_metrics or _latest_metrics(conn, product_id)

    score = OpportunityScorer(intelligence).score_product(product_id)
    card = ResearchCardGenerator(
        repo=intelligence,
        workflow=workflow,
        audit=audit,
        extractor=EvidenceExtractor(intelligence),
    ).generate(product_id, score.id)

    script_version_id = ScriptGenerator(
        repo=scripts,
        jobs=jobs,
        audit=audit,
    ).generate(
        ScriptGenerationRequest(
            product_id=product_id,
            research_card_id=card.id,
            duration_seconds=20,
            audience="Douyin shoppers" if product.region == "CN" else "TikTok shoppers",
            tone="energetic",
            count=1,
        )
    )[0]

    video_service = VideoGenerationService(
        repo=videos,
        scripts=scripts,
        assets=assets,
        jobs=jobs,
        audit=audit,
        store=store,
    )
    video_generation_id, scenes = video_service.create_storyboard(script_version_id)
    video_service.plan_assets(video_generation_id, scenes)
    voiceover, subtitles = video_service.generate_voiceover_and_subtitles(
        video_generation_id, scenes
    )
    render = video_service.render(
        generation_id=video_generation_id,
        scenes=scenes,
        voiceover_asset_id=voiceover.id,
        subtitles_asset_id=subtitles.id,
    )

    review_id = reviews.create_review(
        artifact_type="video_generation",
        artifact_id=video_generation_id,
        product_id=product_id,
    )
    reviews.approve(
        review_id=review_id,
        reviewer="demo",
        checklist=ReviewChecklist(
            version=CHECKLIST_VERSION,
            items={item: True for item in CHECKLIST_ITEMS},
            notes={"quality": "Demo approval for local workflow validation"},
        ),
    )
    exporter = ExportService(
        conn=conn,
        reviews=reviews,
        videos=videos,
        scripts=scripts,
        store=store,
    )
    export_package_id = exporter.create_export_package(
        review_id=review_id, video_generation_id=video_generation_id
    )

    channel_id = publishing.create_channel(name="Manual Douyin", channel_type="manual")
    publish_attempt_id = PublishingService(repo=publishing, jobs=jobs).schedule_package(
        export_package_id=export_package_id,
        channel_id=channel_id,
    )
    publishing.update_attempt_status(publish_attempt_id, status="published")
    metrics_snapshot_id, feedback_signal_id = PublishingService(
        repo=publishing, jobs=jobs
    ).import_metrics(
        attempt_id=publish_attempt_id,
        metrics=MetricsSnapshot(
            views=int(metrics.get("views") or 0),
            engagement=int(metrics.get("engagement") or 0),
            clicks=int(metrics.get("clicks") or 0),
            conversions=int(metrics.get("conversions") or 0),
            revenue=float(metrics.get("revenue") or 0.0),
        ),
        source="manual-demo",
    )

    package = exporter.get_export_package(export_package_id)
    script_version = scripts.get_version(script_version_id)

    return {
        "product_id": product_id,
        "title": product.title,
        "product_url": detail.product_url if detail else product.canonical_url,
        "detail_snapshot_id": detail.id if detail else None,
        "detail_completeness": detail.completeness_status if detail else "missing",
        "detail_missing_fields": list(detail.missing_fields)
        if detail
        else list(REQUIRED_RESULT_FIELDS),
        "score_id": score.id,
        "score": score.total_score,
        "score_confidence": score.confidence,
        "research_card_id": card.id,
        "research_status": card.status,
        "script_version_id": script_version_id,
        "script_status": script_version["status"],
        "video_generation_id": video_generation_id,
        "render_asset_id": render.id,
        "render_uri": render.uri,
        "review_id": review_id,
        "export_package_id": export_package_id,
        "export_package_asset_id": package["package_asset_id"],
        "publish_attempt_id": publish_attempt_id,
        "metrics_snapshot_id": metrics_snapshot_id,
        "feedback_signal_id": feedback_signal_id,
        "caption": package["package"]["caption"],
        "hashtags": package["package"]["hashtags"],
    }


REQUIRED_RESULT_FIELDS = (
    "product_url",
    "image_url",
    "price",
    "selling_points",
    "review_summary",
)


def show_results(*, conn: sqlite3.Connection, limit: int = 10) -> dict[str, Any]:
    Migrator(conn).migrate()
    workflow = WorkflowRepository(conn)
    details = ProductDetailRepository(conn)
    products = workflow.list_products(limit=limit)
    results: list[dict[str, Any]] = []
    for product in products:
        detail = details.get_latest_snapshot(product.id)
        score = _latest_score(conn, product.id)
        script = _latest_script(conn, product.id)
        video = _latest_video(conn, product.id)
        package = _latest_export_package(conn, product.id)
        package_payload = package.get("package") if package else {}
        script_payload = script.get("script") if script else {}
        caption = package_payload.get("caption") or " ".join(script_payload.get("captions", ()))
        hashtags = package_payload.get("hashtags") or script_payload.get("hashtags", ())
        results.append(
            {
                "product_id": product.id,
                "title": product.title,
                "region": product.region,
                "category": product.category,
                "workflow_state": product.workflow_state,
                "score": score.get("total_score") if score else None,
                "score_confidence": score.get("confidence") if score else None,
                "detail_snapshot_id": detail.id if detail else None,
                "detail_completeness": detail.completeness_status if detail else "missing",
                "detail_missing_fields": list(detail.missing_fields)
                if detail
                else list(REQUIRED_RESULT_FIELDS),
                "product_url": detail.product_url if detail else product.canonical_url,
                "platform": detail.platform if detail else None,
                "shop_name": detail.shop_name if detail else None,
                "brand": detail.brand if detail else None,
                "price": detail.price if detail else None,
                "currency": detail.currency if detail else None,
                "caption": caption,
                "hashtags": list(hashtags or ()),
                "script_version_id": script.get("id") if script else None,
                "video_generation_id": video.get("id") if video else None,
                "render_uri": video.get("render_uri") if video else None,
                "export_package_id": package.get("id") if package else None,
            }
        )
    return {"count": len(results), "results": results}


def _read_csv(csv_path: Path) -> list[dict[str, str]]:
    with csv_path.open("r", encoding="utf-8-sig", newline="") as handle:
        return list(csv.DictReader(handle))


def _row_to_manual_record(row: dict[str, str], *, default_region: str) -> dict[str, Any]:
    title = (row.get("title") or "").strip()
    if not title:
        raise ValueError("CSV row is missing title")
    source_id = (
        row.get("source_id")
        or row.get("item_id")
        or row.get("goods_id")
        or ""
    ).strip() or title.lower().replace(" ", "-")
    canonical_url = (
        row.get("canonical_url") or row.get("product_url") or row.get("url") or ""
    ).strip() or None
    return {
        "title": title,
        "region": (row.get("region") or default_region).strip() or default_region,
        "category": (row.get("category") or "").strip() or None,
        "canonical_url": canonical_url,
        "source_id": source_id,
        "metadata": {
            "csv_source": True,
            "raw_row": {key: value for key, value in row.items()},
        },
        "metrics": {
            "views": _parse_metric(row.get("views")),
            "sales": _parse_metric(row.get("sales")),
            "engagement": _parse_metric(row.get("engagement")),
            "growth_rate": _parse_percent(row.get("growth_rate")),
            "commission_rate": _parse_percent(row.get("commission_rate")),
            "margin": _parse_metric(row.get("margin")),
            "competitor_count": _parse_metric(row.get("competitor_count")),
        },
    }


def _latest_metrics(conn: sqlite3.Connection, product_id: str) -> dict[str, Any]:
    row = conn.execute(
        """
        SELECT metrics_json
        FROM source_snapshots
        WHERE product_id = ?
        ORDER BY captured_at DESC
        LIMIT 1
        """,
        (product_id,),
    ).fetchone()
    if row is None:
        return {"views": 0, "engagement": 0, "clicks": 0, "conversions": 0, "revenue": 0.0}
    metrics = from_json(row["metrics_json"])
    return {
        "views": int(metrics.get("views") or 0),
        "engagement": int(metrics.get("engagement") or 0),
        "clicks": int(metrics.get("clicks") or 0),
        "conversions": int(metrics.get("conversions") or 0),
        "revenue": float(metrics.get("revenue") or 0.0),
    }


def _latest_score(conn: sqlite3.Connection, product_id: str) -> dict[str, Any] | None:
    row = conn.execute(
        """
        SELECT id, total_score, confidence
        FROM product_scores
        WHERE product_id = ?
        ORDER BY created_at DESC
        LIMIT 1
        """,
        (product_id,),
    ).fetchone()
    if row is None:
        return None
    return {
        "id": row["id"],
        "total_score": float(row["total_score"]),
        "confidence": row["confidence"],
    }


def _latest_script(conn: sqlite3.Connection, product_id: str) -> dict[str, Any] | None:
    row = conn.execute(
        """
        SELECT sv.*
        FROM script_versions sv
        JOIN script_variants v ON v.id = sv.variant_id
        WHERE v.product_id = ?
        ORDER BY sv.created_at DESC
        LIMIT 1
        """,
        (product_id,),
    ).fetchone()
    if row is None:
        return None
    return {
        "id": row["id"],
        "status": row["status"],
        "script": from_json(row["script_json"]),
    }


def _latest_video(conn: sqlite3.Connection, product_id: str) -> dict[str, Any] | None:
    row = conn.execute(
        """
        SELECT vg.*, a.uri AS render_uri
        FROM video_generations vg
        LEFT JOIN assets a ON a.id = vg.output_asset_id
        WHERE vg.product_id = ?
        ORDER BY vg.created_at DESC
        LIMIT 1
        """,
        (product_id,),
    ).fetchone()
    if row is None:
        return None
    return {
        "id": row["id"],
        "status": row["status"],
        "render_uri": row["render_uri"],
    }


def _latest_export_package(conn: sqlite3.Connection, product_id: str) -> dict[str, Any] | None:
    row = conn.execute(
        """
        SELECT *
        FROM export_packages
        WHERE product_id = ?
        ORDER BY created_at DESC
        LIMIT 1
        """,
        (product_id,),
    ).fetchone()
    if row is None:
        return None
    return {"id": row["id"], "package": from_json(row["package_json"])}


def _parse_metric(value: str | None) -> float:
    text = _clean_text(value)
    if not text or text == "-":
        return 0.0
    if "~" in text:
        left, right = text.split("~", 1)
        return (_parse_metric(left) + _parse_metric(right)) / 2
    multiplier = 1.0
    if text.endswith("+"):
        text = text[:-1]
    if text.endswith("w") or text.endswith("万"):
        multiplier = 10000.0
        text = text[:-1]
    return float(text or 0) * multiplier


def _parse_percent(value: str | None) -> float:
    text = _clean_text(value)
    if not text or text == "-":
        return 0.0
    if "蝉选" in text or "公开" in text:
        parts = [
            part
            for part in text.replace("公开", " ").replace("蝉选", " ").split()
            if part
        ]
        values = [_parse_percent(part) for part in parts]
        return max(values) if values else 0.0
    if "~" in text:
        left, right = text.split("~", 1)
        return (_parse_percent(left) + _parse_percent(right)) / 2
    if text.endswith("+"):
        text = text[:-1]
    if text.endswith("%"):
        text = text[:-1]
    numeric = float(text or 0)
    return numeric if numeric <= 1 else numeric / 100.0


def _clean_text(value: str | None) -> str:
    return (value or "").strip().replace(",", "").replace("，", "").lower()
