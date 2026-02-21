package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpen_CreatesTablesAndIndexes(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	tables := []string{"habits", "entries", "weekly_promotions"}
	for _, tbl := range tables {
		var name string
		err := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", tbl, err)
		}
	}

	indexes := []string{"idx_entries_habit_date", "idx_habits_archived"}
	for _, idx := range indexes {
		var name string
		err := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='index' AND name=?`, idx,
		).Scan(&name)
		if err != nil {
			t.Errorf("index %q not found: %v", idx, err)
		}
	}
}

func TestOpen_IsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	db1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	db1.Close()

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	db2.Close()
}

func TestOpen_CreatesDirectoryIfMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "dir")
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("expected directory to be created")
	}
}

func TestOpen_ForeignKeysEnabled(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	var fkEnabled int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled); err != nil {
		t.Fatalf("PRAGMA foreign_keys: %v", err)
	}
	if fkEnabled != 1 {
		t.Errorf("foreign_keys = %d, want 1", fkEnabled)
	}
}

func TestOpen_MaxConnsOne(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	stats := db.Stats()
	if stats.MaxOpenConnections != 1 {
		t.Errorf("MaxOpenConnections = %d, want 1", stats.MaxOpenConnections)
	}
}

func TestOpen_ForeignKeyConstraintEnforced(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`INSERT INTO entries (habit_id, entry_date, did_it) VALUES (999, '2024-01-01', 1)`)
	if err == nil {
		t.Error("expected foreign key constraint error, got nil")
	}
}

func TestDefaultPath_WithXDGDataHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)

	path := DefaultPath()
	if !strings.HasPrefix(path, dir) {
		t.Errorf("DefaultPath() = %q, want prefix %q", path, dir)
	}
	if !strings.HasSuffix(path, filepath.Join("hbt", "hbt.db")) {
		t.Errorf("DefaultPath() = %q, want suffix hbt/hbt.db", path)
	}
}

func TestDefaultPath_WithoutXDGDataHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")

	path := DefaultPath()
	if !strings.Contains(path, ".local") || !strings.Contains(path, "share") {
		t.Errorf("DefaultPath() = %q, expected ~/.local/share based path", path)
	}
	if !strings.HasSuffix(path, filepath.Join("hbt", "hbt.db")) {
		t.Errorf("DefaultPath() = %q, want suffix hbt/hbt.db", path)
	}
}
