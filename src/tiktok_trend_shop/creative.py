from __future__ import annotations

from dataclasses import dataclass, field
import sqlite3
from typing import Any

from .audit import AuditService
from .jobs import JobRepository
from .repositories import from_json, new_id, to_json, utc_now


SCRIPT_PROMPT_VERSION = "script-agent-v1"
SCRIPT_MODEL = "structured-local"
BANNED_CLAIM_TERMS = ("cure", "guaranteed", "miracle", "risk free profit")


@dataclass(frozen=True)
class SceneBeat:
    start_second: float
    end_second: float
    visual: str
    voiceover: str
    overlay_text: str


@dataclass(frozen=True)
class ScriptClaim:
    text: str
    evidence_refs: tuple[str, ...]


@dataclass(frozen=True)
class SellingScript:
    hook: str
    scene_beats: tuple[SceneBeat, ...]
    voiceover: str
    captions: tuple[str, ...]
    hashtags: tuple[str, ...]
    cta: str
    claims: tuple[ScriptClaim, ...]
    compliance_notes: tuple[str, ...] = ()
    language: str = "en"
    estimated_duration_seconds: int = 20


@dataclass(frozen=True)
class ScriptGenerationRequest:
    product_id: str
    research_card_id: str
    duration_seconds: int = 20
    audience: str = "TikTok shoppers"
    language: str = "en"
    tone: str = "energetic"
    hook_style: str = "problem-solution"
    count: int = 1


@dataclass(frozen=True)
class ValidationResult:
    ready: bool
    errors: tuple[str, ...] = ()
    warnings: tuple[str, ...] = ()


class ScriptRepository:
    def __init__(self, conn: sqlite3.Connection) -> None:
        self.conn = conn

    def create_variant(
        self,
        *,
        product_id: str,
        research_card_id: str,
        params: dict[str, Any],
        prompt_version: str = SCRIPT_PROMPT_VERSION,
        model: str = SCRIPT_MODEL,
        status: str = "draft",
    ) -> str:
        variant_id = new_id("scriptvar")
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                INSERT INTO script_variants(
                    id, product_id, research_card_id, prompt_version, model,
                    params_json, status, created_at, updated_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    variant_id,
                    product_id,
                    research_card_id,
                    prompt_version,
                    model,
                    to_json(params),
                    status,
                    now,
                    now,
                ),
            )
        return variant_id

    def create_version(
        self,
        *,
        variant_id: str,
        script: SellingScript,
        validation: ValidationResult,
        parent_version_id: str | None = None,
    ) -> str:
        version_id = new_id("scriptver")
        version_number = self.next_version_number(variant_id)
        now = utc_now()
        status = "ready" if validation.ready else "blocked"
        with self.conn:
            self.conn.execute(
                """
                INSERT INTO script_versions(
                    id, variant_id, version_number, parent_version_id, status,
                    script_json, validation_json, created_at, updated_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    version_id,
                    variant_id,
                    version_number,
                    parent_version_id,
                    status,
                    to_json(script_to_json(script)),
                    to_json(validation_to_json(validation)),
                    now,
                    now,
                ),
            )
            self.conn.execute(
                "UPDATE script_variants SET status = ?, updated_at = ? WHERE id = ?",
                (status, now, variant_id),
            )
        return version_id

    def next_version_number(self, variant_id: str) -> int:
        row = self.conn.execute(
            "SELECT max(version_number) AS max_version FROM script_versions WHERE variant_id = ?",
            (variant_id,),
        ).fetchone()
        max_version = row["max_version"] if row else None
        return int(max_version or 0) + 1

    def revise_version(
        self,
        *,
        parent_version_id: str,
        script: SellingScript,
        validation: ValidationResult,
    ) -> str:
        parent = self.get_version(parent_version_id)
        return self.create_version(
            variant_id=parent["variant_id"],
            script=script,
            validation=validation,
            parent_version_id=parent_version_id,
        )

    def get_version(self, version_id: str) -> dict[str, Any]:
        row = self.conn.execute(
            "SELECT * FROM script_versions WHERE id = ?", (version_id,)
        ).fetchone()
        if row is None:
            raise KeyError(f"Script version not found: {version_id}")
        return {
            "id": row["id"],
            "variant_id": row["variant_id"],
            "version_number": row["version_number"],
            "parent_version_id": row["parent_version_id"],
            "status": row["status"],
            "script": from_json(row["script_json"]),
            "validation": from_json(row["validation_json"]),
            "created_at": row["created_at"],
            "updated_at": row["updated_at"],
        }

    def get_research_card(self, research_card_id: str) -> dict[str, Any]:
        row = self.conn.execute(
            "SELECT * FROM research_cards WHERE id = ?", (research_card_id,)
        ).fetchone()
        if row is None:
            raise KeyError(f"Research card not found: {research_card_id}")
        return {
            "id": row["id"],
            "product_id": row["product_id"],
            "status": row["status"],
            "card": from_json(row["card_json"]),
            "evidence": from_json(row["evidence_json"]),
            "risk_flags": from_json(row["risk_flags_json"]),
        }


class ScriptValidator:
    def validate(self, script: SellingScript, research_card: dict[str, Any]) -> ValidationResult:
        errors: list[str] = []
        warnings: list[str] = []
        if not script.hook:
            errors.append("hook is required")
        if not script.scene_beats:
            errors.append("at least one scene beat is required")
        if not script.cta:
            errors.append("cta is required")
        if script.estimated_duration_seconds <= 0:
            errors.append("estimated duration must be positive")
        word_count = len(script.voiceover.split())
        estimated_max_words = max(1, script.estimated_duration_seconds * 3)
        if word_count > estimated_max_words:
            errors.append("voiceover exceeds target duration")

        approved_evidence = {
            item["id"]
            for item in research_card.get("evidence", {}).get("items", [])
        }
        approved_claim_texts = {
            claim["text"]
            for claim in research_card.get("card", {}).get("selling_points", [])
        } | {
            claim["text"]
            for claim in research_card.get("card", {}).get("proof_points", [])
        }
        for claim in script.claims:
            if not claim.evidence_refs or not set(claim.evidence_refs).issubset(approved_evidence):
                errors.append(f"unsupported claim: {claim.text}")
            if claim.text not in approved_claim_texts:
                warnings.append(f"claim text is not an exact research-card claim: {claim.text}")
            lowered = claim.text.lower()
            for term in BANNED_CLAIM_TERMS:
                if term in lowered:
                    errors.append(f"banned claim term: {term}")
        return ValidationResult(ready=not errors, errors=tuple(errors), warnings=tuple(warnings))


class ScriptGenerator:
    def __init__(
        self,
        *,
        repo: ScriptRepository,
        jobs: JobRepository,
        audit: AuditService,
        validator: ScriptValidator | None = None,
    ) -> None:
        self.repo = repo
        self.jobs = jobs
        self.audit = audit
        self.validator = validator or ScriptValidator()

    def create_generation_job(self, request: ScriptGenerationRequest) -> str:
        job = self.jobs.create_job(
            job_type="script_generation",
            input_ref_type="research_card",
            input_ref_id=request.research_card_id,
            payload=request.__dict__,
            max_retries=1,
        )
        return job.id

    def generate(self, request: ScriptGenerationRequest) -> list[str]:
        research_card = self.repo.get_research_card(request.research_card_id)
        version_ids: list[str] = []
        for index in range(request.count):
            variant_id = self.repo.create_variant(
                product_id=request.product_id,
                research_card_id=request.research_card_id,
                params=request.__dict__ | {"variant_index": index},
            )
            script = build_script_from_card(research_card, request, variant_index=index)
            validation = self.validator.validate(script, research_card)
            version_id = self.repo.create_version(
                variant_id=variant_id,
                script=script,
                validation=validation,
            )
            self.audit.record_generated_output(
                subject_type="script_version",
                subject_id=version_id,
                provider="local",
                prompt_version=SCRIPT_PROMPT_VERSION,
                model=SCRIPT_MODEL,
                metadata={
                    "research_card_id": request.research_card_id,
                    "ready": validation.ready,
                },
            )
            version_ids.append(version_id)
        return version_ids


def build_script_from_card(
    research_card: dict[str, Any], request: ScriptGenerationRequest, *, variant_index: int = 0
) -> SellingScript:
    card = research_card["card"]
    selling_points = card.get("selling_points", [])
    proof_points = card.get("proof_points", [])
    primary_claim = selling_points[0] if selling_points else {"text": "This product is worth testing", "evidence_refs": []}
    proof_claim = proof_points[0] if proof_points else primary_claim
    benefit = primary_claim["text"]
    hook = (
        f"Stop scrolling if you want a faster {request.audience.lower()} find."
        if request.hook_style == "problem-solution"
        else f"Here is a TikTok find worth checking today."
    )
    midpoint = max(4, request.duration_seconds // 2)
    beats = (
        SceneBeat(0, 3, "Show product close-up", hook, "TikTok find"),
        SceneBeat(3, midpoint, "Show product in use", benefit, "Why it matters"),
        SceneBeat(
            midpoint,
            request.duration_seconds,
            "Show proof and CTA",
            "Check the details before it sells out.",
            "Tap to compare",
        ),
    )
    voiceover = " ".join(beat.voiceover for beat in beats)
    if "douyin" in request.audience.lower():
        hashtags = ("#抖音好物", "#好物分享", f"#{request.tone.lower()}好物")
    else:
        hashtags = ("#tiktokshop", "#finds", f"#{request.tone.lower()}finds")
    return SellingScript(
        hook=hook,
        scene_beats=beats,
        voiceover=voiceover,
        captions=(hook, benefit, "Check the product details."),
        hashtags=hashtags,
        cta="Check the product details and compare today.",
        claims=(
            ScriptClaim(
                text=primary_claim["text"],
                evidence_refs=tuple(primary_claim.get("evidence_refs", ())),
            ),
            ScriptClaim(
                text=proof_claim["text"],
                evidence_refs=tuple(proof_claim.get("evidence_refs", ())),
            ),
        ),
        compliance_notes=("Claims must stay tied to research-card evidence.",),
        language=request.language,
        estimated_duration_seconds=request.duration_seconds,
    )


def repair_script_payload(payload: dict[str, Any]) -> dict[str, Any]:
    repaired = dict(payload)
    repaired.setdefault("hook", "Here is a product worth checking.")
    repaired.setdefault("scene_beats", [])
    repaired.setdefault("voiceover", repaired["hook"])
    repaired.setdefault("captions", [repaired["hook"]])
    repaired.setdefault("hashtags", ["#tiktokshop"])
    repaired.setdefault("cta", "Check the product details.")
    repaired.setdefault("claims", [])
    repaired.setdefault("compliance_notes", [])
    repaired.setdefault("language", "en")
    repaired.setdefault("estimated_duration_seconds", 20)
    return repaired


def script_to_json(script: SellingScript) -> dict[str, Any]:
    return {
        "hook": script.hook,
        "scene_beats": [beat.__dict__ for beat in script.scene_beats],
        "voiceover": script.voiceover,
        "captions": list(script.captions),
        "hashtags": list(script.hashtags),
        "cta": script.cta,
        "claims": [claim.__dict__ for claim in script.claims],
        "compliance_notes": list(script.compliance_notes),
        "language": script.language,
        "estimated_duration_seconds": script.estimated_duration_seconds,
    }


def validation_to_json(validation: ValidationResult) -> dict[str, Any]:
    return {
        "ready": validation.ready,
        "errors": list(validation.errors),
        "warnings": list(validation.warnings),
    }
