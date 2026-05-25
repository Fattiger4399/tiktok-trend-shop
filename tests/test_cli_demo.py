from __future__ import annotations

from contextlib import redirect_stdout
from pathlib import Path
from io import StringIO
import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from tiktok_trend_shop.cli import main


class CliDemoTests(unittest.TestCase):
    def test_run_demo_outputs_workflow_ids(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            old_env = dict(os.environ)
            try:
                os.environ["TTS_DATABASE_URL"] = f"sqlite:///{Path(temp_dir) / 'demo.sqlite3'}"
                os.environ["TTS_STORAGE_ROOT"] = str(Path(temp_dir) / "assets")
                output = StringIO()
                with redirect_stdout(output):
                    exit_code = main(
                        [
                            "run-demo",
                            "--title",
                            "Mini Fan",
                            "--category",
                            "home",
                            "--views",
                            "1800",
                            "--sales",
                            "20",
                        ]
                    )
                payload = json.loads(output.getvalue())
            finally:
                os.environ.clear()
                os.environ.update(old_env)

        self.assertEqual(exit_code, 0)
        self.assertTrue(payload["product_id"].startswith("prod_"))
        self.assertTrue(payload["export_package_id"].startswith("export_"))
        self.assertTrue(payload["publish_attempt_id"].startswith("pub_"))
        self.assertIn("#tiktokshop", payload["hashtags"])


if __name__ == "__main__":
    unittest.main()
