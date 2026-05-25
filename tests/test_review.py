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
from tiktok_trend_shop.repositories import AssetRepository, SourceHealthRepository, WorkflowRepository
from tiktok_trend_shop.review import ApprovalError, CHECKLIST_ITEMS, CHECKLIST_VERSION, ExportService, ReviewChecklist, ReviewRepository
from tiktok_trend_shop.video import VideoGenerationService, VideoRepository


class ReviewTests(unittest.TestCase):
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

        product_id = IngestionService(
            workflow=self.workflow,
            jobs=self.jobs,
            health=self.health,
            audit=self.audit,
            connectors=(ManualImportConnector([{"title": "Desk Lamp", "region": "US", "metrics": {"views": 1000}}]),),
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
        self.product_id = product_id
        self.generation_id = generation_id
        self.script_version_id = script_version_id

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def checklist(self) -> ReviewChecklist:
        return ReviewChecklist(
            version=CHECKLIST_VERSION,
            items={item: True for item in CHECKLIST_ITEMS},
            notes={"quality": "Looks ready"},
        )

    def test_review_state_transitions(self) -> None:
        review_id = self.reviews.create_review(
            artifact_type="video_generation",
            artifact_id=self.generation_id,
            product_id=self.product_id,
        )
        self.assertEqual(len(self.reviews.list_queue()), 1)

        feedback_id = self.reviews.request_changes(
            review_id=review_id,
            route_to_artifact_type="script_version",
            route_to_artifact_id=self.script_version_id,
            reason="wording issue",
            comment="Tighten the hook",
        )

        review = self.reviews.get_review(review_id)
        self.assertTrue(feedback_id.startswith("feedback_"))
        self.assertEqual(review["state"], "changes_requested")

    def test_blocked_export_before_approval(self) -> None:
        review_id = self.reviews.create_review(
            artifact_type="video_generation",
            artifact_id=self.generation_id,
            product_id=self.product_id,
        )
        exporter = ExportService(
            conn=self.conn,
            reviews=self.reviews,
            videos=self.videos,
            scripts=self.scripts,
            store=self.store,
        )

        with self.assertRaisesRegex(ApprovalError, "approved"):
            exporter.create_export_package(
                review_id=review_id,
                video_generation_id=self.generation_id,
            )

    def test_checklist_approval_metadata(self) -> None:
        review_id = self.reviews.create_review(
            artifact_type="video_generation",
            artifact_id=self.generation_id,
            product_id=self.product_id,
        )
        approved = self.reviews.approve(
            review_id=review_id,
            reviewer="qa",
            checklist=self.checklist(),
            notes="approved",
        )

        self.assertEqual(approved["state"], "approved")
        self.assertEqual(approved["checklist"]["version"], CHECKLIST_VERSION)
        self.assertTrue(approved["checklist"]["items"]["claim_support"])

    def test_export_package_contents_and_version_references(self) -> None:
        review_id = self.reviews.create_review(
            artifact_type="video_generation",
            artifact_id=self.generation_id,
            product_id=self.product_id,
        )
        self.reviews.approve(review_id=review_id, reviewer="qa", checklist=self.checklist())
        exporter = ExportService(
            conn=self.conn,
            reviews=self.reviews,
            videos=self.videos,
            scripts=self.scripts,
            store=self.store,
        )

        package_id = exporter.create_export_package(
            review_id=review_id,
            video_generation_id=self.generation_id,
        )
        package = exporter.get_export_package(package_id)
        review = self.reviews.get_review(review_id)

        self.assertEqual(package["script_version_id"], self.script_version_id)
        self.assertEqual(package["package"]["video_generation_id"], self.generation_id)
        self.assertTrue(package["package"]["hashtags"])
        self.assertEqual(review["state"], "exported")


if __name__ == "__main__":
    unittest.main()
