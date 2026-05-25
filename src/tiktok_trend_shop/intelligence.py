from __future__ import annotations

from dataclasses import dataclass, field
import sqlite3
from typing import Any

from .audit import AuditService
from .detail import ProductDetailRepository, product_detail_texts
from .repositories import WorkflowRepository, from_json, new_id, to_json, utc_now


SCORE_MODEL_VERSION = "opportunity-v1"
DEFAULT_WEIGHTS = {
    "growth": 0.25,
    "demand": 0.25,
    "competition": 0.15,
    "economics": 0.15,
    "content_potential": 0.10,
    "risk": 0.10,
}
RISKY_CATEGORIES = {
    "medical",
    "health",
    "supplements",
    "finance",
    "weight-loss",
    "cosmetics",
    "food",
    "children",
    "kids",
    "oral-care",
    "personal-care",
}
RISKY_TERMS = {
    "cure": "medical_claim",
    "treat": "medical_claim",
    "guaranteed": "exaggerated_claim",
    "miracle": "exaggerated_claim",
    "lose weight": "weight_loss_claim",
    "before and after": "sensitive_creative",
    "risk free profit": "financial_claim",
    "治疗": "medical_claim",
    "治愈": "medical_claim",
    "根治": "medical_claim",
    "减肥": "weight_loss_claim",
    "瘦身": "weight_loss_claim",
    "祛痘": "restricted_efficacy_claim",
    "美白": "restricted_efficacy_claim",
    "防蛀": "restricted_efficacy_claim",
    "抗糖": "restricted_efficacy_claim",
    "无效退款": "exaggerated_claim",
    "保证": "exaggerated_claim",
    "儿童": "children_claim",
    "食品安全": "food_safety_claim",
}


@dataclass(frozen=True)
class ScoreComponent:
    name: str
    value: float
    confidence: str
    evidence_refs: tuple[str, ...] = ()


@dataclass(frozen=True)
class OpportunityScore:
    id: str
    product_id: str
    model_version: str
    total_score: float
    confidence: str
    components: dict[str, ScoreComponent]
    evidence: dict[str, Any]
    created_at: str


@dataclass(frozen=True)
class EvidenceItem:
    id: str
    kind: str
    text: str
    source_ref: str
    confidence: str = "medium"


@dataclass(frozen=True)
class ResearchClaim:
    text: str
    evidence_refs: tuple[str, ...]
    supported: bool


@dataclass(frozen=True)
class ProductResearchCard:
    id: str
    product_id: str
    score_id: str | None
    prompt_version: str
    model: str
    status: str
    target_audience: tuple[str, ...]
    selling_points: tuple[ResearchClaim, ...]
    pain_points: tuple[str, ...]
    objections: tuple[str, ...]
    proof_points: tuple[ResearchClaim, ...]
    creative_angles: tuple[str, ...]
    recommended_positioning: str
    detail_warnings: tuple[str, ...] = ()
    risk_flags: tuple[dict[str, str], ...] = ()


class ProductIntelligenceRepository:
    def __init__(self, conn: sqlite3.Connection) -> None:
        self.conn = conn

    def list_source_snapshots(self, product_id: str) -> list[dict[str, Any]]:
        rows = self.conn.execute(
            "SELECT * FROM source_snapshots WHERE product_id = ? ORDER BY captured_at",
            (product_id,),
        ).fetchall()
        return [
            {
                "id": row["id"],
                "provider": row["provider"],
                "source_type": row["source_type"],
                "source_id": row["source_id"],
                "source_url": row["source_url"],
                "captured_at": row["captured_at"],
                "metrics": from_json(row["metrics_json"]),
                "metadata": from_json(row["metadata_json"]),
            }
            for row in rows
        ]

    def list_entities(self, product_id: str) -> list[dict[str, Any]]:
        rows = self.conn.execute(
            "SELECT * FROM normalized_entities WHERE product_id = ? ORDER BY created_at",
            (product_id,),
        ).fetchall()
        return [
            {
                "id": row["id"],
                "entity_type": row["entity_type"],
                "provider": row["provider"],
                "source_id": row["source_id"],
                "source_url": row["source_url"],
                "name": row["name"],
                "metrics": from_json(row["metrics_json"]),
                "metadata": from_json(row["metadata_json"]),
            }
            for row in rows
        ]

    def save_score(
        self,
        *,
        product_id: str,
        total_score: float,
        confidence: str,
        components: dict[str, ScoreComponent],
        evidence: dict[str, Any],
        model_version: str = SCORE_MODEL_VERSION,
    ) -> OpportunityScore:
        score_id = new_id("score")
        now = utc_now()
        serialized_components = {
            name: {
                "value": component.value,
                "confidence": component.confidence,
                "evidence_refs": component.evidence_refs,
            }
            for name, component in components.items()
        }
        with self.conn:
            self.conn.execute(
                """
                INSERT INTO product_scores(
                    id, product_id, model_version, total_score, confidence,
                    components_json, evidence_json, created_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    score_id,
                    product_id,
                    model_version,
                    total_score,
                    confidence,
                    to_json(serialized_components),
                    to_json(evidence),
                    now,
                ),
            )
        return self.get_score(score_id)

    def get_score(self, score_id: str) -> OpportunityScore:
        row = self.conn.execute("SELECT * FROM product_scores WHERE id = ?", (score_id,)).fetchone()
        if row is None:
            raise KeyError(f"Score not found: {score_id}")
        raw_components = from_json(row["components_json"])
        components = {
            name: ScoreComponent(
                name=name,
                value=float(data["value"]),
                confidence=data["confidence"],
                evidence_refs=tuple(data.get("evidence_refs", ())),
            )
            for name, data in raw_components.items()
        }
        return OpportunityScore(
            id=row["id"],
            product_id=row["product_id"],
            model_version=row["model_version"],
            total_score=float(row["total_score"]),
            confidence=row["confidence"],
            components=components,
            evidence=from_json(row["evidence_json"]),
            created_at=row["created_at"],
        )

    def save_research_card(
        self,
        *,
        card: ProductResearchCard,
        evidence: tuple[EvidenceItem, ...],
    ) -> ProductResearchCard:
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                INSERT INTO research_cards(
                    id, product_id, score_id, prompt_version, model, status,
                    card_json, evidence_json, risk_flags_json, created_at, updated_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    card.id,
                    card.product_id,
                    card.score_id,
                    card.prompt_version,
                    card.model,
                    card.status,
                    to_json(_card_to_json(card)),
                    to_json({"items": [item.__dict__ for item in evidence]}),
                    to_json({"items": list(card.risk_flags)}),
                    now,
                    now,
                ),
            )
        return card

    def add_risk_flag(
        self,
        *,
        product_id: str,
        artifact_type: str,
        category: str,
        severity: str,
        message: str,
        artifact_id: str | None = None,
    ) -> str:
        flag_id = new_id("risk")
        with self.conn:
            self.conn.execute(
                """
                INSERT INTO risk_flags(
                    id, product_id, artifact_type, artifact_id, category,
                    severity, message, created_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    flag_id,
                    product_id,
                    artifact_type,
                    artifact_id,
                    category,
                    severity,
                    message,
                    utc_now(),
                ),
            )
        return flag_id


class OpportunityScorer:
    def __init__(self, repo: ProductIntelligenceRepository) -> None:
        self.repo = repo

    def score_product(self, product_id: str) -> OpportunityScore:
        snapshots = self.repo.list_source_snapshots(product_id)
        entities = self.repo.list_entities(product_id)
        metrics = _merge_metrics(snapshots)
        evidence = {"snapshot_ids": [snapshot["id"] for snapshot in snapshots]}
        demand_raw = (
            metrics.get("views", 0) + metrics.get("sales", 0) * 40
            if "views" in metrics or "sales" in metrics
            else None
        )
        economics_raw = (
            metrics.get("commission_rate", 0) * 100 + metrics.get("margin", 0)
            if "commission_rate" in metrics or "margin" in metrics
            else None
        )
        components = {
            "growth": _component("growth", metrics.get("growth_rate"), "growth_rate", snapshots),
            "demand": _component("demand", demand_raw, "views/sales", snapshots),
            "competition": _component(
                "competition",
                100 - min(float(metrics["competitor_count"]), 100)
                if "competitor_count" in metrics
                else None,
                "competitor_count",
                snapshots,
                missing_default=50,
            ),
            "economics": _component(
                "economics",
                economics_raw,
                "commission/margin",
                snapshots,
            ),
            "content_potential": ScoreComponent(
                name="content_potential",
                value=min(100.0, len(entities) * 18.0),
                confidence="high" if entities else "low",
                evidence_refs=tuple(entity["id"] for entity in entities),
            ),
            "risk": ScoreComponent(
                name="risk",
                value=90.0,
                confidence="medium",
                evidence_refs=(),
            ),
        }
        weighted = sum(
            min(max(component.value, 0.0), 100.0) * DEFAULT_WEIGHTS[name]
            for name, component in components.items()
        )
        confidence = _overall_confidence(components)
        return self.repo.save_score(
            product_id=product_id,
            total_score=round(weighted, 2),
            confidence=confidence,
            components=components,
            evidence=evidence,
        )


class EvidenceExtractor:
    def __init__(self, repo: ProductIntelligenceRepository) -> None:
        self.repo = repo

    def extract(self, product_id: str) -> tuple[EvidenceItem, ...]:
        snapshots = self.repo.list_source_snapshots(product_id)
        entities = self.repo.list_entities(product_id)
        detail = ProductDetailRepository(self.repo.conn).get_latest_snapshot(product_id)
        evidence: list[EvidenceItem] = []
        for snapshot in snapshots:
            metrics = snapshot["metrics"]
            if metrics:
                evidence.append(
                    EvidenceItem(
                        id=f"evidence:{snapshot['id']}",
                        kind="metric",
                        text=f"Collected metrics: {_format_metrics(metrics)}",
                        source_ref=snapshot["id"],
                        confidence="high",
                    )
                )
        for entity in entities:
            evidence.append(
                EvidenceItem(
                    id=f"evidence:{entity['id']}",
                    kind=entity["entity_type"],
                    text=f"{entity['entity_type']} signal: {entity.get('name') or entity.get('source_id')}",
                    source_ref=entity["id"],
                    confidence="medium",
                )
            )
        if detail:
            if detail.product_url:
                evidence.append(
                    EvidenceItem(
                        id=f"evidence:{detail.id}:product_url",
                        kind="product_detail",
                        text=f"Product URL: {detail.product_url}",
                        source_ref=detail.id,
                        confidence="high",
                    )
                )
            if detail.price is not None:
                currency = detail.currency or ""
                evidence.append(
                    EvidenceItem(
                        id=f"evidence:{detail.id}:price",
                        kind="product_detail",
                        text=f"Price: {currency} {detail.price}".strip(),
                        source_ref=detail.id,
                        confidence="high",
                    )
                )
            if detail.shop_name:
                evidence.append(
                    EvidenceItem(
                        id=f"evidence:{detail.id}:shop",
                        kind="product_detail",
                        text=f"Shop: {detail.shop_name}",
                        source_ref=detail.id,
                        confidence="medium",
                    )
                )
            if detail.brand:
                evidence.append(
                    EvidenceItem(
                        id=f"evidence:{detail.id}:brand",
                        kind="product_detail",
                        text=f"Brand: {detail.brand}",
                        source_ref=detail.id,
                        confidence="medium",
                    )
                )
            for index, point in enumerate(detail.selling_points):
                evidence.append(
                    EvidenceItem(
                        id=f"evidence:{detail.id}:selling_point:{index}",
                        kind="selling_point",
                        text=point,
                        source_ref=detail.id,
                        confidence="high",
                    )
                )
            for key, value in detail.specs.items():
                evidence.append(
                    EvidenceItem(
                        id=f"evidence:{detail.id}:spec:{key}",
                        kind="spec",
                        text=f"{key}: {value}",
                        source_ref=detail.id,
                        confidence="medium",
                    )
                )
            if detail.review_summary:
                evidence.append(
                    EvidenceItem(
                        id=f"evidence:{detail.id}:review_summary",
                        kind="review",
                        text=detail.review_summary,
                        source_ref=detail.id,
                        confidence="medium",
                    )
                )
            for index, highlight in enumerate(detail.review_highlights):
                evidence.append(
                    EvidenceItem(
                        id=f"evidence:{detail.id}:review_highlight:{index}",
                        kind="review",
                        text=highlight,
                        source_ref=detail.id,
                        confidence="medium",
                    )
                )
        return tuple(evidence)


class RiskDetector:
    def detect(
        self, *, category: str | None, claims: tuple[ResearchClaim, ...], texts: tuple[str, ...]
    ) -> tuple[dict[str, str], ...]:
        flags: list[dict[str, str]] = []
        if category and category.lower() in RISKY_CATEGORIES:
            flags.append(
                {
                    "category": "policy_sensitive_category",
                    "severity": "high",
                    "message": f"Category requires review: {category}",
                }
            )
        for claim in claims:
            if not claim.supported:
                flags.append(
                    {
                        "category": "unsupported_claim",
                        "severity": "high",
                        "message": claim.text,
                    }
                )
        searchable = " ".join(tuple(claim.text for claim in claims) + texts).lower()
        for term, category_name in RISKY_TERMS.items():
            if term in searchable:
                flags.append(
                    {
                        "category": category_name,
                        "severity": "high",
                        "message": f"Risky claim term detected: {term}",
                    }
                )
        return tuple(flags)


class ResearchCardGenerator:
    def __init__(
        self,
        *,
        repo: ProductIntelligenceRepository,
        workflow: WorkflowRepository,
        audit: AuditService,
        extractor: EvidenceExtractor,
        risk_detector: RiskDetector | None = None,
        prompt_version: str = "research-card-v1",
        model: str = "structured-local",
    ) -> None:
        self.repo = repo
        self.workflow = workflow
        self.audit = audit
        self.extractor = extractor
        self.risk_detector = risk_detector or RiskDetector()
        self.prompt_version = prompt_version
        self.model = model

    def generate(self, product_id: str, score_id: str | None = None) -> ProductResearchCard:
        product = self.workflow.get_product(product_id)
        evidence = self.extractor.extract(product_id)
        evidence_ids = tuple(item.id for item in evidence)
        has_evidence = bool(evidence_ids)
        detail = ProductDetailRepository(self.repo.conn).get_latest_snapshot(product_id)
        detail_evidence = tuple(item for item in evidence if detail and item.source_ref == detail.id)
        selling_evidence = tuple(item for item in detail_evidence if item.kind == "selling_point")
        review_evidence = tuple(item for item in detail_evidence if item.kind == "review")
        selling_claims = tuple(
            ResearchClaim(
                text=item.text,
                evidence_refs=(item.id,),
                supported=True,
            )
            for item in selling_evidence[:3]
        )
        if not selling_claims:
            selling_claims = (
                ResearchClaim(
                    text=f"{product.title} has measurable demand signals from collected trend data.",
                    evidence_refs=evidence_ids[:2],
                    supported=has_evidence,
                ),
            )
        if review_evidence:
            proof_claim = ResearchClaim(
                text=f"Review signal: {review_evidence[0].text}",
                evidence_refs=(review_evidence[0].id,),
                supported=True,
            )
        else:
            proof_claim = ResearchClaim(
                text="Trend metrics and creative references support testing this product.",
                evidence_refs=evidence_ids,
                supported=has_evidence,
            )
        detail_warnings = (
            tuple(f"Missing product detail field: {field}" for field in detail.missing_fields)
            if detail and detail.missing_fields
            else ()
        )
        risk_texts = (
            product.title,
            product.metadata.get("description", "") if product.metadata else "",
            *product_detail_texts(detail),
        )
        risk_flags = self.risk_detector.detect(
            category=product.category,
            claims=(*selling_claims, proof_claim),
            texts=tuple(str(text) for text in risk_texts if text),
        )
        status = "review_required" if risk_flags else "ready"
        card = ProductResearchCard(
            id=new_id("card"),
            product_id=product_id,
            score_id=score_id,
            prompt_version=self.prompt_version,
            model=self.model,
            status=status,
            target_audience=("trend shoppers", "TikTok impulse buyers"),
            selling_points=selling_claims,
            pain_points=("Need a quick way to understand why the product matters",),
            objections=("May need proof that the product quality matches the claim",),
            proof_points=(proof_claim,),
            creative_angles=("Problem-solution demo", "Fast product benefit countdown"),
            recommended_positioning=f"Position {product.title} as a practical TikTok discovery with visible proof.",
            detail_warnings=detail_warnings,
            risk_flags=risk_flags,
        )
        self.repo.save_research_card(card=card, evidence=evidence)
        for flag in risk_flags:
            self.repo.add_risk_flag(
                product_id=product_id,
                artifact_type="research_card",
                artifact_id=card.id,
                category=flag["category"],
                severity=flag["severity"],
                message=flag["message"],
            )
        self.audit.record_generated_output(
            subject_type="research_card",
            subject_id=card.id,
            provider="local",
            prompt_version=self.prompt_version,
            model=self.model,
            metadata={"evidence_refs": evidence_ids, "status": status},
        )
        if risk_flags:
            self.workflow.update_product_state(product_id, "review_required")
        elif not has_evidence:
            self.workflow.update_product_state(product_id, "low_confidence")
        else:
            self.workflow.update_product_state(product_id, "research_ready")
        return card


def _component(
    name: str,
    raw_value: Any,
    evidence_label: str,
    snapshots: list[dict[str, Any]],
    *,
    missing_default: float | None = None,
) -> ScoreComponent:
    has_value = raw_value is not None
    value = float(raw_value if has_value else missing_default or 0.0)
    if name == "growth":
        value = min(100.0, max(0.0, value * 100.0))
    elif name == "demand":
        value = min(100.0, value / 100.0)
    else:
        value = min(100.0, max(0.0, value))
    return ScoreComponent(
        name=name,
        value=round(value, 2),
        confidence="medium" if has_value else "low",
        evidence_refs=tuple(snapshot["id"] for snapshot in snapshots if evidence_label),
    )


def _overall_confidence(components: dict[str, ScoreComponent]) -> str:
    low_count = sum(1 for component in components.values() if component.confidence == "low")
    if low_count >= 3:
        return "low"
    if low_count:
        return "medium"
    return "high"


def _merge_metrics(snapshots: list[dict[str, Any]]) -> dict[str, float]:
    merged: dict[str, float] = {}
    for snapshot in snapshots:
        for key, value in snapshot["metrics"].items():
            if isinstance(value, (int, float)):
                merged[key] = max(float(value), merged.get(key, 0.0))
    return merged


def _format_metrics(metrics: dict[str, Any]) -> str:
    return ", ".join(f"{key}={value}" for key, value in sorted(metrics.items()))


def _card_to_json(card: ProductResearchCard) -> dict[str, Any]:
    return {
        "target_audience": list(card.target_audience),
        "selling_points": [claim.__dict__ for claim in card.selling_points],
        "pain_points": list(card.pain_points),
        "objections": list(card.objections),
        "proof_points": [claim.__dict__ for claim in card.proof_points],
        "creative_angles": list(card.creative_angles),
        "recommended_positioning": card.recommended_positioning,
        "detail_warnings": list(card.detail_warnings),
    }
