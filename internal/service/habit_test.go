package service

import (
	"database/sql"
	"hbt/internal/db"
	"path/filepath"
	"testing"
	"time"
)

// openTestDB creates a temporary SQLite database for testing.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("openTestDB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestCreateHabit_NotObligated(t *testing.T) {
	database := openTestDB(t)
	start := d("2024-01-08")

	h, err := CreateHabit(database, "Morning run", start, false, nil)
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}

	if h.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if h.Name != "Morning run" {
		t.Errorf("Name = %q, want %q", h.Name, "Morning run")
	}
	if !h.StartDate.Equal(start) {
		t.Errorf("StartDate = %v, want %v", h.StartDate, start)
	}
	if h.IsObligated {
		t.Error("IsObligated should be false")
	}
	if h.ObligatedSinceDate != nil {
		t.Errorf("ObligatedSinceDate should be nil, got %v", h.ObligatedSinceDate)
	}
	if h.Archived {
		t.Error("Archived should be false")
	}
}

func TestCreateHabit_Obligated(t *testing.T) {
	database := openTestDB(t)
	start := d("2024-01-01")
	since := d("2024-01-08")

	h, err := CreateHabit(database, "Exercise", start, true, &since)
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}

	if !h.IsObligated {
		t.Error("IsObligated should be true")
	}
	if h.ObligatedSinceDate == nil {
		t.Fatal("ObligatedSinceDate should not be nil")
	}
	if !h.ObligatedSinceDate.Equal(since) {
		t.Errorf("ObligatedSinceDate = %v, want %v", h.ObligatedSinceDate, since)
	}
}

func TestCreateHabit_ObligatedWithoutSinceDate(t *testing.T) {
	database := openTestDB(t)
	h, err := CreateHabit(database, "Meditate", d("2024-01-01"), true, nil)
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}
	if h.ObligatedSinceDate != nil {
		t.Errorf("ObligatedSinceDate should be nil when not provided, got %v", h.ObligatedSinceDate)
	}
}

func TestGetHabit_Existing(t *testing.T) {
	database := openTestDB(t)
	created, _ := CreateHabit(database, "Read", d("2024-01-01"), false, nil)

	got, err := GetHabit(database, created.ID)
	if err != nil {
		t.Fatalf("GetHabit: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID = %d, want %d", got.ID, created.ID)
	}
	if got.Name != created.Name {
		t.Errorf("Name = %q, want %q", got.Name, created.Name)
	}
}

func TestGetHabit_NonExistent(t *testing.T) {
	database := openTestDB(t)
	_, err := GetHabit(database, 9999)
	if err == nil {
		t.Error("expected error for non-existent habit, got nil")
	}
}

func TestListHabits_Empty(t *testing.T) {
	database := openTestDB(t)
	habits, err := ListHabits(database, false)
	if err != nil {
		t.Fatalf("ListHabits: %v", err)
	}
	if len(habits) != 0 {
		t.Errorf("expected empty list, got %d habits", len(habits))
	}
}

func TestListHabits_ObligatedFirst(t *testing.T) {
	database := openTestDB(t)
	since := d("2024-01-01")

	// Create: optional first, then obligated
	_, _ = CreateHabit(database, "Optional", d("2023-12-01"), false, nil)
	_, _ = CreateHabit(database, "Obligated", d("2023-11-01"), true, &since)

	habits, err := ListHabits(database, false)
	if err != nil {
		t.Fatalf("ListHabits: %v", err)
	}
	if len(habits) != 2 {
		t.Fatalf("expected 2 habits, got %d", len(habits))
	}
	if !habits[0].IsObligated {
		t.Errorf("first habit should be obligated, got name=%q", habits[0].Name)
	}
	if habits[1].IsObligated {
		t.Errorf("second habit should not be obligated, got name=%q", habits[1].Name)
	}
}

func TestListHabits_MultipleObligatedSortedBySinceDate(t *testing.T) {
	database := openTestDB(t)
	later := d("2024-02-01")
	earlier := d("2024-01-01")

	_, _ = CreateHabit(database, "Later OBL", d("2023-01-01"), true, &later)
	_, _ = CreateHabit(database, "Earlier OBL", d("2023-01-01"), true, &earlier)

	habits, err := ListHabits(database, false)
	if err != nil {
		t.Fatalf("ListHabits: %v", err)
	}
	if habits[0].Name != "Earlier OBL" {
		t.Errorf("first obligated = %q, want 'Earlier OBL'", habits[0].Name)
	}
}

func TestListHabits_ExcludesArchivedByDefault(t *testing.T) {
	database := openTestDB(t)
	h, _ := CreateHabit(database, "Active", d("2024-01-01"), false, nil)
	archived, _ := CreateHabit(database, "Archived", d("2024-01-01"), false, nil)
	_ = ArchiveHabit(database, archived.ID, "done")

	habits, err := ListHabits(database, false)
	if err != nil {
		t.Fatalf("ListHabits: %v", err)
	}
	if len(habits) != 1 {
		t.Fatalf("expected 1 habit (non-archived), got %d", len(habits))
	}
	if habits[0].ID != h.ID {
		t.Errorf("expected active habit, got %q", habits[0].Name)
	}
}

func TestListHabits_IncludesArchivedWhenRequested(t *testing.T) {
	database := openTestDB(t)
	_, _ = CreateHabit(database, "Active", d("2024-01-01"), false, nil)
	archived, _ := CreateHabit(database, "Archived", d("2024-01-01"), false, nil)
	_ = ArchiveHabit(database, archived.ID, "done")

	habits, err := ListHabits(database, true)
	if err != nil {
		t.Fatalf("ListHabits: %v", err)
	}
	if len(habits) != 2 {
		t.Fatalf("expected 2 habits, got %d", len(habits))
	}
}

func TestArchiveHabit_SetsArchivedAndComment(t *testing.T) {
	database := openTestDB(t)
	h, _ := CreateHabit(database, "Sleep early", d("2024-01-01"), false, nil)

	err := ArchiveHabit(database, h.ID, "No longer needed")
	if err != nil {
		t.Fatalf("ArchiveHabit: %v", err)
	}

	got, _ := GetHabit(database, h.ID)
	if !got.Archived {
		t.Error("Archived should be true")
	}
	if got.ArchiveComment != "No longer needed" {
		t.Errorf("ArchiveComment = %q, want %q", got.ArchiveComment, "No longer needed")
	}
}

func TestArchiveHabit_EmptyComment(t *testing.T) {
	database := openTestDB(t)
	h, _ := CreateHabit(database, "Test", d("2024-01-01"), false, nil)

	if err := ArchiveHabit(database, h.ID, ""); err != nil {
		t.Fatalf("ArchiveHabit: %v", err)
	}
	got, _ := GetHabit(database, h.ID)
	if !got.Archived {
		t.Error("Archived should be true")
	}
}

func TestSetObligated_SetToObligated(t *testing.T) {
	database := openTestDB(t)
	h, _ := CreateHabit(database, "Walk", d("2024-01-01"), false, nil)
	since := d("2024-03-01")

	if err := SetObligated(database, h.ID, true, since); err != nil {
		t.Fatalf("SetObligated: %v", err)
	}

	got, _ := GetHabit(database, h.ID)
	if !got.IsObligated {
		t.Error("IsObligated should be true after SetObligated")
	}
	if got.ObligatedSinceDate == nil {
		t.Fatal("ObligatedSinceDate should not be nil")
	}
	if !got.ObligatedSinceDate.Equal(since) {
		t.Errorf("ObligatedSinceDate = %v, want %v", got.ObligatedSinceDate, since)
	}
}

func TestSetObligated_SetToNotObligated(t *testing.T) {
	database := openTestDB(t)
	since := d("2024-01-01")
	h, _ := CreateHabit(database, "Walk", d("2023-01-01"), true, &since)

	if err := SetObligated(database, h.ID, false, time.Time{}); err != nil {
		t.Fatalf("SetObligated: %v", err)
	}

	got, _ := GetHabit(database, h.ID)
	if got.IsObligated {
		t.Error("IsObligated should be false after SetObligated(false)")
	}
}

func TestParseDate_ValidDate(t *testing.T) {
	t.Parallel()
	got, err := parseDate("2024-06-15")
	if err != nil {
		t.Fatalf("parseDate: %v", err)
	}
	want := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("parseDate = %v, want %v", got, want)
	}
}

func TestParseDate_InvalidDate(t *testing.T) {
	t.Parallel()
	_, err := parseDate("not-a-date")
	if err == nil {
		t.Error("expected error for invalid date, got nil")
	}
}

func TestFmtDate_RoundTrip(t *testing.T) {
	t.Parallel()
	original := "2024-06-15"
	ts, _ := parseDate(original)
	if got := fmtDate(ts); got != original {
		t.Errorf("fmtDate(parseDate(%q)) = %q, want %q", original, got, original)
	}
}
