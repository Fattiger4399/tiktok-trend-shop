from __future__ import annotations

from pathlib import Path
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from tiktok_trend_shop.db import Migrator, connect
from tiktok_trend_shop.pipeline import import_products_csv, run_batch_workflow


class CsvPipelineTests(unittest.TestCase):
    def test_import_csv_and_run_batch(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            csv_path = Path(temp_dir) / "products.csv"
            csv_path.write_text(
                "\n".join(
                    [
                        "title,region,category,canonical_url,source_id,views,sales,engagement,growth_rate,commission_rate,margin,competitor_count",
                        "Mini Fan,CN,home,,csv-1,100w+,5w~10w,1200,5%~10%,5%,,",
                        "Desk Lamp,CN,home,,csv-2,50000,1000,500,0.2,0.1,,",
                    ]
                ),
                encoding="utf-8",
            )
            conn = connect("sqlite:///:memory:")
            Migrator(conn).migrate()

            imported = import_products_csv(conn=conn, csv_path=csv_path, default_region="CN")
            result = run_batch_workflow(
                conn=conn,
                storage_root=Path(temp_dir) / "assets",
                limit=2,
            )

        self.assertEqual(imported["imported_rows"], 2)
        self.assertEqual(len(imported["product_ids"]), 2)
        self.assertEqual(result["count"], 2)
        self.assertTrue(result["results"][0]["export_package_id"].startswith("export_"))


if __name__ == "__main__":
    unittest.main()
