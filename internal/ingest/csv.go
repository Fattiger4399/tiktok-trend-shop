// Package ingest contains the CSV importer used by the CLI. The bulk of the
// import logic now lives in internal/importer; this package re-exports a thin
// adapter that the CLI commands call.
package ingest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"tiktok-trend-shop/internal/importer"
	"tiktok-trend-shop/internal/product"
)

// ImportResult mirrors the shape produced by the CLI for backward compatibility.
type ImportResult = importer.Result

// Importer is the CLI-facing entry point.
type Importer struct {
	inner *importer.Importer
}

// NewImporter returns a CSV importer bound to the product repository.
func NewImporter(repo *product.Repository) *Importer {
	return &Importer{inner: importer.New(repo.DB(), repo)}
}

// ImportCSV imports a CSV file from disk using the shared importer.
func (i *Importer) ImportCSV(ctx context.Context, path, defaultRegion string) (ImportResult, error) {
	if _, err := os.Stat(path); err != nil {
		return ImportResult{}, fmt.Errorf("stat csv: %w", err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ImportResult{}, err
	}
	return i.inner.ImportFromPath(ctx, abs, defaultRegion, "manual-csv")
}