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
from tiktok_trend_shop.ingestion import CollectionRequest, IngestionService, ManualImportConnector
from tiktok_trend_shop.intelligence import EvidenceExtractor, OpportunityScorer, ProductIntelligenceRepository, ResearchCardGenerator
from tiktok_trend_shop.jobs import JobRepository
from tiktok_trend_shop.publishing import MetricsSnapshot, PublishingRepository, PublishingService
from tiktok_trend_shop.repositories import AssetRepository, SourceHealthRepository, WorkflowRepository
from tiktok_trend_shop.review import ApprovalError, CHECKLIST_ITEMS, CHECKLIST_VERSION, ExportService, ReviewChecklist, ReviewRepository
from tiktok_trend_shop.video import VideoGenerationService, VideoRepository


class PublishingTests(unittest.TestCase):
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
        self.reviews = ReviewRepository(self.conn)
        self.publishing = PublishingRepository(self.conn)

        product_id = IngestionService(
            workflow=self.workflow,
            jobs=self.jobs,
            health=self.health,
            audit=self.audit,
            connectors=(ManualImportConnector([{"title": "Mini Fan", "region": "US", "metrics": {"views": 1800}}]),),
        ).collect(CollectionRequest(provider="manual", region="US"))[0]
        score = OpportunityScorer(self.intel).score_product(product_id)
        card = ResearchCardGenerator(
            repo=self.intel,
            workflow=self.workflow,
            audit=self.audit,
            extractor=EvidenceExtractor(self.intel),
        ).generate(product_id, score.id)
        script_version_id = ScriptGenerator(
            repo=self.scripts,
            jobs=self.jobs,
            audit=self.audit,
        ).generate(ScriptGenerationRequest(product_id=product_id, research_card_id=card.id))[0]
        video_service = VideoGenerationService(
            repo=self.videos,
            scripts=self.scripts,
            assets=self.assets,
            jobs=self.jobs,
            audit=self.audit,
            store=self.store,
        )
        generation_id, scenes = video_service.create_storyboard(script_version_id)
        video_service.plan_assets(generation_id, scenes)
        voiceover, subtitles = video_service.generate_voiceover_and_subtitles(generation_id, scenes)
        video_service.render(
            generation_id=generation_id,
            scenes=scenes,
            voiceover_asset_id=voiceover.id,
            subtitles_asset_id=subtitles.id,
        )
        review_id = self.reviews.create_review(
            artifact_type="video_generation",
            artifact_id=generation_id,
            product_id=product_id,
        )
        self.reviews.approve(
            review_id=review_id,
            reviewer="qa",
            checklist=ReviewChecklist(
                version=CHECKLIST_VERSION,
                items={item: True for item in CHECKLIST_ITEMS},
            ),
        )
        self.package_id = ExportService(
            conn=self.conn,
            reviews=self.reviews,
            videos=self.videos,
            scripts=self.scripts,
            store=self.store,
        ).create_export_package(review_id=review_id, video_generation_id=generation_id)
        self.product_id = product_id
        self.script_version_id = script_version_id
        self.generation_id = generation_id
        self.card_id = card.id

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def test_publishing_readiness_gates(self) -> None:
        channel_id = self.publishing.create_channel(name="Manual TikTok", channel_type="manual")
        attempt_id = PublishingService(repo=self.publishing, jobs=self.jobs).schedule_package(
            export_package_id=self.package_id,
            channel_id=channel_id,
            scheduled_at="2026-05-07T10:00:00+00:00",
        )
        attempt = self.publishing.get_attempt(attempt_id)

        self.assertEqual(attempt["status"], "scheduled")
        with self.assertRaises(KeyError):
            PublishingService(repo=self.publishing, jobs=self.jobs).schedule_package(
                export_package_id="export_missing",
                channel_id=channel_id,
            )

    def test_publish_attempt_status_transitions(self) -> None:
        channel_id = self.publishing.create_channel(
            name="Official API",
            channel_type="api",
            provider="tiktok",
        )
        attempt_id = PublishingService(repo=self.publishing, jobs=self.jobs).schedule_package(
            export_package_id=self.package_id,
            channel_id=channel_id,
        )
        updated = PublishingService(repo=self.publishing, jobs=self.jobs).record_provider_status(
            attempt_id=attempt_id,
            provider_status="published",
            response={"id": "tt-1"},
        )
        poll_jobs = self.conn.execute(
            "SELECT count(*) FROM jobs WHERE job_type = 'publish_status_poll'"
        ).fetchone()[0]

        self.assertEqual(updated["status"], "published")
        self.assertEqual(updated["provider_response"]["id"], "tt-1")
        self.assertEqual(poll_jobs, 1)

    def test_metrics_snapshot_ingestion(self) -> None:
        channel_id = self.publishing.create_channel(name="Manual TikTok", channel_type="manual")
        attempt_id = PublishingService(repo=self.publishing, jobs=self.jobs).schedule_package(
            export_package_id=self.package_id,
            channel_id=channel_id,
        )
        self.publishing.update_attempt_status(attempt_id, status="published")

        snapshot_id, _ = PublishingService(repo=self.publishing, jobs=self.jobs).import_metrics(
            attempt_id=attempt_id,
            metrics=MetricsSnapshot(views=500, engagement=30, clicks=5),
            source="manual",
        )
        snapshot = self.publishing.get_metrics_snapshot(snapshot_id)

        self.assertEqual(snapshot["metrics"]["views"], 500)
        self.assertEqual(snapshot["source"], "manual")

    def test_attribution_links_and_feedback_signals(self) -> None:
        channel_id = self.publishing.create_channel(name="Manual TikTok", channel_type="manual")
        attempt_id = PublishingService(repo=self.publishing, jobs=self.jobs).schedule_package(
            export_package_id=self.package_id,
            channel_id=channel_id,
        )
        self.publishing.update_attempt_status(attempt_id, status="published")

        snapshot_id, signal_id = PublishingService(repo=self.publishing, jobs=self.jobs).import_metrics(
            attempt_id=attempt_id,
            metrics=MetricsSnapshot(views=1000, engagement=120, clicks=40, conversions=3, revenue=99),
            source="manual",
        )
        snapshot = self.publishing.get_metrics_snapshot(snapshot_id)
        signal = self.publishing.get_feedback_signal(signal_id)

        self.assertEqual(snapshot["product_id"], self.product_id)
        self.assertEqual(snapshot["script_version_id"], self.script_version_id)
        self.assertEqual(snapshot["video_generation_id"], self.generation_id)
        self.assertEqual(snapshot["research_card_id"], self.card_id)
        self.assertEqual(signal["strength"], "strong")


if __name__ == "__main__":
    unittest.main()
