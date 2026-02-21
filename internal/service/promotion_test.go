package service

import (
	"testing"
	"time"
)

func TestNeedsWeeklyPromotion_NoHistory(t *testing.T) {
	database := openTestDB(t)
	// Add a non-obligated habit so there's something to promote.
	_, err := CreateHabit(database, "Walk", d("2024-01-01"), false, nil)
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}

	today := d("2024-01-08") // Monday
	got, err := NeedsWeeklyPromotion(database, today)
	if err != nil {
		t.Fatalf("NeedsWeeklyPromotion: %v", err)
	}
	if !got {
		t.Error("expected NeedsWeeklyPromotion=true when no history exists")
	}
}

func TestNeedsWeeklyPromotion_PromotedThisWeek(t *testing.T) {
	database := openTestDB(t)
	_, err := CreateHabit(database, "Walk", d("2024-01-01"), false, nil)
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}

	today := d("2024-01-10") // Wednesday
	if err := RecordWeeklyPromotion(database, today); err != nil {
		t.Fatalf("RecordWeeklyPromotion: %v", err)
	}

	got, err := NeedsWeeklyPromotion(database, today)
	if err != nil {
		t.Fatalf("NeedsWeeklyPromotion: %v", err)
	}
	if got {
		t.Error("expected NeedsWeeklyPromotion=false when already promoted this week")
	}
}

func TestNeedsWeeklyPromotion_PromotedPreviousWeek(t *testing.T) {
	database := openTestDB(t)
	_, err := CreateHabit(database, "Walk", d("2024-01-01"), false, nil)
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}

	lastWeek := d("2024-01-03") // Wednesday of previous week
	if err := RecordWeeklyPromotion(database, lastWeek); err != nil {
		t.Fatalf("RecordWeeklyPromotion: %v", err)
	}

	thisWeek := d("2024-01-10") // Wednesday of this week
	got, err := NeedsWeeklyPromotion(database, thisWeek)
	if err != nil {
		t.Fatalf("NeedsWeeklyPromotion: %v", err)
	}
	if !got {
		t.Error("expected NeedsWeeklyPromotion=true when only previous week is recorded")
	}
}

func TestNeedsWeeklyPromotion_NoNonObligatedHabits(t *testing.T) {
	database := openTestDB(t)
	// Only add an obligated habit — nothing to promote.
	_, err := CreateHabit(database, "Walk", d("2024-01-01"), true, nil)
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}

	today := d("2024-01-08")
	got, err := NeedsWeeklyPromotion(database, today)
	if err != nil {
		t.Fatalf("NeedsWeeklyPromotion: %v", err)
	}
	if got {
		t.Error("expected NeedsWeeklyPromotion=false when no non-obligated habits exist")
	}
}

func TestRecordWeeklyPromotion_Idempotent(t *testing.T) {
	database := openTestDB(t)
	today := d("2024-01-10")

	if err := RecordWeeklyPromotion(database, today); err != nil {
		t.Fatalf("first RecordWeeklyPromotion: %v", err)
	}
	if err := RecordWeeklyPromotion(database, today); err != nil {
		t.Fatalf("second RecordWeeklyPromotion (should be idempotent): %v", err)
	}
}

func TestRecordWeeklyPromotion_NormalizesToMonday(t *testing.T) {
	database := openTestDB(t)
	_, err := CreateHabit(database, "Walk", d("2024-01-01"), false, nil)
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}

	// Record on Wednesday Jan 10; should store Monday Jan 8.
	wednesday := d("2024-01-10")
	if err := RecordWeeklyPromotion(database, wednesday); err != nil {
		t.Fatalf("RecordWeeklyPromotion: %v", err)
	}

	// Check from Monday's perspective — should see it as already done.
	monday := d("2024-01-08")
	got, err := NeedsWeeklyPromotion(database, monday)
	if err != nil {
		t.Fatalf("NeedsWeeklyPromotion: %v", err)
	}
	if got {
		t.Error("expected NeedsWeeklyPromotion=false: recording on Wednesday should cover the whole week")
	}
}

func TestListNonObligatedHabits_ExcludesObligatedAndArchived(t *testing.T) {
	database := openTestDB(t)

	since := time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC)
	_, _ = CreateHabit(database, "Obligated", d("2024-01-01"), true, &since)
	h2, _ := CreateHabit(database, "NonObl1", d("2024-01-02"), false, nil)
	h3, _ := CreateHabit(database, "NonObl2", d("2024-01-03"), false, nil)
	_ = ArchiveHabit(database, h3.ID, "done")
	_, _ = CreateHabit(database, "NonObl3", d("2024-01-04"), false, nil)

	habits, err := ListNonObligatedHabits(database)
	if err != nil {
		t.Fatalf("ListNonObligatedHabits: %v", err)
	}

	if len(habits) != 2 {
		t.Fatalf("expected 2 non-obligated non-archived habits, got %d", len(habits))
	}
	names := map[int64]string{}
	for _, h := range habits {
		names[h.ID] = h.Name
	}
	if names[h2.ID] != "NonObl1" {
		t.Errorf("expected NonObl1 in results, got %v", names)
	}
	for _, h := range habits {
		if h.IsObligated {
			t.Errorf("expected only non-obligated habits, got %q (obligated)", h.Name)
		}
		if h.Archived {
			t.Errorf("expected only non-archived habits, got %q (archived)", h.Name)
		}
	}
}
