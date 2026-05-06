from __future__ import annotations

from pathlib import Path
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from tiktok_trend_shop.assets import ExternalAssetRegistry, LocalAssetStore
from tiktok_trend_shop.db import Migrator, connect
from tiktok_trend_shop.repositories import AssetRepository, WorkflowRepository


class AssetTests(unittest.TestCase):
    def setUp(self) -> None:
        self.conn = connect("sqlite:///:memory:")
        Migrator(self.conn).migrate()
        self.assets = AssetRepository(self.conn)
        self.workflow = WorkflowRepository(self.conn)

    def test_local_asset_metadata_and_workflow_reference(self) -> None:
        product = self.workflow.create_product(
            title="Portable blender",
            region="US",
            category="home",
            metadata={"source": "manual"},
        )
        with tempfile.TemporaryDirectory() as temp_dir:
            store = LocalAssetStore(Path(temp_dir), self.assets)
            asset = store.save_bytes(
                kind="product-image",
                filename="product image.png",
                content=b"fake-image",
                content_type="image/png",
                metadata={"product_id": product.id},
            )

        self.assertEqual(asset.backend, "local")
        self.assertEqual(asset.byte_size, len(b"fake-image"))
        self.assertTrue(asset.checksum.startswith("sha256:"))
        self.assertEqual(asset.metadata["product_id"], product.id)

        review_id = self.workflow.create_review_state(
            artifact_type="asset",
            artifact_id=asset.id,
            state="needs_review",
            product_id=product.id,
        )
        self.assertTrue(review_id.startswith("review_"))

    def test_external_asset_registration(self) -> None:
        registry = ExternalAssetRegistry(self.assets)
        asset = registry.register(
            kind="render",
            uri="s3://bucket/render.mp4",
            content_type="video/mp4",
            metadata={"provider": "example"},
        )

        self.assertEqual(asset.backend, "external")
        self.assertEqual(asset.uri, "s3://bucket/render.mp4")


if __name__ == "__main__":
    unittest.main()
