from __future__ import annotations

from dataclasses import dataclass

from tiktok_trend_shop.jobs import JobRepository, JobStatus


@dataclass
class WorkerResult:
    job_id: str
    status: str
    message: str


class WorkerRunner:
    def __init__(self, jobs: JobRepository) -> None:
        self.jobs = jobs

    def mark_job_unhandled(self, job_id: str) -> WorkerResult:
        job = self.jobs.start_job(job_id)
        failed = self.jobs.fail_job(job.id, "No worker handler registered for job type")
        return WorkerResult(
            job_id=failed.id,
            status=JobStatus.FAILED,
            message=failed.last_error or "failed",
        )
