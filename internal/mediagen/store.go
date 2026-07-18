package mediagen

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tiktok-trend-shop/internal/id"
)

// defaultStorageRoot is used when no storage root is configured.
const defaultStorageRoot = "./assets"

// AssetStore persists generated files under a content-addressed local root
// and indexes them in the assets table.
type AssetStore struct {
	db   *sql.DB
	root string
}

// NewAssetStore builds a store writing files under root ("./assets" when
// empty). The root is served publicly by the /assets/ static route.
func NewAssetStore(db *sql.DB, root string) *AssetStore {
	if root == "" {
		root = defaultStorageRoot
	}
	return &AssetStore{db: db, root: root}
}

// Root reports the storage root directory.
func (s *AssetStore) Root() string { return s.root }

// Save writes data to {root}/{kind}/{digest[:12]}.png (sha256 content
// addressing) and records the asset row. productID may be empty (stored as
// NULL); meta lands in metadata_json. The returned Asset carries the public
// uri /assets/{kind}/{digest[:12]}.png.
func (s *AssetStore) Save(kind string, data []byte, productID string, meta map[string]any) (Asset, error) {
	if kind == "" || strings.Contains(kind, "..") || strings.ContainsAny(kind, `/\`) {
		return Asset{}, fmt.Errorf("mediagen: invalid asset kind %q", kind)
	}
	digest := sha256.Sum256(data)
	checksum := hex.EncodeToString(digest[:])
	filename := checksum[:12] + ".png"
	dir := filepath.Join(s.root, kind)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Asset{}, fmt.Errorf("mediagen: create asset dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, filename), data, 0o644); err != nil {
		return Asset{}, fmt.Errorf("mediagen: write asset file: %w", err)
	}
	metadata, err := marshalObject(meta)
	if err != nil {
		return Asset{}, err
	}
	assetID := id.New("asset")
	uri := "/assets/" + kind + "/" + filename
	_, err = s.db.ExecContext(context.Background(), `
		INSERT INTO assets(
			id, kind, backend, uri, content_type, byte_size, checksum,
			product_id, request_id, metadata_json, created_at
		)
		VALUES (?, ?, 'local', ?, 'image/png', ?, ?, ?, NULL, ?, ?)
	`, assetID, kind, uri, len(data), checksum, nullableString(productID), metadata, utcNow())
	if err != nil {
		return Asset{}, fmt.Errorf("mediagen: record asset: %w", err)
	}
	return s.Get(context.Background(), assetID)
}

// Get returns one asset by id, or sql.ErrNoRows when it does not exist.
func (s *AssetStore) Get(ctx context.Context, assetID string) (Asset, error) {
	row := s.db.QueryRowContext(ctx, assetSelectColumns+` WHERE id = ?`, assetID)
	return scanAsset(row)
}

// ListByProduct returns the assets attached to a product, newest first.
func (s *AssetStore) ListByProduct(ctx context.Context, productID string) ([]Asset, error) {
	rows, err := s.db.QueryContext(ctx, assetSelectColumns+`
		WHERE product_id = ?
		ORDER BY created_at DESC, rowid DESC
	`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []Asset{}
	for rows.Next() {
		asset, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, asset)
	}
	return results, rows.Err()
}

const assetSelectColumns = `
	SELECT id, kind, backend, uri, content_type, byte_size, checksum,
		product_id, request_id, metadata_json, created_at
	FROM assets
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAsset(row rowScanner) (Asset, error) {
	var (
		asset     Asset
		productID sql.NullString
		requestID sql.NullString
		metadata  string
	)
	if err := row.Scan(
		&asset.ID,
		&asset.Kind,
		&asset.Backend,
		&asset.URI,
		&asset.ContentType,
		&asset.ByteSize,
		&asset.Checksum,
		&productID,
		&requestID,
		&metadata,
		&asset.CreatedAt,
	); err != nil {
		return Asset{}, err
	}
	asset.ProductID = nullStringPtr(productID)
	asset.RequestID = nullStringPtr(requestID)
	asset.Metadata = unmarshalMap(metadata)
	return asset, nil
}

func utcNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func marshalObject(value map[string]any) (string, error) {
	if value == nil {
		value = map[string]any{}
	}
	data, err := json.Marshal(value)
	return string(data), err
}

func unmarshalMap(value string) map[string]any {
	if value == "" {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(value), &out); err != nil {
		return map[string]any{}
	}
	return out
}
