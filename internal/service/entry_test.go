package service

import (
	"testing"
	"time"
)

func TestRecordEntry_NewEntry(t *testing.T) {
	database := openTestDB(t)
	h, _ := CreateHabit(database, "Run", d("2024-01-01"), false, nil)

	err := RecordEntry(database, h.ID, d("2024-01-08"), true)
	if err != nil {
		t.Fatalf("RecordEntry: %v", err)
	}

	e, err := GetEntryForDay(database, h.ID, d("2024-01-08"))
	if err != nil {
		t.Fatalf("GetEntryForDay: %v", err)
	}
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if !e.DidIt {
		t.Error("DidIt should be true")
	}
	if e.HabitID != h.ID {
		t.Errorf("HabitID = %d, want %d", e.HabitID, h.ID)
	}
}

func TestRecordEntry_UpsertOverwritesExisting(t *testing.T) {
	database := openTestDB(t)
	h, _ := CreateHabit(database, "Run", d("2024-01-01"), false, nil)
	date := d("2024-01-08")

	// First record as done
	_ = RecordEntry(database, h.ID, date, true)
	// Overwrite as skipped
	err := RecordEntry(database, h.ID, date, false)
	if err != nil {
		t.Fatalf("RecordEntry upsert: %v", err)
	}

	e, _ := GetEntryForDay(database, h.ID, date)
	if e == nil {
		t.Fatal("expected entry after upsert")
	}
	if e.DidIt {
		t.Error("DidIt should be false after overwrite")
	}
}

func TestRecordEntry_UpsertFromSkipToDone(t *testing.T) {
	database := openTestDB(t)
	h, _ := CreateHabit(database, "Run", d("2024-01-01"), false, nil)
	date := d("2024-01-08")

	_ = RecordEntry(database, h.ID, date, false)
	_ = RecordEntry(database, h.ID, date, true)

	e, _ := GetEntryForDay(database, h.ID, date)
	if e == nil || !e.DidIt {
		t.Error("expected DidIt=true after overwriting skip with done")
	}
}

func TestGetEntryForDay_NoEntry_ReturnsNil(t *testing.T) {
	database := openTestDB(t)
	h, _ := CreateHabit(database, "Run", d("2024-01-01"), false, nil)

	e, err := GetEntryForDay(database, h.ID, d("2024-01-08"))
	if err != nil {
		t.Fatalf("GetEntryForDay: %v", err)
	}
	if e != nil {
		t.Errorf("expected nil for missing entry, got %+v", e)
	}
}

func TestGetEntryForDay_CorrectHabitIsolation(t *testing.T) {
	database := openTestDB(t)
	h1, _ := CreateHabit(database, "Run", d("2024-01-01"), false, nil)
	h2, _ := CreateHabit(database, "Walk", d("2024-01-01"), false, nil)
	date := d("2024-01-08")

	_ = RecordEntry(database, h1.ID, date, true)

	e, err := GetEntryForDay(database, h2.ID, date)
	if err != nil {
		t.Fatalf("GetEntryForDay: %v", err)
	}
	if e != nil {
		t.Error("expected nil for h2 which has no entry on that date")
	}
}

func TestGetEntriesForHabit_Empty(t *testing.T) {
	database := openTestDB(t)
	h, _ := CreateHabit(database, "Run", d("2024-01-01"), false, nil)

	entries, err := GetEntriesForHabit(database, h.ID)
	if err != nil {
		t.Fatalf("GetEntriesForHabit: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty, got %d entries", len(entries))
	}
}

func TestGetEntriesForHabit_OrderedByDate(t *testing.T) {
	database := openTestDB(t)
	h, _ := CreateHabit(database, "Run", d("2024-01-01"), false, nil)

	// Insert out of order
	for _, dt := range []string{"2024-01-10", "2024-01-08", "2024-01-12", "2024-01-09"} {
		_ = RecordEntry(database, h.ID, d(dt), true)
	}

	entries, err := GetEntriesForHabit(database, h.ID)
	if err != nil {
		t.Fatalf("GetEntriesForHabit: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}
	for i := 1; i < len(entries); i++ {
		if !entries[i-1].EntryDate.Before(entries[i].EntryDate) {
			t.Errorf("entries not sorted: %v >= %v", entries[i-1].EntryDate, entries[i].EntryDate)
		}
	}
}

func TestGetPendingBackfill_NoHabits(t *testing.T) {
	database := openTestDB(t)
	items, err := GetPendingBackfill(database, d("2024-01-15"))
	if err != nil {
		t.Fatalf("GetPendingBackfill: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty backfill, got %d items", len(items))
	}
}

func TestGetPendingBackfill_HabitWithNoEntries_Excluded(t *testing.T) {
	database := openTestDB(t)
	_, _ = CreateHabit(database, "New Habit", d("2024-01-01"), false, nil)

	items, err := GetPendingBackfill(database, d("2024-01-15"))
	if err != nil {
		t.Fatalf("GetPendingBackfill: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("habit with no entries should not generate backfill, got %d items", len(items))
	}
}

func TestGetPendingBackfill_AllDaysCovered_Empty(t *testing.T) {
	database := openTestDB(t)
	h, _ := CreateHabit(database, "Run", d("2024-01-08"), false, nil)
	today := d("2024-01-10")

	_ = RecordEntry(database, h.ID, d("2024-01-08"), true)
	_ = RecordEntry(database, h.ID, d("2024-01-09"), true)

	items, err := GetPendingBackfill(database, today)
	if err != nil {
		t.Fatalf("GetPendingBackfill: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("all days covered, expected 0 backfill items, got %d", len(items))
	}
}

func TestGetPendingBackfill_SingleGap(t *testing.T) {
	database := openTestDB(t)
	h, _ := CreateHabit(database, "Run", d("2024-01-08"), false, nil)
	today := d("2024-01-11")

	_ = RecordEntry(database, h.ID, d("2024-01-08"), true)
	// gap on Jan 9
	_ = RecordEntry(database, h.ID, d("2024-01-10"), true)

	items, err := GetPendingBackfill(database, today)
	if err != nil {
		t.Fatalf("GetPendingBackfill: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 backfill item, got %d", len(items))
	}
	if !items[0].Date.Equal(d("2024-01-09")) {
		t.Errorf("backfill date = %v, want 2024-01-09", items[0].Date)
	}
	if items[0].Habit.ID != h.ID {
		t.Errorf("habit ID = %d, want %d", items[0].Habit.ID, h.ID)
	}
}

func TestGetPendingBackfill_MultipleGaps(t *testing.T) {
	database := openTestDB(t)
	h, _ := CreateHabit(database, "Run", d("2024-01-08"), false, nil)
	today := d("2024-01-13")

	// Entry on Jan 8 and Jan 12 — gaps on Jan 9, 10, 11
	_ = RecordEntry(database, h.ID, d("2024-01-08"), true)
	_ = RecordEntry(database, h.ID, d("2024-01-12"), true)

	items, err := GetPendingBackfill(database, today)
	if err != nil {
		t.Fatalf("GetPendingBackfill: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 backfill items, got %d", len(items))
	}
	for i, wantDate := range []string{"2024-01-09", "2024-01-10", "2024-01-11"} {
		if !items[i].Date.Equal(d(wantDate)) {
			t.Errorf("item[%d].Date = %v, want %s", i, items[i].Date, wantDate)
		}
	}
}

func TestGetPendingBackfill_TodayNotIncluded(t *testing.T) {
	database := openTestDB(t)
	h, _ := CreateHabit(database, "Run", d("2024-01-08"), false, nil)
	today := d("2024-01-09")

	_ = RecordEntry(database, h.ID, d("2024-01-08"), true)
	// No entry today

	items, err := GetPendingBackfill(database, today)
	if err != nil {
		t.Fatalf("GetPendingBackfill: %v", err)
	}
	for _, item := range items {
		if item.Date.Equal(today) {
			t.Errorf("today (%v) should not appear in backfill", today)
		}
	}
}

func TestGetPendingBackfill_ArchivedHabit_Excluded(t *testing.T) {
	database := openTestDB(t)
	h, _ := CreateHabit(database, "Run", d("2024-01-08"), false, nil)
	today := d("2024-01-11")

	_ = RecordEntry(database, h.ID, d("2024-01-08"), true)
	_ = ArchiveHabit(database, h.ID, "done")

	items, err := GetPendingBackfill(database, today)
	if err != nil {
		t.Fatalf("GetPendingBackfill: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("archived habit should be excluded from backfill, got %d items", len(items))
	}
}

func TestGetPendingBackfill_MultipleHabits(t *testing.T) {
	database := openTestDB(t)
	h1, _ := CreateHabit(database, "Run", d("2024-01-08"), false, nil)
	h2, _ := CreateHabit(database, "Walk", d("2024-01-08"), false, nil)
	today := d("2024-01-11")

	_ = RecordEntry(database, h1.ID, d("2024-01-08"), true)
	// h1 gaps: Jan 9, Jan 10

	_ = RecordEntry(database, h2.ID, d("2024-01-08"), true)
	_ = RecordEntry(database, h2.ID, d("2024-01-09"), true)
	// h2 gap: Jan 10

	items, err := GetPendingBackfill(database, today)
	if err != nil {
		t.Fatalf("GetPendingBackfill: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 total backfill items (2+1), got %d", len(items))
	}
}

func TestGetPendingBackfill_DaysBeforeFirstEntry_NotIncluded(t *testing.T) {
	database := openTestDB(t)
	// Habit starts Jan 1, first entry on Jan 10 — should not backfill Jan 1-9.
	h, _ := CreateHabit(database, "Run", d("2024-01-01"), false, nil)
	today := d("2024-01-11")

	_ = RecordEntry(database, h.ID, d("2024-01-10"), true)

	items, err := GetPendingBackfill(database, today)
	if err != nil {
		t.Fatalf("GetPendingBackfill: %v", err)
	}
	for _, item := range items {
		if item.Date.Before(d("2024-01-10")) {
			t.Errorf("backfill includes date %v which is before first entry", item.Date)
		}
	}
}

func TestGetPendingBackfill_EntryOnlyOnTodayExcluded(t *testing.T) {
	database := openTestDB(t)
	h, _ := CreateHabit(database, "Run", d("2024-01-08"), false, nil)
	today := d("2024-01-09")

	// Only entry is today
	_ = RecordEntry(database, h.ID, today, true)

	items, err := GetPendingBackfill(database, today)
	if err != nil {
		t.Fatalf("GetPendingBackfill: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("entry only on today should not create backfill, got %d items", len(items))
	}
}

func TestScanEntry_DatePreserved(t *testing.T) {
	database := openTestDB(t)
	h, _ := CreateHabit(database, "Run", d("2024-01-01"), false, nil)
	date := d("2024-06-15")

	_ = RecordEntry(database, h.ID, date, true)

	e, err := GetEntryForDay(database, h.ID, date)
	if err != nil {
		t.Fatalf("GetEntryForDay: %v", err)
	}
	if e == nil {
		t.Fatal("expected entry")
	}
	if !e.EntryDate.Equal(date) {
		t.Errorf("EntryDate = %v, want %v", e.EntryDate, date)
	}
}

func TestRecordEntry_DateNormalizedToMidnight(t *testing.T) {
	database := openTestDB(t)
	h, _ := CreateHabit(database, "Run", d("2024-01-01"), false, nil)

	// Store with a non-midnight time; should still be retrievable via midnight date
	nonMidnight := time.Date(2024, 6, 15, 23, 59, 59, 0, time.UTC)
	_ = RecordEntry(database, h.ID, nonMidnight, true)

	// fmtDate truncates to YYYY-MM-DD, so retrieve with midnight should work
	e, _ := GetEntryForDay(database, h.ID, d("2024-06-15"))
	if e == nil {
		t.Fatal("expected to retrieve entry using midnight date after storing non-midnight time")
	}
}
