from __future__ import annotations

from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from tiktok_trend_shop.audit import AuditService
from tiktok_trend_shop.creative import (
    SceneBeat,
    ScriptClaim,
    ScriptGenerationRequest,
    ScriptGenerator,
    ScriptRepository,
    ScriptValidator,
    SellingScript,
    ValidationResult,
    repair_script_payload,
)
from tiktok_trend_shop.db import Migrator, connect
from tiktok_trend_shop.ingestion import CollectionRequest, IngestionService, ManualImportConnector
from tiktok_trend_shop.intelligence import EvidenceExtractor, OpportunityScorer, ProductIntelligenceRepository, ResearchCardGenerator
from tiktok_trend_shop.jobs import JobRepository
from tiktok_trend_shop.repositories import SourceHealthRepository, WorkflowRepository


class CreativeTests(unittest.TestCase):
    def setUp(self) -> None:
        self.conn = connect("sqlite:///:memory:")
        Migrator(self.conn).migrate()
        self.workflow = WorkflowRepository(self.conn)
        self.jobs = JobRepository(self.conn)
        self.health = SourceHealthRepository(self.conn)
        self.audit = AuditService(self.conn)
        self.intel = ProductIntelligenceRepository(self.conn)
        self.scripts = ScriptRepository(self.conn)

        service = IngestionService(
            workflow=self.workflow,
            jobs=self.jobs,
            health=self.health,
            audit=self.audit,
            connectors=(
                ManualImportConnector(
                    [
                        {
                            "title": "Portable Blender",
                            "region": "US",
                            "category": "home",
                            "metrics": {"views": 5000, "sales": 50},
                        }
                    ]
                ),
            ),
        )
        self.product_id = service.collect(CollectionRequest(provider="manual", region="US"))[0]
        score = OpportunityScorer(self.intel).score_product(self.product_id)
        card = ResearchCardGenerator(
            repo=self.intel,
            workflow=self.workflow,
            audit=self.audit,
            extractor=EvidenceExtractor(self.intel),
        ).generate(self.product_id, score.id)
        self.card_id = card.id

    def test_script_schema_validation(self) -> None:
        repaired = repair_script_payload({})

        self.assertEqual(repaired["hook"], "Here is a product worth checking.")
        self.assertEqual(repaired["cta"], "Check the product details.")

    def test_creative_parameter_handling(self) -> None:
        generator = ScriptGenerator(repo=self.scripts, jobs=self.jobs, audit=self.audit)
        version_ids = generator.generate(
            ScriptGenerationRequest(
                product_id=self.product_id,
                research_card_id=self.card_id,
                duration_seconds=20,
                tone="energetic",
                hook_style="problem-solution",
                count=2,
            )
        )

        self.assertEqual(len(version_ids), 2)
        version = self.scripts.get_version(version_ids[0])
        self.assertEqual(version["status"], "ready")
        self.assertEqual(version["script"]["estimated_duration_seconds"], 20)
        self.assertIn("#energeticfinds", version["script"]["hashtags"])

    def test_unsupported_claim_rejection(self) -> None:
        research_card = self.scripts.get_research_card(self.card_id)
        script = SellingScript(
            hook="Try this",
            scene_beats=(SceneBeat(0, 5, "show", "Try this", "Try this"),),
            voiceover="Try this",
            captions=("Try this",),
            hashtags=("#tiktokshop",),
            cta="Check it out",
            claims=(ScriptClaim("Unsupported miracle claim", ()),),
        )

        validation = ScriptValidator().validate(script, research_card)

        self.assertFalse(validation.ready)
        self.assertTrue(any("unsupported claim" in error for error in validation.errors))

    def test_script_version_history(self) -> None:
        generator = ScriptGenerator(repo=self.scripts, jobs=self.jobs, audit=self.audit)
        version_id = generator.generate(
            ScriptGenerationRequest(product_id=self.product_id, research_card_id=self.card_id)
        )[0]
        original = self.scripts.get_version(version_id)
        script_data = original["script"]
        script_data["hook"] = "Updated hook"
        script = SellingScript(
            hook=script_data["hook"],
            scene_beats=tuple(SceneBeat(**beat) for beat in script_data["scene_beats"]),
            voiceover=script_data["voiceover"],
            captions=tuple(script_data["captions"]),
            hashtags=tuple(script_data["hashtags"]),
            cta=script_data["cta"],
            claims=tuple(ScriptClaim(**claim) for claim in script_data["claims"]),
            compliance_notes=tuple(script_data["compliance_notes"]),
            language=script_data["language"],
            estimated_duration_seconds=script_data["estimated_duration_seconds"],
        )
        revised_id = self.scripts.revise_version(
            parent_version_id=version_id,
            script=script,
            validation=ValidationResult(ready=True),
        )
        revised = self.scripts.get_version(revised_id)

        self.assertEqual(revised["version_number"], 2)
        self.assertEqual(revised["parent_version_id"], version_id)
        self.assertEqual(revised["script"]["hook"], "Updated hook")


if __name__ == "__main__":
    unittest.main()
