from __future__ import annotations

import sqlite3
from typing import Any

from .domain.models import JobRecord
from .repositories import from_json, new_id, to_json, utc_now


class JobStatus:
    QUEUED = "queued"
    RUNNING = "running"
    SUCCEEDED = "succeeded"
    FAILED = "failed"
    CANCELED = "canceled"


class JobRepository:
    def __init__(self, conn: sqlite3.Connection) -> None:
        self.conn = conn

    def create_job(
        self,
        *,
        job_type: str,
        input_ref_type: str | None = None,
        input_ref_id: str | None = None,
        payload: dict[str, Any] | None = None,
        max_retries: int = 0,
    ) -> JobRecord:
        job_id = new_id("job")
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                INSERT INTO jobs(
                    id, job_type, status, input_ref_type, input_ref_id, payload_json,
                    retry_count, max_retries, created_at, updated_at
                )
                VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?, ?)
                """,
                (
                    job_id,
                    job_type,
                    JobStatus.QUEUED,
                    input_ref_type,
                    input_ref_id,
                    to_json(payload),
                    max_retries,
                    now,
                    now,
                ),
            )
        return self.get_job(job_id)

    def get_job(self, job_id: str) -> JobRecord:
        row = self.conn.execute("SELECT * FROM jobs WHERE id = ?", (job_id,)).fetchone()
        if row is None:
            raise KeyError(f"Job not found: {job_id}")
        return _job_from_row(row)

    def start_job(self, job_id: str) -> JobRecord:
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                UPDATE jobs
                SET status = ?, started_at = COALESCE(started_at, ?), updated_at = ?
                WHERE id = ?
                """,
                (JobStatus.RUNNING, now, now, job_id),
            )
        return self.get_job(job_id)

    def complete_job(self, job_id: str) -> JobRecord:
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                UPDATE jobs
                SET status = ?, completed_at = ?, updated_at = ?, last_error = NULL
                WHERE id = ?
                """,
                (JobStatus.SUCCEEDED, now, now, job_id),
            )
        return self.get_job(job_id)

    def fail_job(self, job_id: str, error: str) -> JobRecord:
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                UPDATE jobs
                SET status = ?,
                    retry_count = retry_count + 1,
                    last_error = ?,
                    completed_at = ?,
                    updated_at = ?
                WHERE id = ?
                """,
                (JobStatus.FAILED, error, now, now, job_id),
            )
        return self.get_job(job_id)

    def retry_job(self, job_id: str) -> JobRecord:
        job = self.get_job(job_id)
        if job.status != JobStatus.FAILED:
            raise ValueError("Only failed jobs can be retried")
        if job.retry_count > job.max_retries:
            raise ValueError("Job has no retries remaining")
        now = utc_now()
        with self.conn:
            self.conn.execute(
                """
                UPDATE jobs
                SET status = ?, last_error = NULL, completed_at = NULL, updated_at = ?
                WHERE id = ?
                """,
                (JobStatus.QUEUED, now, job_id),
            )
        return self.get_job(job_id)


def _job_from_row(row: sqlite3.Row) -> JobRecord:
    return JobRecord(
        id=row["id"],
        job_type=row["job_type"],
        status=row["status"],
        input_ref_type=row["input_ref_type"],
        input_ref_id=row["input_ref_id"],
        payload=from_json(row["payload_json"]),
        retry_count=row["retry_count"],
        max_retries=row["max_retries"],
        last_error=row["last_error"],
        created_at=row["created_at"],
        updated_at=row["updated_at"],
        started_at=row["started_at"],
        completed_at=row["completed_at"],
    )
