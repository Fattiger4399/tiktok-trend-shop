from __future__ import annotations

import argparse
import json

from .config import AppConfig
from .db import Migrator, connect_from_config


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="TikTok Trend Shop foundation utilities")
    subcommands = parser.add_subparsers(dest="command", required=True)
    subcommands.add_parser("config", help="Print redacted runtime configuration")
    subcommands.add_parser("migrate", help="Run SQLite migrations")
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    config = AppConfig.from_env()
    if args.command == "config":
        print(json.dumps(config.redacted(), indent=2, sort_keys=True))
        return 0
    if args.command == "migrate":
        conn = connect_from_config(config)
        applied = Migrator(conn).migrate()
        print(json.dumps({"applied": applied}, indent=2))
        return 0
    raise AssertionError(f"Unhandled command: {args.command}")


if __name__ == "__main__":
    raise SystemExit(main())
