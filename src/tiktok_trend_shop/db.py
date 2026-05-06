from __future__ import annotations

from pathlib import Path
import sqlite3

from .config import AppConfig


class DatabaseError(RuntimeError):
    """Raised when database setup fails."""


MIGRATIONS: tuple[tuple[str, str], ...] = (
    (
        "001_foundation",
        """
        CREATE TABLE IF NOT EXISTS products (
            id TEXT PRIMARY KEY,
            canonical_url TEXT,
            title TEXT NOT NULL,
            region TEXT NOT NULL,
            category TEXT,
            workflow_state TEXT NOT NULL DEFAULT 'candidate',
            metadata_json TEXT NOT NULL DEFAULT '{}',
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        );

        CREATE TABLE IF NOT EXISTS source_snapshots (
            id TEXT PRIMARY KEY,
            product_id TEXT NOT NULL,
            provider TEXT NOT NULL,
            source_type TEXT NOT NULL,
            source_id TEXT,
            source_url TEXT,
            captured_at TEXT NOT NULL,
            metrics_json TEXT NOT NULL DEFAULT '{}',
            raw_ref TEXT,
            metadata_json TEXT NOT NULL DEFAULT '{}',
            created_at TEXT NOT NULL,
            FOREIGN KEY(product_id) REFERENCES products(id)
        );

        CREATE TABLE IF NOT EXISTS jobs (
            id TEXT PRIMARY KEY,
            job_type TEXT NOT NULL,
            status TEXT NOT NULL,
            input_ref_type TEXT,
            input_ref_id TEXT,
            payload_json TEXT NOT NULL DEFAULT '{}',
            retry_count INTEGER NOT NULL DEFAULT 0,
            max_retries INTEGER NOT NULL DEFAULT 0,
            last_error TEXT,
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL,
            started_at TEXT,
            completed_at TEXT
        );

        CREATE TABLE IF NOT EXISTS assets (
            id TEXT PRIMARY KEY,
            kind TEXT NOT NULL,
            backend TEXT NOT NULL,
            uri TEXT NOT NULL,
            content_type TEXT,
            byte_size INTEGER,
            checksum TEXT,
            metadata_json TEXT NOT NULL DEFAULT '{}',
            created_at TEXT NOT NULL
        );

        CREATE TABLE IF NOT EXISTS review_states (
            id TEXT PRIMARY KEY,
            artifact_type TEXT NOT NULL,
            artifact_id TEXT NOT NULL,
            product_id TEXT,
            state TEXT NOT NULL,
            reviewer TEXT,
            notes TEXT,
            checklist_json TEXT NOT NULL DEFAULT '{}',
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL,
            FOREIGN KEY(product_id) REFERENCES products(id)
        );

        CREATE TABLE IF NOT EXISTS analytics_refs (
            id TEXT PRIMARY KEY,
            product_id TEXT NOT NULL,
            channel TEXT NOT NULL,
            external_id TEXT,
            metrics_json TEXT NOT NULL DEFAULT '{}',
            collected_at TEXT NOT NULL,
            metadata_json TEXT NOT NULL DEFAULT '{}',
            created_at TEXT NOT NULL,
            FOREIGN KEY(product_id) REFERENCES products(id)
        );

        CREATE TABLE IF NOT EXISTS audit_events (
            id TEXT PRIMARY KEY,
            subject_type TEXT NOT NULL,
            subject_id TEXT NOT NULL,
            event_type TEXT NOT NULL,
            actor TEXT,
            provider TEXT,
            source_url TEXT,
            source_id TEXT,
            prompt_version TEXT,
            model TEXT,
            renderer TEXT,
            metadata_json TEXT NOT NULL DEFAULT '{}',
            created_at TEXT NOT NULL
        );

        CREATE INDEX IF NOT EXISTS idx_source_snapshots_product ON source_snapshots(product_id);
        CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
        CREATE INDEX IF NOT EXISTS idx_assets_kind ON assets(kind);
        CREATE INDEX IF NOT EXISTS idx_review_states_product ON review_states(product_id);
        CREATE INDEX IF NOT EXISTS idx_analytics_refs_product ON analytics_refs(product_id);
        CREATE INDEX IF NOT EXISTS idx_audit_events_subject ON audit_events(subject_type, subject_id);
        """,
    ),
)


def sqlite_path_from_url(database_url: str) -> Path:
    if not database_url.startswith("sqlite:///"):
        raise DatabaseError("Only sqlite:/// database URLs are supported")
    path = database_url.removeprefix("sqlite:///")
    return Path(path)


def connect(database_url: str) -> sqlite3.Connection:
    path = sqlite_path_from_url(database_url)
    if str(path) not in {":memory:", ""}:
        path.parent.mkdir(parents=True, exist_ok=True)
    conn = sqlite3.connect(str(path))
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA foreign_keys = ON")
    return conn


def connect_from_config(config: AppConfig) -> sqlite3.Connection:
    return connect(config.database_url)


class Migrator:
    def __init__(self, conn: sqlite3.Connection) -> None:
        self.conn = conn

    def migrate(self) -> list[str]:
        self.conn.execute(
            """
            CREATE TABLE IF NOT EXISTS schema_migrations (
                name TEXT PRIMARY KEY,
                applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
            )
            """
        )
        applied = {
            row["name"]
            for row in self.conn.execute("SELECT name FROM schema_migrations").fetchall()
        }
        applied_now: list[str] = []
        for name, sql in MIGRATIONS:
            if name in applied:
                continue
            with self.conn:
                self.conn.executescript(sql)
                self.conn.execute("INSERT INTO schema_migrations(name) VALUES (?)", (name,))
            applied_now.append(name)
        return applied_now
