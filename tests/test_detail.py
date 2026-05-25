from __future__ import annotations

from contextlib import redirect_stdout
from io import StringIO
from pathlib import Path
import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from tiktok_trend_shop.audit import AuditService
from tiktok_trend_shop.cli import main
from tiktok_trend_shop.db import Migrator, connect
from tiktok_trend_shop.detail import (
    ProductDetailEnrichmentService,
    ProductDetailPayload,
    ProductDetailRepository,
)
from tiktok_trend_shop.ingestion import CollectionRequest, IngestionService, ManualImportConnector
from tiktok_trend_shop.intelligence import (
    EvidenceExtractor,
    OpportunityScorer,
    ProductIntelligenceRepository,
    ResearchCardGenerator,
)
from tiktok_trend_shop.jobs import JobRepository
from tiktok_trend_shop.pipeline import import_products_csv
from tiktok_trend_shop.repositories import SourceHealthRepository, WorkflowRepository


class ProductDetailTests(unittest.TestCase):
    def setUp(self) -> None:
        self.conn = connect("sqlite:///:memory:")
        Migrator(self.conn).migrate()
        self.workflow = WorkflowRepository(self.conn)
        self.audit = AuditService(self.conn)
        self.details = ProductDetailRepository(self.conn)

    def test_detail_snapshot_storage_and_latest_lookup(self) -> None:
        product = self.workflow.create_product(title="Wet Noodles", region="CN", category="food")
        service = ProductDetailEnrichmentService(
            details=self.details,
            workflow=self.workflow,
            audit=self.audit,
        )

        first = service.enrich_product(
            product.id,
            ProductDetailPayload(
                provider="manual",
                platform="taobao",
                product_url="https://item.example/1",
                image_url="https://img.example/1.jpg",
                price=19.9,
                selling_points=("Ready to eat",),
                review_summary="Buyers mention convenient packaging.",
            ),
        )
        second = service.enrich_product(
            product.id,
            ProductDetailPayload(
                provider="manual",
                platform="taobao",
                product_url="https://item.example/1",
                price=18.8,
                selling_points=("Updated price",),
            ),
        )

        latest = self.details.get_latest_snapshot(product.id)
        self.assertEqual(latest.id, second.id)
        self.assertEqual(first.completeness_status, "complete")
        self.assertEqual(second.completeness_status, "incomplete")
        self.assertIn("image_url", second.missing_fields)

    def test_csv_detail_enrichment_and_completeness(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            csv_path = Path(temp_dir) / "products.csv"
            csv_path.write_text(
                "\n".join(
                    [
                        "title,region,category,product_url,platform,shop_name,brand,price,image_url,selling_points,review_summary,views",
                        "Wet Noodles,CN,food,https://item.example/1,taobao,Demo Shop,Demo Brand,19.9,https://img.example/1.jpg,Ready to eat|Family pack,Convenient for quick meals,10000",
                    ]
                ),
                encoding="utf-8",
            )

            imported = import_products_csv(conn=self.conn, csv_path=csv_path, default_region="CN")
            detail = self.details.get_latest_snapshot(imported["product_ids"][0])

        self.assertEqual(len(imported["detail_snapshot_ids"]), 1)
        self.assertEqual(detail.completeness_status, "complete")
        self.assertEqual(detail.shop_name, "Demo Shop")
        self.assertEqual(detail.selling_points, ("Ready to eat", "Family pack"))

    def test_research_card_uses_detail_evidence_and_scans_risk(self) -> None:
        jobs = JobRepository(self.conn)
        health = SourceHealthRepository(self.conn)
        product_id = IngestionService(
            workflow=self.workflow,
            jobs=jobs,
            health=health,
            audit=self.audit,
            connectors=(
                ManualImportConnector(
                    [
                        {
                            "title": "Green Mud Cleanser",
                            "region": "CN",
                            "category": "cosmetics",
                            "metrics": {"views": 12000},
                        }
                    ]
                ),
            ),
        ).collect(CollectionRequest(provider="manual", region="CN"))[0]
        ProductDetailEnrichmentService(
            details=self.details,
            workflow=self.workflow,
            audit=self.audit,
        ).enrich_product(
            product_id,
            ProductDetailPayload(
                provider="manual",
                platform="taobao",
                product_url="https://item.example/cleanser",
                image_url="https://img.example/cleanser.jpg",
                price=39.9,
                selling_points=("Guaranteed acne cure",),
                review_summary="Some buyers mention oil-control usage.",
                specs={"volume": "120g"},
            ),
        )
        intel = ProductIntelligenceRepository(self.conn)
        score = OpportunityScorer(intel).score_product(product_id)

        card = ResearchCardGenerator(
            repo=intel,
            workflow=self.workflow,
            audit=self.audit,
            extractor=EvidenceExtractor(intel),
        ).generate(product_id, score.id)

        self.assertEqual(card.selling_points[0].text, "Guaranteed acne cure")
        self.assertEqual(card.status, "review_required")
        self.assertTrue(any(flag["category"] == "medical_claim" for flag in card.risk_flags))

    def test_show_results_cli_outputs_detail_and_exports(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            old_env = dict(os.environ)
            csv_path = Path(temp_dir) / "products.csv"
            csv_path.write_text(
                "\n".join(
                    [
                        "title,region,category,product_url,platform,price,image_url,selling_points,review_summary,views,sales",
                        "Desk Lamp,CN,home,https://item.example/lamp,taobao,29.9,https://img.example/lamp.jpg,Soft light,Looks good on desks,10000,200",
                    ]
                ),
                encoding="utf-8",
            )
            try:
                os.environ["TTS_DATABASE_URL"] = f"sqlite:///{Path(temp_dir) / 'demo.sqlite3'}"
                os.environ["TTS_STORAGE_ROOT"] = str(Path(temp_dir) / "assets")
                with redirect_stdout(StringIO()):
                    main(["import-csv", "--file", str(csv_path), "--region", "CN"])
                    main(["run-batch", "--limit", "1"])
                output = StringIO()
                with redirect_stdout(output):
                    exit_code = main(["show-results", "--limit", "1"])
                payload = json.loads(output.getvalue())
            finally:
                os.environ.clear()
                os.environ.update(old_env)

        self.assertEqual(exit_code, 0)
        self.assertEqual(payload["count"], 1)
        result = payload["results"][0]
        self.assertEqual(result["detail_completeness"], "complete")
        self.assertEqual(result["product_url"], "https://item.example/lamp")
        self.assertTrue(result["export_package_id"].startswith("export_"))


if __name__ == "__main__":
    unittest.main()
