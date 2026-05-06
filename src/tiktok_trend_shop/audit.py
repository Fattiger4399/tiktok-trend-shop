from __future__ import annotations

from dataclasses import dataclass
import sqlite3
from typing import Any

from .domain.models import AuditEvent
from .repositories import from_json, new_id, to_json, utc_now


@dataclass(frozen=True)
class AuditMetadata:
    provider: str | None = None
    source_url: str | None = None
    source_id: str | None = None
    prompt_version: str | None = None
    model: str | None = None
    renderer: str | None = None
    metadata: dict[str, Any] | None = None


class AuditService:
    def __init__(self, conn: sqlite3.Connection) -> None:
        self.conn = conn

    def record_event(
        self,
        *,
        subject_type: str,
        subject_id: str,
        event_type: str,
        actor: str | None = None,
        audit: AuditMetadata | None = None,
    ) -> AuditEvent:
        audit = audit or AuditMetadata()
        event_id = new_id("audit")
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                INSERT INTO audit_events(
                    id, subject_type, subject_id, event_type, actor, provider,
                    source_url, source_id, prompt_version, model, renderer,
                    metadata_json, created_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    event_id,
                    subject_type,
                    subject_id,
                    event_type,
                    actor,
                    audit.provider,
                    audit.source_url,
                    audit.source_id,
                    audit.prompt_version,
                    audit.model,
                    audit.renderer,
                    to_json(audit.metadata),
                    now,
                ),
            )
        return self.get_event(event_id)

    def record_external_input(
        self,
        *,
        subject_type: str,
        subject_id: str,
        provider: str,
        source_url: str | None = None,
        source_id: str | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> AuditEvent:
        return self.record_event(
            subject_type=subject_type,
            subject_id=subject_id,
            event_type="external_input",
            audit=AuditMetadata(
                provider=provider,
                source_url=source_url,
                source_id=source_id,
                metadata=metadata,
            ),
        )

    def record_generated_output(
        self,
        *,
        subject_type: str,
        subject_id: str,
        provider: str | None = None,
        prompt_version: str | None = None,
        model: str | None = None,
        renderer: str | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> AuditEvent:
        return self.record_event(
            subject_type=subject_type,
            subject_id=subject_id,
            event_type="generated_output",
            audit=AuditMetadata(
                provider=provider,
                prompt_version=prompt_version,
                model=model,
                renderer=renderer,
                metadata=metadata,
            ),
        )

    def get_event(self, event_id: str) -> AuditEvent:
        row = self.conn.execute("SELECT * FROM audit_events WHERE id = ?", (event_id,)).fetchone()
        if row is None:
            raise KeyError(f"Audit event not found: {event_id}")
        return AuditEvent(
            id=row["id"],
            subject_type=row["subject_type"],
            subject_id=row["subject_id"],
            event_type=row["event_type"],
            actor=row["actor"],
            provider=row["provider"],
            source_url=row["source_url"],
            source_id=row["source_id"],
            prompt_version=row["prompt_version"],
            model=row["model"],
            renderer=row["renderer"],
            metadata=from_json(row["metadata_json"]),
            created_at=row["created_at"],
        )
