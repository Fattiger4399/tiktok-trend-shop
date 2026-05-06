from __future__ import annotations

from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from tiktok_trend_shop.db import Migrator, connect
from tiktok_trend_shop.jobs import JobRepository, JobStatus


class JobTests(unittest.TestCase):
    def setUp(self) -> None:
        self.conn = connect("sqlite:///:memory:")
        Migrator(self.conn).migrate()
        self.jobs = JobRepository(self.conn)

    def test_job_lifecycle_and_retry(self) -> None:
        job = self.jobs.create_job(
            job_type="collect_trends",
            input_ref_type="product",
            input_ref_id="prod_1",
            payload={"region": "US"},
            max_retries=1,
        )
        self.assertEqual(job.status, JobStatus.QUEUED)

        running = self.jobs.start_job(job.id)
        self.assertEqual(running.status, JobStatus.RUNNING)
        self.assertIsNotNone(running.started_at)

        failed = self.jobs.fail_job(job.id, "provider timed out")
        self.assertEqual(failed.status, JobStatus.FAILED)
        self.assertEqual(failed.retry_count, 1)
        self.assertEqual(failed.last_error, "provider timed out")

        queued = self.jobs.retry_job(job.id)
        self.assertEqual(queued.status, JobStatus.QUEUED)
        self.assertIsNone(queued.last_error)

        done = self.jobs.complete_job(job.id)
        self.assertEqual(done.status, JobStatus.SUCCEEDED)
        self.assertIsNotNone(done.completed_at)


if __name__ == "__main__":
    unittest.main()
