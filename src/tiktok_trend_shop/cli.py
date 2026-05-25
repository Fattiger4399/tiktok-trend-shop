from __future__ import annotations

import argparse
import json
from pathlib import Path

from .config import AppConfig
from .db import Migrator, connect_from_config
from .pipeline import (
    import_products_csv,
    run_batch_workflow,
    run_manual_demo_pipeline,
    show_results,
)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="TikTok Trend Shop foundation utilities")
    subcommands = parser.add_subparsers(dest="command", required=True)
    subcommands.add_parser("config", help="Print redacted runtime configuration")
    subcommands.add_parser("migrate", help="Run SQLite migrations")
    demo = subcommands.add_parser("run-demo", help="Run the local manual product workflow")
    demo.add_argument("--title", required=True, help="Product title")
    demo.add_argument("--region", default="US", help="Market region, e.g. US")
    demo.add_argument("--category", default=None, help="Product category")
    demo.add_argument("--url", default=None, help="Canonical product URL")
    demo.add_argument("--views", type=int, default=1800, help="Manual trend views")
    demo.add_argument("--sales", type=int, default=0, help="Manual sales count")
    demo.add_argument("--engagement", type=int, default=120, help="Manual engagement count")
    demo.add_argument("--clicks", type=int, default=20, help="Manual click count")
    demo.add_argument("--conversions", type=int, default=1, help="Manual conversion count")
    demo.add_argument("--revenue", type=float, default=0.0, help="Manual revenue")
    import_csv = subcommands.add_parser("import-csv", help="Import product rows from CSV")
    import_csv.add_argument("--file", required=True, help="CSV file path")
    import_csv.add_argument("--region", default="CN", help="Default region for rows")
    batch = subcommands.add_parser("run-batch", help="Run workflow for imported products")
    batch.add_argument("--limit", type=int, default=10, help="Maximum number of products")
    results = subcommands.add_parser("show-results", help="Show latest product workflow results")
    results.add_argument("--limit", type=int, default=10, help="Maximum number of products")
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    config = AppConfig.from_env()
    if args.command == "config":
        print(json.dumps(config.redacted(), indent=2, sort_keys=True, ensure_ascii=False))
        return 0
    if args.command == "migrate":
        conn = connect_from_config(config)
        try:
            applied = Migrator(conn).migrate()
            print(json.dumps({"applied": applied}, indent=2, ensure_ascii=False))
        finally:
            conn.close()
        return 0
    if args.command == "run-demo":
        conn = connect_from_config(config)
        try:
            result = run_manual_demo_pipeline(
                conn=conn,
                storage_root=config.storage_root,
                title=args.title,
                region=args.region,
                category=args.category,
                canonical_url=args.url,
                views=args.views,
                sales=args.sales,
                engagement=args.engagement,
                clicks=args.clicks,
                conversions=args.conversions,
                revenue=args.revenue,
            )
            print(json.dumps(result, indent=2, sort_keys=True, ensure_ascii=False))
        finally:
            conn.close()
        return 0
    if args.command == "import-csv":
        conn = connect_from_config(config)
        try:
            result = import_products_csv(
                conn=conn,
                csv_path=Path(args.file),
                default_region=args.region,
            )
            print(json.dumps(result, indent=2, sort_keys=True, ensure_ascii=False))
        finally:
            conn.close()
        return 0
    if args.command == "run-batch":
        conn = connect_from_config(config)
        try:
            result = run_batch_workflow(
                conn=conn,
                storage_root=config.storage_root,
                limit=args.limit,
            )
            print(json.dumps(result, indent=2, sort_keys=True, ensure_ascii=False))
        finally:
            conn.close()
        return 0
    if args.command == "show-results":
        conn = connect_from_config(config)
        try:
            result = show_results(conn=conn, limit=args.limit)
            print(json.dumps(result, indent=2, sort_keys=True, ensure_ascii=False))
        finally:
            conn.close()
        return 0
    raise AssertionError(f"Unhandled command: {args.command}")


if __name__ == "__main__":
    raise SystemExit(main())
