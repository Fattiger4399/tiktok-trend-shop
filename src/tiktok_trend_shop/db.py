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
    (
        "002_trend_source_ingestion",
        """
        CREATE TABLE IF NOT EXISTS source_provider_states (
            provider TEXT PRIMARY KEY,
            enabled INTEGER NOT NULL DEFAULT 1,
            status TEXT NOT NULL DEFAULT 'healthy',
            last_success_at TEXT,
            last_error TEXT,
            rate_limited_until TEXT,
            metadata_json TEXT NOT NULL DEFAULT '{}',
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        );

        CREATE TABLE IF NOT EXISTS normalized_entities (
            id TEXT PRIMARY KEY,
            product_id TEXT,
            entity_type TEXT NOT NULL,
            provider TEXT NOT NULL,
            source_id TEXT,
            source_url TEXT,
            name TEXT,
            metrics_json TEXT NOT NULL DEFAULT '{}',
            metadata_json TEXT NOT NULL DEFAULT '{}',
            captured_at TEXT NOT NULL,
            created_at TEXT NOT NULL,
            FOREIGN KEY(product_id) REFERENCES products(id)
        );

        CREATE TABLE IF NOT EXISTS collection_schedules (
            id TEXT PRIMARY KEY,
            provider TEXT NOT NULL,
            region TEXT NOT NULL,
            category TEXT,
            keyword TEXT,
            mode TEXT NOT NULL,
            enabled INTEGER NOT NULL DEFAULT 1,
            interval_minutes INTEGER NOT NULL DEFAULT 1440,
            next_run_at TEXT,
            metadata_json TEXT NOT NULL DEFAULT '{}',
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        );

        CREATE INDEX IF NOT EXISTS idx_products_canonical_url ON products(canonical_url);
        CREATE INDEX IF NOT EXISTS idx_products_title_region ON products(title, region);
        CREATE INDEX IF NOT EXISTS idx_normalized_entities_product ON normalized_entities(product_id);
        CREATE INDEX IF NOT EXISTS idx_normalized_entities_lookup
            ON normalized_entities(entity_type, provider, source_id);
        CREATE INDEX IF NOT EXISTS idx_collection_schedules_enabled
            ON collection_schedules(enabled, next_run_at);
        """,
    ),
    (
        "003_product_intelligence",
        """
        CREATE TABLE IF NOT EXISTS product_scores (
            id TEXT PRIMARY KEY,
            product_id TEXT NOT NULL,
            model_version TEXT NOT NULL,
            total_score REAL NOT NULL,
            confidence TEXT NOT NULL,
            components_json TEXT NOT NULL,
            evidence_json TEXT NOT NULL DEFAULT '{}',
            created_at TEXT NOT NULL,
            FOREIGN KEY(product_id) REFERENCES products(id)
        );

        CREATE TABLE IF NOT EXISTS research_cards (
            id TEXT PRIMARY KEY,
            product_id TEXT NOT NULL,
            score_id TEXT,
            prompt_version TEXT NOT NULL,
            model TEXT NOT NULL,
            status TEXT NOT NULL,
            card_json TEXT NOT NULL,
            evidence_json TEXT NOT NULL,
            risk_flags_json TEXT NOT NULL DEFAULT '{}',
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL,
            FOREIGN KEY(product_id) REFERENCES products(id),
            FOREIGN KEY(score_id) REFERENCES product_scores(id)
        );

        CREATE TABLE IF NOT EXISTS risk_flags (
            id TEXT PRIMARY KEY,
            product_id TEXT NOT NULL,
            artifact_type TEXT NOT NULL,
            artifact_id TEXT,
            category TEXT NOT NULL,
            severity TEXT NOT NULL,
            message TEXT NOT NULL,
            created_at TEXT NOT NULL,
            FOREIGN KEY(product_id) REFERENCES products(id)
        );

        CREATE INDEX IF NOT EXISTS idx_product_scores_product ON product_scores(product_id);
        CREATE INDEX IF NOT EXISTS idx_research_cards_product ON research_cards(product_id);
        CREATE INDEX IF NOT EXISTS idx_risk_flags_product ON risk_flags(product_id);
        """,
    ),
    (
        "004_creative_script_agent",
        """
        CREATE TABLE IF NOT EXISTS script_variants (
            id TEXT PRIMARY KEY,
            product_id TEXT NOT NULL,
            research_card_id TEXT NOT NULL,
            prompt_version TEXT NOT NULL,
            model TEXT NOT NULL,
            params_json TEXT NOT NULL,
            status TEXT NOT NULL,
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL,
            FOREIGN KEY(product_id) REFERENCES products(id),
            FOREIGN KEY(research_card_id) REFERENCES research_cards(id)
        );

        CREATE TABLE IF NOT EXISTS script_versions (
            id TEXT PRIMARY KEY,
            variant_id TEXT NOT NULL,
            version_number INTEGER NOT NULL,
            parent_version_id TEXT,
            status TEXT NOT NULL,
            script_json TEXT NOT NULL,
            validation_json TEXT NOT NULL DEFAULT '{}',
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL,
            FOREIGN KEY(variant_id) REFERENCES script_variants(id),
            FOREIGN KEY(parent_version_id) REFERENCES script_versions(id)
        );

        CREATE INDEX IF NOT EXISTS idx_script_variants_product ON script_variants(product_id);
        CREATE INDEX IF NOT EXISTS idx_script_versions_variant ON script_versions(variant_id);
        """,
    ),
    (
        "005_ai_video_generation",
        """
        CREATE TABLE IF NOT EXISTS video_generations (
            id TEXT PRIMARY KEY,
            product_id TEXT NOT NULL,
            script_version_id TEXT NOT NULL,
            status TEXT NOT NULL,
            storyboard_json TEXT NOT NULL,
            asset_plan_json TEXT NOT NULL DEFAULT '{}',
            output_asset_id TEXT,
            validation_json TEXT NOT NULL DEFAULT '{}',
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL,
            FOREIGN KEY(product_id) REFERENCES products(id),
            FOREIGN KEY(script_version_id) REFERENCES script_versions(id),
            FOREIGN KEY(output_asset_id) REFERENCES assets(id)
        );

        CREATE TABLE IF NOT EXISTS video_generation_stages (
            id TEXT PRIMARY KEY,
            video_generation_id TEXT NOT NULL,
            stage TEXT NOT NULL,
            status TEXT NOT NULL,
            retry_count INTEGER NOT NULL DEFAULT 0,
            last_error TEXT,
            asset_id TEXT,
            metadata_json TEXT NOT NULL DEFAULT '{}',
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL,
            FOREIGN KEY(video_generation_id) REFERENCES video_generations(id),
            FOREIGN KEY(asset_id) REFERENCES assets(id)
        );

        CREATE INDEX IF NOT EXISTS idx_video_generations_product ON video_generations(product_id);
        CREATE INDEX IF NOT EXISTS idx_video_generation_stages_generation
            ON video_generation_stages(video_generation_id);
        """,
    ),
    (
        "006_review_export_workflow",
        """
        CREATE TABLE IF NOT EXISTS review_feedback (
            id TEXT PRIMARY KEY,
            review_id TEXT NOT NULL,
            route_to_artifact_type TEXT NOT NULL,
            route_to_artifact_id TEXT NOT NULL,
            reason TEXT NOT NULL,
            comment TEXT,
            created_at TEXT NOT NULL,
            FOREIGN KEY(review_id) REFERENCES review_states(id)
        );

        CREATE TABLE IF NOT EXISTS export_packages (
            id TEXT PRIMARY KEY,
            product_id TEXT NOT NULL,
            review_id TEXT NOT NULL,
            video_generation_id TEXT NOT NULL,
            script_version_id TEXT NOT NULL,
            package_asset_id TEXT NOT NULL,
            package_json TEXT NOT NULL,
            created_at TEXT NOT NULL,
            FOREIGN KEY(product_id) REFERENCES products(id),
            FOREIGN KEY(review_id) REFERENCES review_states(id),
            FOREIGN KEY(video_generation_id) REFERENCES video_generations(id),
            FOREIGN KEY(script_version_id) REFERENCES script_versions(id),
            FOREIGN KEY(package_asset_id) REFERENCES assets(id)
        );

        CREATE INDEX IF NOT EXISTS idx_review_feedback_review ON review_feedback(review_id);
        CREATE INDEX IF NOT EXISTS idx_export_packages_product ON export_packages(product_id);
        """,
    ),
    (
        "007_publishing_analytics_loop",
        """
        CREATE TABLE IF NOT EXISTS publishing_channels (
            id TEXT PRIMARY KEY,
            name TEXT NOT NULL,
            channel_type TEXT NOT NULL,
            provider TEXT NOT NULL,
            account_ref TEXT,
            enabled INTEGER NOT NULL DEFAULT 1,
            metadata_json TEXT NOT NULL DEFAULT '{}',
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        );

        CREATE TABLE IF NOT EXISTS publish_attempts (
            id TEXT PRIMARY KEY,
            export_package_id TEXT NOT NULL,
            channel_id TEXT NOT NULL,
            scheduled_at TEXT,
            status TEXT NOT NULL,
            provider_status TEXT,
            provider_response_json TEXT NOT NULL DEFAULT '{}',
            error TEXT,
            retry_count INTEGER NOT NULL DEFAULT 0,
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL,
            FOREIGN KEY(export_package_id) REFERENCES export_packages(id),
            FOREIGN KEY(channel_id) REFERENCES publishing_channels(id)
        );

        CREATE TABLE IF NOT EXISTS performance_metric_snapshots (
            id TEXT PRIMARY KEY,
            publish_attempt_id TEXT NOT NULL,
            export_package_id TEXT NOT NULL,
            channel_id TEXT NOT NULL,
            product_id TEXT NOT NULL,
            video_generation_id TEXT NOT NULL,
            script_version_id TEXT NOT NULL,
            research_card_id TEXT,
            source TEXT NOT NULL,
            metrics_json TEXT NOT NULL,
            collected_at TEXT NOT NULL,
            created_at TEXT NOT NULL,
            FOREIGN KEY(publish_attempt_id) REFERENCES publish_attempts(id),
            FOREIGN KEY(export_package_id) REFERENCES export_packages(id),
            FOREIGN KEY(channel_id) REFERENCES publishing_channels(id),
            FOREIGN KEY(product_id) REFERENCES products(id),
            FOREIGN KEY(video_generation_id) REFERENCES video_generations(id),
            FOREIGN KEY(script_version_id) REFERENCES script_versions(id),
            FOREIGN KEY(research_card_id) REFERENCES research_cards(id)
        );

        CREATE TABLE IF NOT EXISTS feedback_signals (
            id TEXT PRIMARY KEY,
            product_id TEXT NOT NULL,
            script_version_id TEXT,
            signal_type TEXT NOT NULL,
            strength TEXT NOT NULL,
            message TEXT NOT NULL,
            metrics_snapshot_id TEXT NOT NULL,
            created_at TEXT NOT NULL,
            FOREIGN KEY(product_id) REFERENCES products(id),
            FOREIGN KEY(script_version_id) REFERENCES script_versions(id),
            FOREIGN KEY(metrics_snapshot_id) REFERENCES performance_metric_snapshots(id)
        );

        CREATE INDEX IF NOT EXISTS idx_publish_attempts_package ON publish_attempts(export_package_id);
        CREATE INDEX IF NOT EXISTS idx_metrics_product ON performance_metric_snapshots(product_id);
        CREATE INDEX IF NOT EXISTS idx_feedback_product ON feedback_signals(product_id);
        """,
    ),
    (
        "008_product_detail_enrichment",
        """
        CREATE TABLE IF NOT EXISTS product_detail_snapshots (
            id TEXT PRIMARY KEY,
            product_id TEXT NOT NULL,
            provider TEXT NOT NULL,
            platform TEXT,
            source_id TEXT,
            source_url TEXT,
            product_url TEXT,
            title TEXT,
            shop_name TEXT,
            brand TEXT,
            price REAL,
            currency TEXT,
            image_url TEXT,
            specs_json TEXT NOT NULL DEFAULT '{}',
            selling_points_json TEXT NOT NULL DEFAULT '{}',
            review_summary TEXT,
            review_highlights_json TEXT NOT NULL DEFAULT '{}',
            warnings_json TEXT NOT NULL DEFAULT '{}',
            missing_fields_json TEXT NOT NULL DEFAULT '{}',
            completeness_status TEXT NOT NULL,
            raw_json TEXT NOT NULL DEFAULT '{}',
            metadata_json TEXT NOT NULL DEFAULT '{}',
            captured_at TEXT NOT NULL,
            created_at TEXT NOT NULL,
            FOREIGN KEY(product_id) REFERENCES products(id)
        );

        CREATE INDEX IF NOT EXISTS idx_product_detail_snapshots_product
            ON product_detail_snapshots(product_id, captured_at);
        CREATE INDEX IF NOT EXISTS idx_product_detail_snapshots_product_url
            ON product_detail_snapshots(product_url);
        CREATE INDEX IF NOT EXISTS idx_product_detail_snapshots_source
            ON product_detail_snapshots(provider, source_id);
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
