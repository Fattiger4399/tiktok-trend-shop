from __future__ import annotations

from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from tiktok_trend_shop.audit import AuditService
from tiktok_trend_shop.db import Migrator, connect
from tiktok_trend_shop.ingestion import CollectionRequest, IngestionService, ManualImportConnector
from tiktok_trend_shop.intelligence import (
    EvidenceExtractor,
    OpportunityScorer,
    ProductIntelligenceRepository,
    ResearchCardGenerator,
    ResearchClaim,
    RiskDetector,
)
from tiktok_trend_shop.jobs import JobRepository
from tiktok_trend_shop.repositories import SourceHealthRepository, WorkflowRepository


class IntelligenceTests(unittest.TestCase):
    def setUp(self) -> None:
        self.conn = connect("sqlite:///:memory:")
        Migrator(self.conn).migrate()
        self.workflow = WorkflowRepository(self.conn)
        self.jobs = JobRepository(self.conn)
        self.health = SourceHealthRepository(self.conn)
        self.audit = AuditService(self.conn)
        self.intel = ProductIntelligenceRepository(self.conn)

    def ingest_product(self, raw: dict[str, object]) -> str:
        service = IngestionService(
            workflow=self.workflow,
            jobs=self.jobs,
            health=self.health,
            audit=self.audit,
            connectors=(ManualImportConnector([raw]),),
        )
        return service.collect(CollectionRequest(provider="manual", region="US"))[0]

    def test_score_calculation_and_component_breakdown(self) -> None:
        product_id = self.ingest_product(
            {
                "title": "Portable Blender",
                "region": "US",
                "category": "home",
                "metrics": {
                    "views": 5000,
                    "sales": 80,
                    "growth_rate": 0.42,
                    "commission_rate": 0.18,
                    "margin": 25,
                    "competitor_count": 12,
                },
                "videos": [{"id": "v1", "title": "Demo"}],
            }
        )

        score = OpportunityScorer(self.intel).score_product(product_id)

        self.assertGreater(score.total_score, 0)
        self.assertEqual(score.model_version, "opportunity-v1")
        self.assertIn("demand", score.components)
        self.assertEqual(score.components["growth"].confidence, "medium")

    def test_low_data_confidence_behavior(self) -> None:
        product_id = self.workflow.create_product(title="Sparse Product", region="US").id

        score = OpportunityScorer(self.intel).score_product(product_id)

        self.assertEqual(score.confidence, "low")
        self.assertEqual(score.components["growth"].confidence, "low")

    def test_evidence_backed_research_card_validation(self) -> None:
        product_id = self.ingest_product(
            {
                "title": "LED Mirror",
                "region": "US",
                "category": "beauty",
                "metrics": {"views": 1500},
            }
        )
        score = OpportunityScorer(self.intel).score_product(product_id)
        generator = ResearchCardGenerator(
            repo=self.intel,
            workflow=self.workflow,
            audit=self.audit,
            extractor=EvidenceExtractor(self.intel),
        )

        card = generator.generate(product_id, score.id)

        self.assertEqual(card.status, "ready")
        self.assertTrue(card.selling_points[0].supported)
        self.assertTrue(card.selling_points[0].evidence_refs)
        product = self.workflow.get_product(product_id)
        self.assertEqual(product.workflow_state, "research_ready")

    def test_risky_claim_detection(self) -> None:
        flags = RiskDetector().detect(
            category="health",
            claims=(
                ResearchClaim(
                    text="This miracle item can cure discomfort",
                    evidence_refs=(),
                    supported=False,
                ),
            ),
            texts=(),
        )

        categories = {flag["category"] for flag in flags}
        self.assertIn("policy_sensitive_category", categories)
        self.assertIn("unsupported_claim", categories)
        self.assertIn("medical_claim", categories)


if __name__ == "__main__":
    unittest.main()
