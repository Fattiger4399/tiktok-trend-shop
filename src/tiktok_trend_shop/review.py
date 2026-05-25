from __future__ import annotations

from dataclasses import dataclass
import sqlite3
from typing import Any

from .assets import LocalAssetStore
from .creative import ScriptRepository
from .repositories import from_json, new_id, to_json, utc_now
from .video import VideoRepository


REVIEW_STATES = {
    "draft",
    "needs_review",
    "changes_requested",
    "approved",
    "rejected",
    "exported",
}
REVIEWABLE_ARTIFACTS = {
    "research_card",
    "script_version",
    "storyboard",
    "video_generation",
}
CHECKLIST_VERSION = "review-checklist-v1"
CHECKLIST_ITEMS = (
    "claim_support",
    "policy_sensitive_category",
    "copyright",
    "ai_disclosure",
    "quality",
)


class ApprovalError(RuntimeError):
    """Raised when an export or publishing gate fails."""


@dataclass(frozen=True)
class ReviewChecklist:
    version: str
    items: dict[str, bool]
    notes: dict[str, str] | None = None

    def validate_for_approval(self) -> None:
        missing = [item for item in CHECKLIST_ITEMS if not self.items.get(item)]
        if missing:
            raise ApprovalError(f"Checklist items missing approval: {', '.join(missing)}")


class ReviewRepository:
    def __init__(self, conn: sqlite3.Connection) -> None:
        self.conn = conn

    def create_review(
        self,
        *,
        artifact_type: str,
        artifact_id: str,
        product_id: str | None,
        owner: str | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> str:
        if artifact_type not in REVIEWABLE_ARTIFACTS:
            raise ValueError(f"Unsupported review artifact type: {artifact_type}")
        review_id = new_id("review")
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                INSERT INTO review_states(
                    id, artifact_type, artifact_id, product_id, state, reviewer,
                    notes, checklist_json, created_at, updated_at
                )
                VALUES (?, ?, ?, ?, 'needs_review', ?, ?, '{}', ?, ?)
                """,
                (
                    review_id,
                    artifact_type,
                    artifact_id,
                    product_id,
                    owner,
                    to_json(metadata),
                    now,
                    now,
                ),
            )
        return review_id

    def list_queue(self, *, state: str = "needs_review") -> list[dict[str, Any]]:
        rows = self.conn.execute(
            "SELECT * FROM review_states WHERE state = ? ORDER BY created_at",
            (state,),
        ).fetchall()
        return [self._from_row(row) for row in rows]

    def get_review(self, review_id: str) -> dict[str, Any]:
        row = self.conn.execute("SELECT * FROM review_states WHERE id = ?", (review_id,)).fetchone()
        if row is None:
            raise KeyError(f"Review not found: {review_id}")
        return self._from_row(row)

    def approve(
        self,
        *,
        review_id: str,
        reviewer: str,
        checklist: ReviewChecklist,
        notes: str | None = None,
    ) -> dict[str, Any]:
        checklist.validate_for_approval()
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                UPDATE review_states
                SET state = 'approved', reviewer = ?, notes = ?, checklist_json = ?,
                    updated_at = ?
                WHERE id = ?
                """,
                (
                    reviewer,
                    notes,
                    to_json(
                        {
                            "version": checklist.version,
                            "items": checklist.items,
                            "notes": checklist.notes or {},
                        }
                    ),
                    now,
                    review_id,
                ),
            )
        return self.get_review(review_id)

    def request_changes(
        self,
        *,
        review_id: str,
        route_to_artifact_type: str,
        route_to_artifact_id: str,
        reason: str,
        comment: str | None = None,
    ) -> str:
        feedback_id = new_id("feedback")
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                UPDATE review_states
                SET state = 'changes_requested', notes = ?, updated_at = ?
                WHERE id = ?
                """,
                (reason, now, review_id),
            )
            self.conn.execute(
                """
                INSERT INTO review_feedback(
                    id, review_id, route_to_artifact_type, route_to_artifact_id,
                    reason, comment, created_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    feedback_id,
                    review_id,
                    route_to_artifact_type,
                    route_to_artifact_id,
                    reason,
                    comment,
                    now,
                ),
            )
        return feedback_id

    def mark_exported(self, review_id: str) -> None:
        with self.conn:
            self.conn.execute(
                "UPDATE review_states SET state = 'exported', updated_at = ? WHERE id = ?",
                (utc_now(), review_id),
            )

    def _from_row(self, row: sqlite3.Row) -> dict[str, Any]:
        return {
            "id": row["id"],
            "artifact_type": row["artifact_type"],
            "artifact_id": row["artifact_id"],
            "product_id": row["product_id"],
            "state": row["state"],
            "reviewer": row["reviewer"],
            "notes": row["notes"],
            "checklist": from_json(row["checklist_json"]),
            "created_at": row["created_at"],
            "updated_at": row["updated_at"],
        }


class ExportService:
    def __init__(
        self,
        *,
        conn: sqlite3.Connection,
        reviews: ReviewRepository,
        videos: VideoRepository,
        scripts: ScriptRepository,
        store: LocalAssetStore,
    ) -> None:
        self.conn = conn
        self.reviews = reviews
        self.videos = videos
        self.scripts = scripts
        self.store = store

    def create_export_package(self, *, review_id: str, video_generation_id: str) -> str:
        review = self.reviews.get_review(review_id)
        if review["state"] != "approved":
            raise ApprovalError("Video must be approved before export")
        if review["artifact_type"] != "video_generation":
            raise ApprovalError("Review must belong to a video generation")
        if review["artifact_id"] != video_generation_id:
            raise ApprovalError("Review artifact does not match requested video generation")
        generation = self.videos.get_generation(video_generation_id)
        if generation["status"] != "rendered" or not generation["output_asset_id"]:
            raise ApprovalError("Video generation must be rendered before export")
        script_version = self.scripts.get_version(generation["script_version_id"])
        script = script_version["script"]
        package = {
            "video_generation_id": video_generation_id,
            "video_asset_id": generation["output_asset_id"],
            "script_version_id": generation["script_version_id"],
            "caption": " ".join(script.get("captions", ())),
            "hashtags": script.get("hashtags", ()),
            "product_id": generation["product_id"],
            "review_id": review_id,
            "metadata": {
                "checklist_version": review["checklist"].get("version"),
                "script_version_number": script_version["version_number"],
                "validation": generation["validation"],
            },
        }
        asset = self.store.save_bytes(
            kind="export-package",
            filename=f"{video_generation_id}-export.json",
            content=to_json(package).encode("utf-8"),
            content_type="application/json",
            metadata={
                "video_generation_id": video_generation_id,
                "review_id": review_id,
                "product_id": generation["product_id"],
            },
        )
        package_id = new_id("export")
        with self.conn:
            self.conn.execute(
                """
                INSERT INTO export_packages(
                    id, product_id, review_id, video_generation_id, script_version_id,
                    package_asset_id, package_json, created_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    package_id,
                    generation["product_id"],
                    review_id,
                    video_generation_id,
                    generation["script_version_id"],
                    asset.id,
                    to_json(package),
                    utc_now(),
                ),
            )
        self.reviews.mark_exported(review_id)
        return package_id

    def get_export_package(self, package_id: str) -> dict[str, Any]:
        row = self.conn.execute(
            "SELECT * FROM export_packages WHERE id = ?", (package_id,)
        ).fetchone()
        if row is None:
            raise KeyError(f"Export package not found: {package_id}")
        return {
            "id": row["id"],
            "product_id": row["product_id"],
            "review_id": row["review_id"],
            "video_generation_id": row["video_generation_id"],
            "script_version_id": row["script_version_id"],
            "package_asset_id": row["package_asset_id"],
            "package": from_json(row["package_json"]),
        }
