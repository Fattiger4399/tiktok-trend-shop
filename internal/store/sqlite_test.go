package store

import (
	"path/filepath"
	"testing"
)

func TestMigrateCreatesCoreTables(t *testing.T) {
	db, err := Open("sqlite:///" + filepath.Join(t.TempDir(), "test.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"products", "source_snapshots", "product_detail_snapshots"} {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?",
			table,
		).Scan(&name)
		if err != nil {
			t.Fatalf("missing table %s: %v", table, err)
		}
	}
}
