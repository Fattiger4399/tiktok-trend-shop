from __future__ import annotations

from dataclasses import dataclass
import sqlite3
from typing import Any, Protocol

from .jobs import JobRepository
from .repositories import from_json, new_id, to_json, utc_now
from .review import ApprovalError


PUBLISH_STATUSES = {"scheduled", "submitted", "published", "failed", "canceled"}


class PublishingProvider(Protocol):
    name: str

    def submit(self, package: dict[str, Any]) -> dict[str, Any]:
        ...

    def poll_status(self, provider_response: dict[str, Any]) -> dict[str, Any]:
        ...


@dataclass(frozen=True)
class MetricsSnapshot:
    views: int = 0
    engagement: int = 0
    clicks: int = 0
    conversions: int = 0
    revenue: float = 0.0

    def as_dict(self) -> dict[str, Any]:
        return {
            "views": self.views,
            "engagement": self.engagement,
            "clicks": self.clicks,
            "conversions": self.conversions,
            "revenue": self.revenue,
        }


class PublishingRepository:
    def __init__(self, conn: sqlite3.Connection) -> None:
        self.conn = conn

    def create_channel(
        self,
        *,
        name: str,
        channel_type: str = "manual",
        provider: str = "manual",
        account_ref: str | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> str:
        channel_id = new_id("channel")
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                INSERT INTO publishing_channels(
                    id, name, channel_type, provider, account_ref, metadata_json,
                    created_at, updated_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    channel_id,
                    name,
                    channel_type,
                    provider,
                    account_ref,
                    to_json(metadata),
                    now,
                    now,
                ),
            )
        return channel_id

    def get_channel(self, channel_id: str) -> dict[str, Any]:
        row = self.conn.execute(
            "SELECT * FROM publishing_channels WHERE id = ?", (channel_id,)
        ).fetchone()
        if row is None:
            raise KeyError(f"Publishing channel not found: {channel_id}")
        return {
            "id": row["id"],
            "name": row["name"],
            "channel_type": row["channel_type"],
            "provider": row["provider"],
            "account_ref": row["account_ref"],
            "enabled": bool(row["enabled"]),
            "metadata": from_json(row["metadata_json"]),
        }

    def get_export_package(self, package_id: str) -> dict[str, Any]:
        row = self.conn.execute(
            """
            SELECT ep.*, rs.state AS review_state
            FROM export_packages ep
            JOIN review_states rs ON rs.id = ep.review_id
            WHERE ep.id = ?
            """,
            (package_id,),
        ).fetchone()
        if row is None:
            raise KeyError(f"Export package not found: {package_id}")
        return {
            "id": row["id"],
            "product_id": row["product_id"],
            "review_id": row["review_id"],
            "review_state": row["review_state"],
            "video_generation_id": row["video_generation_id"],
            "script_version_id": row["script_version_id"],
            "package_asset_id": row["package_asset_id"],
            "package": from_json(row["package_json"]),
        }

    def create_attempt(
        self,
        *,
        export_package_id: str,
        channel_id: str,
        scheduled_at: str | None = None,
        status: str = "scheduled",
    ) -> str:
        if status not in PUBLISH_STATUSES:
            raise ValueError(f"Invalid publish status: {status}")
        attempt_id = new_id("pub")
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                INSERT INTO publish_attempts(
                    id, export_package_id, channel_id, scheduled_at, status,
                    created_at, updated_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?)
                """,
                (attempt_id, export_package_id, channel_id, scheduled_at, status, now, now),
            )
        return attempt_id

    def update_attempt_status(
        self,
        attempt_id: str,
        *,
        status: str,
        provider_status: str | None = None,
        provider_response: dict[str, Any] | None = None,
        error: str | None = None,
    ) -> dict[str, Any]:
        if status not in PUBLISH_STATUSES:
            raise ValueError(f"Invalid publish status: {status}")
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                UPDATE publish_attempts
                SET status = ?, provider_status = ?, provider_response_json = ?,
                    error = ?, retry_count = retry_count + CASE WHEN ? IS NULL THEN 0 ELSE 1 END,
                    updated_at = ?
                WHERE id = ?
                """,
                (
                    status,
                    provider_status,
                    to_json(provider_response),
                    error,
                    error,
                    now,
                    attempt_id,
                ),
            )
        return self.get_attempt(attempt_id)

    def get_attempt(self, attempt_id: str) -> dict[str, Any]:
        row = self.conn.execute(
            "SELECT * FROM publish_attempts WHERE id = ?", (attempt_id,)
        ).fetchone()
        if row is None:
            raise KeyError(f"Publish attempt not found: {attempt_id}")
        return {
            "id": row["id"],
            "export_package_id": row["export_package_id"],
            "channel_id": row["channel_id"],
            "scheduled_at": row["scheduled_at"],
            "status": row["status"],
            "provider_status": row["provider_status"],
            "provider_response": from_json(row["provider_response_json"]),
            "error": row["error"],
            "retry_count": row["retry_count"],
        }

    def create_metrics_snapshot(
        self,
        *,
        attempt_id: str,
        metrics: MetricsSnapshot,
        source: str,
        collected_at: str | None = None,
    ) -> str:
        attempt = self.get_attempt(attempt_id)
        if attempt["status"] != "published":
            raise ApprovalError("Performance metrics require a published attempt")
        package = self.get_export_package(attempt["export_package_id"])
        research_card_id = self._research_card_for_script(package["script_version_id"])
        snapshot_id = new_id("perf")
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                INSERT INTO performance_metric_snapshots(
                    id, publish_attempt_id, export_package_id, channel_id, product_id,
                    video_generation_id, script_version_id, research_card_id, source,
                    metrics_json, collected_at, created_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    snapshot_id,
                    attempt_id,
                    package["id"],
                    attempt["channel_id"],
                    package["product_id"],
                    package["video_generation_id"],
                    package["script_version_id"],
                    research_card_id,
                    source,
                    to_json(metrics.as_dict()),
                    collected_at or now,
                    now,
                ),
            )
        return snapshot_id

    def get_metrics_snapshot(self, snapshot_id: str) -> dict[str, Any]:
        row = self.conn.execute(
            "SELECT * FROM performance_metric_snapshots WHERE id = ?", (snapshot_id,)
        ).fetchone()
        if row is None:
            raise KeyError(f"Metrics snapshot not found: {snapshot_id}")
        return {
            "id": row["id"],
            "publish_attempt_id": row["publish_attempt_id"],
            "export_package_id": row["export_package_id"],
            "channel_id": row["channel_id"],
            "product_id": row["product_id"],
            "video_generation_id": row["video_generation_id"],
            "script_version_id": row["script_version_id"],
            "research_card_id": row["research_card_id"],
            "source": row["source"],
            "metrics": from_json(row["metrics_json"]),
            "collected_at": row["collected_at"],
        }

    def create_feedback_signal(
        self,
        *,
        snapshot_id: str,
        signal_type: str,
        strength: str,
        message: str,
    ) -> str:
        snapshot = self.get_metrics_snapshot(snapshot_id)
        signal_id = new_id("signal")
        with self.conn:
            self.conn.execute(
                """
                INSERT INTO feedback_signals(
                    id, product_id, script_version_id, signal_type, strength,
                    message, metrics_snapshot_id, created_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    signal_id,
                    snapshot["product_id"],
                    snapshot["script_version_id"],
                    signal_type,
                    strength,
                    message,
                    snapshot_id,
                    utc_now(),
                ),
            )
        return signal_id

    def get_feedback_signal(self, signal_id: str) -> dict[str, Any]:
        row = self.conn.execute(
            "SELECT * FROM feedback_signals WHERE id = ?", (signal_id,)
        ).fetchone()
        if row is None:
            raise KeyError(f"Feedback signal not found: {signal_id}")
        return {
            "id": row["id"],
            "product_id": row["product_id"],
            "script_version_id": row["script_version_id"],
            "signal_type": row["signal_type"],
            "strength": row["strength"],
            "message": row["message"],
            "metrics_snapshot_id": row["metrics_snapshot_id"],
        }

    def _research_card_for_script(self, script_version_id: str) -> str | None:
        row = self.conn.execute(
            """
            SELECT sv.id, var.research_card_id
            FROM script_versions sv
            JOIN script_variants var ON var.id = sv.variant_id
            WHERE sv.id = ?
            """,
            (script_version_id,),
        ).fetchone()
        return row["research_card_id"] if row else None


class PublishingService:
    def __init__(self, *, repo: PublishingRepository, jobs: JobRepository) -> None:
        self.repo = repo
        self.jobs = jobs

    def schedule_package(
        self, *, export_package_id: str, channel_id: str, scheduled_at: str | None = None
    ) -> str:
        package = self.repo.get_export_package(export_package_id)
        if package["review_state"] != "exported":
            raise ApprovalError("Export package must be approved and exported before publishing")
        channel = self.repo.get_channel(channel_id)
        if not channel["enabled"]:
            raise ApprovalError("Publishing channel is disabled")
        attempt_id = self.repo.create_attempt(
            export_package_id=export_package_id,
            channel_id=channel_id,
            scheduled_at=scheduled_at,
        )
        if channel["channel_type"] == "api":
            self.jobs.create_job(
                job_type="publish_status_poll",
                input_ref_type="publish_attempt",
                input_ref_id=attempt_id,
                payload={"provider": channel["provider"]},
                max_retries=3,
            )
        return attempt_id

    def record_provider_status(
        self, *, attempt_id: str, provider_status: str, response: dict[str, Any]
    ) -> dict[str, Any]:
        status = {
            "queued": "submitted",
            "processing": "submitted",
            "published": "published",
            "failed": "failed",
        }.get(provider_status, "submitted")
        error = response.get("error") if status == "failed" else None
        return self.repo.update_attempt_status(
            attempt_id,
            status=status,
            provider_status=provider_status,
            provider_response=response,
            error=error,
        )

    def import_metrics(
        self, *, attempt_id: str, metrics: MetricsSnapshot, source: str = "manual"
    ) -> tuple[str, str | None]:
        snapshot_id = self.repo.create_metrics_snapshot(
            attempt_id=attempt_id,
            metrics=metrics,
            source=source,
        )
        signal_id = generate_feedback_signal(self.repo, snapshot_id)
        return snapshot_id, signal_id


def generate_feedback_signal(repo: PublishingRepository, snapshot_id: str) -> str | None:
    snapshot = repo.get_metrics_snapshot(snapshot_id)
    metrics = snapshot["metrics"]
    views = int(metrics.get("views", 0))
    engagement = int(metrics.get("engagement", 0))
    conversions = int(metrics.get("conversions", 0))
    if views < 100:
        return None
    engagement_rate = engagement / views if views else 0
    if conversions > 0 or engagement_rate >= 0.08:
        return repo.create_feedback_signal(
            snapshot_id=snapshot_id,
            signal_type="creative_pattern",
            strength="strong",
            message="High engagement or conversion signal; reuse this product/script pattern.",
        )
    if engagement_rate < 0.02:
        return repo.create_feedback_signal(
            snapshot_id=snapshot_id,
            signal_type="creative_pattern",
            strength="weak",
            message="Low engagement signal; revise hook or product positioning before scaling.",
        )
    return repo.create_feedback_signal(
        snapshot_id=snapshot_id,
        signal_type="creative_pattern",
        strength="moderate",
        message="Moderate performance signal; continue testing variants.",
    )
