package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

func Open(databaseURL string) (*sql.DB, error) {
	path, err := SQLitePath(databaseURL)
	if err != nil {
		return nil, err
	}
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// DB is an alias for *sql.DB used by callers that want a typed handle.
type DB = *sql.DB

func SQLitePath(databaseURL string) (string, error) {
	if databaseURL == ":memory:" || databaseURL == "sqlite:///:memory:" {
		return ":memory:", nil
	}
	const prefix = "sqlite:///"
	if !strings.HasPrefix(databaseURL, prefix) {
		return "", fmt.Errorf("only sqlite:/// database URLs are supported")
	}
	path := strings.TrimPrefix(databaseURL, prefix)
	if path == "" {
		return "", fmt.Errorf("sqlite database path is empty")
	}
	return path, nil
}

// Migration is one additive step applied in order.
type Migration struct {
	Name  string
	SQL   string
	Check func(*sql.DB) (bool, error)
}

// Migrations are applied in declared order. Each migration's Check function
// determines whether it has already been applied. When Check returns false the
// SQL block is executed and the name is recorded in schema_migrations.
var Migrations = []Migration{
	{
		Name: "go_core_backend_001",
		SQL: coreBackend001SQL,
		Check: func(db *sql.DB) (bool, error) {
			return tableExists(db, "products")
		},
	},
	{
		Name: "go_amazon_identity_002",
		SQL:  amazonIdentity002SQL,
		Check: func(db *sql.DB) (bool, error) {
			return columnExists(db, "products", "marketplace")
		},
	},
	{
		Name: "go_canonical_categories_003",
		SQL:  canonicalCategories003SQL,
		Check: func(db *sql.DB) (bool, error) {
			return tableExists(db, "canonical_categories")
		},
	},
	{
		Name: "go_category_assignments_004",
		SQL:  categoryAssignments004SQL,
		Check: func(db *sql.DB) (bool, error) {
			return tableExists(db, "product_category_assignments")
		},
	},
	{
		Name: "go_hotspot_scores_005",
		SQL:  hotspotScores005SQL,
		Check: func(db *sql.DB) (bool, error) {
			return tableExists(db, "hotspot_scores")
		},
	},
	{
		Name: "go_import_jobs_006",
		SQL:  importJobs006SQL,
		Check: func(db *sql.DB) (bool, error) {
			return tableExists(db, "import_jobs")
		},
	},
	{
		Name: "go_material_requests_007",
		SQL:  materialRequests007SQL,
		Check: func(db *sql.DB) (bool, error) {
			return tableExists(db, "material_requests")
		},
	},
	{
		Name: "go_copy_variants_008",
		SQL:  copyVariants008SQL,
		Check: func(db *sql.DB) (bool, error) {
			return tableExists(db, "copy_variants")
		},
	},
	{
		Name: "go_review_deliveries_009",
		SQL:  reviewDeliveries009SQL,
		Check: func(db *sql.DB) (bool, error) {
			return tableExists(db, "review_events")
		},
	},
	{
		Name: "go_auth_users_010",
		SQL:  authUsers010SQL,
		Check: func(db *sql.DB) (bool, error) {
			return tableExists(db, "users")
		},
	},
	{
		Name: "go_dossier_assets_011",
		SQL:  dossierAssets011SQL,
		Check: func(db *sql.DB) (bool, error) {
			return tableExists(db, "dossier_assets")
		},
	},
	{
		Name: "go_mediagen_assets_012",
		SQL:  mediagenAssets012SQL,
		Check: func(db *sql.DB) (bool, error) {
			return tableExists(db, "image_gen_tasks")
		},
	},
}

func Migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		name TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return err
	}
	applied, err := loadAppliedMigrations(db)
	if err != nil {
		return err
	}
	for _, m := range Migrations {
		if applied[m.Name] {
			continue
		}
		already, err := m.Check(db)
		if err != nil {
			return err
		}
		if already {
			if _, err := db.Exec(
				`INSERT OR IGNORE INTO schema_migrations(name) VALUES (?)`,
				m.Name,
			); err != nil {
				return err
			}
			applied[m.Name] = true
			continue
		}
		if _, err := db.Exec(m.SQL); err != nil {
			return fmt.Errorf("apply migration %s: %w", m.Name, err)
		}
		if _, err := db.Exec(
			`INSERT INTO schema_migrations(name) VALUES (?)`,
			m.Name,
		); err != nil {
			return err
		}
		applied[m.Name] = true
	}
	return nil
}

func loadAppliedMigrations(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query(`SELECT name FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out[name] = true
	}
	return out, rows.Err()
}

func tableExists(db *sql.DB, name string) (bool, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type IN ('table','view') AND name = ?`,
		name,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func columnExists(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if strings.EqualFold(name, column) {
			return true, nil
		}
	}
	return false, rows.Err()
}

const coreBackend001SQL = `
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

CREATE INDEX IF NOT EXISTS idx_products_canonical_url ON products(canonical_url);
CREATE INDEX IF NOT EXISTS idx_products_title_region ON products(title, region);
CREATE INDEX IF NOT EXISTS idx_source_snapshots_product ON source_snapshots(product_id);
CREATE INDEX IF NOT EXISTS idx_product_detail_snapshots_product
	ON product_detail_snapshots(product_id, captured_at);
`

const amazonIdentity002SQL = `
ALTER TABLE products ADD COLUMN marketplace TEXT;
ALTER TABLE products ADD COLUMN asin TEXT;
ALTER TABLE products ADD COLUMN source_category TEXT;
ALTER TABLE products ADD COLUMN source_category_id TEXT;
ALTER TABLE products ADD COLUMN last_captured_at TEXT;

CREATE INDEX IF NOT EXISTS idx_products_marketplace_asin
	ON products(marketplace, asin);
CREATE UNIQUE INDEX IF NOT EXISTS uq_products_marketplace_asin
	ON products(marketplace, asin)
	WHERE marketplace IS NOT NULL AND asin IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_source_snapshots_captured
	ON source_snapshots(captured_at);
CREATE INDEX IF NOT EXISTS idx_source_snapshots_provider_source
	ON source_snapshots(provider, source_id);

ALTER TABLE source_snapshots ADD COLUMN metric_kind TEXT NOT NULL DEFAULT 'observed';
`

const canonicalCategories003SQL = `
CREATE TABLE IF NOT EXISTS canonical_categories (
	id TEXT PRIMARY KEY,
	parent_id TEXT,
	name TEXT NOT NULL,
	slug TEXT NOT NULL UNIQUE,
	display_order INTEGER NOT NULL DEFAULT 0,
	active INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY(parent_id) REFERENCES canonical_categories(id)
);

CREATE TABLE IF NOT EXISTS source_category_mappings (
	id TEXT PRIMARY KEY,
	provider TEXT NOT NULL,
	source_category_id TEXT NOT NULL,
	source_category_name TEXT,
	canonical_category_id TEXT NOT NULL,
	confidence REAL NOT NULL DEFAULT 1.0,
	method TEXT NOT NULL DEFAULT 'exact',
	active INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(provider, source_category_id),
	FOREIGN KEY(canonical_category_id) REFERENCES canonical_categories(id)
);

CREATE INDEX IF NOT EXISTS idx_source_mappings_canonical
	ON source_category_mappings(canonical_category_id);
`

const categoryAssignments004SQL = `
CREATE TABLE IF NOT EXISTS product_category_assignments (
	id TEXT PRIMARY KEY,
	product_id TEXT NOT NULL,
	canonical_category_id TEXT,
	method TEXT NOT NULL,
	confidence REAL NOT NULL DEFAULT 0.0,
	source_category_id TEXT,
	source_category_name TEXT,
	manual INTEGER NOT NULL DEFAULT 0,
	needs_review INTEGER NOT NULL DEFAULT 0,
	assigned_at TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(product_id),
	FOREIGN KEY(product_id) REFERENCES products(id),
	FOREIGN KEY(canonical_category_id) REFERENCES canonical_categories(id)
);

CREATE INDEX IF NOT EXISTS idx_assignments_canonical
	ON product_category_assignments(canonical_category_id);
CREATE INDEX IF NOT EXISTS idx_assignments_needs_review
	ON product_category_assignments(needs_review);
`

const hotspotScores005SQL = `
CREATE TABLE IF NOT EXISTS hotspot_scores (
	id TEXT PRIMARY KEY,
	product_id TEXT NOT NULL,
	marketplace TEXT,
	canonical_category_id TEXT,
	model_version TEXT NOT NULL,
	time_window TEXT NOT NULL,
	comparison_group TEXT,
	total_score REAL NOT NULL,
	confidence TEXT NOT NULL,
	components_json TEXT NOT NULL DEFAULT '{}',
	missing_components_json TEXT NOT NULL DEFAULT '{}',
	evidence_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	FOREIGN KEY(product_id) REFERENCES products(id),
	FOREIGN KEY(canonical_category_id) REFERENCES canonical_categories(id)
);

CREATE INDEX IF NOT EXISTS idx_hotspot_product
	ON hotspot_scores(product_id, created_at);
CREATE INDEX IF NOT EXISTS idx_hotspot_group
	ON hotspot_scores(marketplace, canonical_category_id, time_window, created_at);
`

const importJobs006SQL = `
CREATE TABLE IF NOT EXISTS import_jobs (
	id TEXT PRIMARY KEY,
	source TEXT NOT NULL,
	filename TEXT,
	status TEXT NOT NULL,
	total_rows INTEGER NOT NULL DEFAULT 0,
	imported_rows INTEGER NOT NULL DEFAULT 0,
	updated_rows INTEGER NOT NULL DEFAULT 0,
	duplicate_rows INTEGER NOT NULL DEFAULT 0,
	rejected_rows INTEGER NOT NULL DEFAULT 0,
	idempotency_key TEXT,
	started_at TEXT NOT NULL,
	completed_at TEXT,
	summary_json TEXT NOT NULL DEFAULT '{}',
	errors_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_import_jobs_status
	ON import_jobs(status, created_at);
CREATE INDEX IF NOT EXISTS idx_import_jobs_idempotency
	ON import_jobs(idempotency_key);

CREATE TABLE IF NOT EXISTS import_job_rows (
	id TEXT PRIMARY KEY,
	import_job_id TEXT NOT NULL,
	row_number INTEGER NOT NULL,
	status TEXT NOT NULL,
	product_id TEXT,
	detail_snapshot_id TEXT,
	reason TEXT,
	field TEXT,
	raw_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	FOREIGN KEY(import_job_id) REFERENCES import_jobs(id)
);

CREATE INDEX IF NOT EXISTS idx_import_job_rows_job
	ON import_job_rows(import_job_id, row_number);
`

const materialRequests007SQL = `
CREATE TABLE IF NOT EXISTS material_requests (
	id TEXT PRIMARY KEY,
	product_id TEXT NOT NULL,
	client_id TEXT NOT NULL DEFAULT '',
	usage TEXT NOT NULL DEFAULT '',
	style TEXT NOT NULL DEFAULT '',
	focus TEXT NOT NULL DEFAULT '',
	notes TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'submitted',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY(product_id) REFERENCES products(id)
);

CREATE INDEX IF NOT EXISTS idx_material_requests_product
	ON material_requests(product_id);
CREATE INDEX IF NOT EXISTS idx_material_requests_status
	ON material_requests(status);
`

const copyVariants008SQL = `
CREATE TABLE IF NOT EXISTS copy_variants (
	id TEXT PRIMARY KEY,
	request_id TEXT NOT NULL,
	variant_no INTEGER NOT NULL,
	hook TEXT NOT NULL DEFAULT '',
	body TEXT NOT NULL DEFAULT '',
	caption TEXT NOT NULL DEFAULT '',
	hashtags TEXT NOT NULL DEFAULT '[]',
	provider TEXT NOT NULL,
	model TEXT NOT NULL DEFAULT '',
	prompt_version TEXT NOT NULL DEFAULT '1.0',
	created_at TEXT NOT NULL,
	FOREIGN KEY(request_id) REFERENCES material_requests(id)
);

CREATE INDEX IF NOT EXISTS idx_copy_variants_request
	ON copy_variants(request_id);
`

const reviewDeliveries009SQL = `
CREATE TABLE IF NOT EXISTS review_events (
	id TEXT PRIMARY KEY,
	request_id TEXT NOT NULL,
	action TEXT NOT NULL,
	actor TEXT NOT NULL DEFAULT '',
	note TEXT NOT NULL DEFAULT '',
	variant_id TEXT,
	created_at TEXT NOT NULL,
	FOREIGN KEY(request_id) REFERENCES material_requests(id)
);

CREATE INDEX IF NOT EXISTS idx_review_events_request
	ON review_events(request_id);

CREATE TABLE IF NOT EXISTS deliveries (
	id TEXT PRIMARY KEY,
	request_id TEXT NOT NULL UNIQUE,
	variant_id TEXT NOT NULL,
	actor TEXT NOT NULL DEFAULT '',
	package_json TEXT NOT NULL,
	created_at TEXT NOT NULL,
	FOREIGN KEY(request_id) REFERENCES material_requests(id),
	FOREIGN KEY(variant_id) REFERENCES copy_variants(id)
);

CREATE INDEX IF NOT EXISTS idx_deliveries_request
	ON deliveries(request_id);
`

const authUsers010SQL = `
CREATE TABLE IF NOT EXISTS users (
	id TEXT PRIMARY KEY,
	username TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	role TEXT NOT NULL CHECK(role IN ('operator','client')),
	display_name TEXT NOT NULL DEFAULT '',
	active INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS auth_tokens (
	token TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	created_at TEXT NOT NULL,
	expires_at TEXT NOT NULL,
	FOREIGN KEY(user_id) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_auth_tokens_user
	ON auth_tokens(user_id);
`

const dossierAssets011SQL = `
CREATE TABLE IF NOT EXISTS dossier_assets (
	id TEXT PRIMARY KEY,
	product_id TEXT NOT NULL,
	kind TEXT NOT NULL,
	url TEXT NOT NULL DEFAULT '',
	content TEXT NOT NULL DEFAULT '',
	source TEXT NOT NULL DEFAULT '',
	note TEXT NOT NULL DEFAULT '',
	created_by TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	FOREIGN KEY(product_id) REFERENCES products(id)
);

CREATE INDEX IF NOT EXISTS idx_dossier_assets_product
	ON dossier_assets(product_id);
`

const mediagenAssets012SQL = `
CREATE TABLE IF NOT EXISTS assets (
	id TEXT PRIMARY KEY,
	kind TEXT NOT NULL,
	backend TEXT NOT NULL DEFAULT 'local',
	uri TEXT NOT NULL,
	content_type TEXT NOT NULL DEFAULT 'image/png',
	byte_size INTEGER NOT NULL DEFAULT 0,
	checksum TEXT NOT NULL DEFAULT '',
	product_id TEXT,
	request_id TEXT,
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_assets_kind
	ON assets(kind);
CREATE INDEX IF NOT EXISTS idx_assets_product
	ON assets(product_id);

CREATE TABLE IF NOT EXISTS image_gen_tasks (
	id TEXT PRIMARY KEY,
	product_id TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'queued',
	usage TEXT NOT NULL DEFAULT '',
	style TEXT NOT NULL DEFAULT '',
	quality TEXT NOT NULL DEFAULT '',
	prompt TEXT NOT NULL DEFAULT '',
	seed INTEGER NOT NULL DEFAULT 0,
	width INTEGER NOT NULL DEFAULT 0,
	height INTEGER NOT NULL DEFAULT 0,
	comfy_prompt_id TEXT,
	asset_id TEXT,
	last_error TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	completed_at TEXT,
	FOREIGN KEY(product_id) REFERENCES products(id)
);

CREATE INDEX IF NOT EXISTS idx_image_gen_tasks_product
	ON image_gen_tasks(product_id);
`
