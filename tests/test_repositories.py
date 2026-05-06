from __future__ import annotations

from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from tiktok_trend_shop.audit import AuditService
from tiktok_trend_shop.db import Migrator, connect
from tiktok_trend_shop.repositories import WorkflowRepository


class RepositoryTests(unittest.TestCase):
    def setUp(self) -> None:
        self.conn = connect("sqlite:///:memory:")
        Migrator(self.conn).migrate()
        self.workflow = WorkflowRepository(self.conn)
        self.audit = AuditService(self.conn)

    def test_product_snapshot_analytics_and_audit_records(self) -> None:
        product = self.workflow.create_product(
            title="LED vanity mirror",
            region="US",
            category="beauty",
            canonical_url="https://example.test/product/1",
        )
        updated = self.workflow.update_product_state(product.id, "research_ready")
        self.assertEqual(updated.workflow_state, "research_ready")
        self.assertEqual(updated.created_at, product.created_at)

        snapshot_id = self.workflow.add_source_snapshot(
            product_id=product.id,
            provider="manual",
            source_type="product",
            source_url=product.canonical_url,
            metrics={"views": 1000},
        )
        analytics_id = self.workflow.create_analytics_ref(
            product_id=product.id,
            channel="manual-tiktok",
            metrics={"views": 200},
        )
        event = self.audit.record_external_input(
            subject_type="source_snapshot",
            subject_id=snapshot_id,
            provider="manual",
            source_url=product.canonical_url,
            metadata={"analytics_id": analytics_id},
        )

        self.assertTrue(snapshot_id.startswith("snap_"))
        self.assertTrue(analytics_id.startswith("metric_"))
        self.assertEqual(event.provider, "manual")
        self.assertEqual(event.metadata["analytics_id"], analytics_id)


if __name__ == "__main__":
    unittest.main()
