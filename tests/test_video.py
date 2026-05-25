from __future__ import annotations

from pathlib import Path
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from tiktok_trend_shop.assets import LocalAssetStore
from tiktok_trend_shop.audit import AuditService
from tiktok_trend_shop.creative import ScriptGenerationRequest, ScriptGenerator, ScriptRepository
from tiktok_trend_shop.db import Migrator, connect
from tiktok_trend_shop.domain.models import AssetRecord
from tiktok_trend_shop.ingestion import CollectionRequest, IngestionService, ManualImportConnector
from tiktok_trend_shop.intelligence import EvidenceExtractor, OpportunityScorer, ProductIntelligenceRepository, ResearchCardGenerator
from tiktok_trend_shop.jobs import JobRepository
from tiktok_trend_shop.repositories import AssetRepository, SourceHealthRepository, WorkflowRepository
from tiktok_trend_shop.video import VideoGenerationService, VideoRepository, validate_render_asset


class VideoTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        self.conn = connect("sqlite:///:memory:")
        Migrator(self.conn).migrate()
        self.workflow = WorkflowRepository(self.conn)
        self.jobs = JobRepository(self.conn)
        self.health = SourceHealthRepository(self.conn)
        self.audit = AuditService(self.conn)
        self.assets = AssetRepository(self.conn)
        self.store = LocalAssetStore(Path(self.temp_dir.name), self.assets)
        self.intel = ProductIntelligenceRepository(self.conn)
        self.scripts = ScriptRepository(self.conn)
        self.videos = VideoRepository(self.conn)

        product_id = IngestionService(
            workflow=self.workflow,
            jobs=self.jobs,
            health=self.health,
            audit=self.audit,
            connectors=(ManualImportConnector([{"title": "LED Mirror", "region": "US", "metrics": {"views": 2000}}]),),
        ).collect(CollectionRequest(provider="manual", region="US"))[0]
        score = OpportunityScorer(self.intel).score_product(product_id)
        card = ResearchCardGenerator(
            repo=self.intel,
            workflow=self.workflow,
            audit=self.audit,
            extractor=EvidenceExtractor(self.intel),
        ).generate(product_id, score.id)
        self.version_id = ScriptGenerator(
            repo=self.scripts,
            jobs=self.jobs,
            audit=self.audit,
        ).generate(ScriptGenerationRequest(product_id=product_id, research_card_id=card.id))[0]
        self.product_id = product_id

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def service(self) -> VideoGenerationService:
        return VideoGenerationService(
            repo=self.videos,
            scripts=self.scripts,
            assets=self.assets,
            jobs=self.jobs,
            audit=self.audit,
            store=self.store,
        )

    def test_storyboard_creation_from_script_input(self) -> None:
        generation_id, scenes = self.service().create_storyboard(self.version_id)

        self.assertTrue(generation_id.startswith("video_"))
        self.assertEqual(len(scenes), 3)
        self.assertEqual(scenes[0].index, 0)

    def test_asset_provenance_metadata(self) -> None:
        source_asset = self.store.save_bytes(
            kind="product-image",
            filename="mirror.png",
            content=b"image",
            content_type="image/png",
            metadata={"product_id": self.product_id},
        )
        generation_id, scenes = self.service().create_storyboard(self.version_id)
        plans = self.service().plan_assets(generation_id, scenes)

        self.assertEqual(plans[0].asset_id, source_asset.id)
        self.assertEqual(plans[0].origin, "source_product_asset")

    def test_render_validation_rules(self) -> None:
        generation_id, scenes = self.service().create_storyboard(self.version_id)
        self.service().plan_assets(generation_id, scenes)
        voiceover, subtitles = self.service().generate_voiceover_and_subtitles(generation_id, scenes)
        output = self.service().render(
            generation_id=generation_id,
            scenes=scenes,
            voiceover_asset_id=voiceover.id,
            subtitles_asset_id=subtitles.id,
        )
        generation = self.videos.get_generation(generation_id)

        self.assertEqual(output.content_type, "video/mp4")
        self.assertEqual(generation["status"], "rendered")
        self.assertTrue(generation["validation"]["ready"])

        invalid = AssetRecord(
            id="asset_bad",
            kind="render",
            backend="local",
            uri="bad.mp4",
            content_type="video/mp4",
            byte_size=1,
            checksum=None,
            metadata={"aspect_ratio": "1:1"},
            created_at="now",
        )
        self.assertFalse(validate_render_asset(invalid).ready)

    def test_retrying_failed_pipeline_stage(self) -> None:
        generation_id, _ = self.service().create_storyboard(self.version_id)
        stage_id = self.videos.create_stage(generation_id=generation_id, stage="voiceover", status="queued")

        failed = self.videos.fail_stage(stage_id, "TTS failed")
        retry = self.videos.retry_stage(stage_id)

        self.assertEqual(failed["status"], "failed")
        self.assertEqual(failed["retry_count"], 1)
        self.assertEqual(retry["status"], "queued")
        self.assertIsNone(retry["last_error"])


if __name__ == "__main__":
    unittest.main()
