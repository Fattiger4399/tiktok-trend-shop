from __future__ import annotations

from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from tiktok_trend_shop.audit import AuditService
from tiktok_trend_shop.config import ProviderConfig
from tiktok_trend_shop.db import Migrator, connect
from tiktok_trend_shop.ingestion import (
    CollectionRequest,
    IngestionService,
    ManualImportConnector,
    RateLimitError,
    SourceProviderRegistry,
    TrendSourceConnector,
)
from tiktok_trend_shop.jobs import JobRepository
from tiktok_trend_shop.repositories import SourceHealthRepository, WorkflowRepository


class FailingConnector:
    name = "broken"

    def collect(self, request: CollectionRequest):
        raise RateLimitError("rate limited", retry_after="2026-05-07T00:00:00+00:00")


class IngestionTests(unittest.TestCase):
    def setUp(self) -> None:
        self.conn = connect("sqlite:///:memory:")
        Migrator(self.conn).migrate()
        self.workflow = WorkflowRepository(self.conn)
        self.jobs = JobRepository(self.conn)
        self.health = SourceHealthRepository(self.conn)
        self.audit = AuditService(self.conn)

    def test_connector_configuration_validation(self) -> None:
        registry = SourceProviderRegistry(
            (
                ProviderConfig(kind="source", name="manual", enabled=True),
                ProviderConfig(kind="source", name="paid", enabled=False),
            )
        )

        self.assertEqual(registry.validate_enabled("manual").name, "manual")
        with self.assertRaisesRegex(ValueError, "disabled"):
            registry.validate_enabled("paid")
        with self.assertRaisesRegex(ValueError, "Unknown source provider"):
            registry.validate_enabled("missing")

    def test_manual_import_normalizes_snapshots_and_entities(self) -> None:
        connector = ManualImportConnector(
            [
                {
                    "title": "Portable Blender",
                    "region": "US",
                    "category": "home",
                    "canonical_url": "https://shop.example/blender",
                    "source_id": "sku-1",
                    "metrics": {"views": 1200, "sales": 30},
                    "videos": [
                        {
                            "id": "vid-1",
                            "title": "Smoothie demo",
                            "url": "https://tiktok.example/v/1",
                            "metrics": {"likes": 50},
                        }
                    ],
                    "creators": [{"id": "creator-1", "handle": "@demo"}],
                    "hashtags": [{"name": "#kitchenfinds"}],
                }
            ]
        )
        service = IngestionService(
            workflow=self.workflow,
            jobs=self.jobs,
            health=self.health,
            audit=self.audit,
            connectors=(connector,),
        )

        ids = service.collect(CollectionRequest(provider="manual", region="US"))

        self.assertEqual(len(ids), 1)
        product = self.workflow.get_product(ids[0])
        self.assertEqual(product.title, "Portable Blender")
        entity_count = self.conn.execute("SELECT count(*) FROM normalized_entities").fetchone()[0]
        snapshot_count = self.conn.execute("SELECT count(*) FROM source_snapshots").fetchone()[0]
        audit_count = self.conn.execute("SELECT count(*) FROM audit_events").fetchone()[0]
        self.assertEqual(entity_count, 3)
        self.assertEqual(snapshot_count, 1)
        self.assertEqual(audit_count, 1)

    def test_product_deduplication_across_repeated_imports(self) -> None:
        raw = {
            "title": "LED Mirror",
            "region": "US",
            "canonical_url": "https://shop.example/mirror",
            "source_id": "mirror-1",
        }
        service = IngestionService(
            workflow=self.workflow,
            jobs=self.jobs,
            health=self.health,
            audit=self.audit,
            connectors=(ManualImportConnector([raw]),),
        )

        first = service.collect(CollectionRequest(provider="manual", region="US"))
        second = service.collect(CollectionRequest(provider="manual", region="US"))

        self.assertEqual(first, second)
        product_count = self.conn.execute("SELECT count(*) FROM products").fetchone()[0]
        snapshot_count = self.conn.execute("SELECT count(*) FROM source_snapshots").fetchone()[0]
        self.assertEqual(product_count, 1)
        self.assertEqual(snapshot_count, 2)

    def test_failed_connector_health_state(self) -> None:
        service = IngestionService(
            workflow=self.workflow,
            jobs=self.jobs,
            health=self.health,
            audit=self.audit,
            connectors=(FailingConnector(),),
        )

        with self.assertRaises(RateLimitError):
            service.collect(CollectionRequest(provider="broken", region="US"))

        state = self.health.get_provider_state("broken")
        self.assertIsNotNone(state)
        self.assertEqual(state["status"], "rate_limited")
        self.assertEqual(state["last_error"], "rate limited")


if __name__ == "__main__":
    unittest.main()
