package web

import (
	"database/sql"
	"fmt"
	"hbt/internal/db"
	"hbt/internal/model"
	"hbt/internal/service"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("openTestDB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func newTestServer(t *testing.T, database *sql.DB) *Server {
	t.Helper()
	srv, err := NewServer(database)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv
}

func d(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func itoa(n int64) string {
	return fmt.Sprintf("%d", n)
}

// --- Index page tests ---

func TestIndex_EmptyState(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "No habits yet") {
		t.Error("expected 'No habits yet' in empty state")
	}
}

func TestIndex_ShowsHabitName(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	service.CreateHabit(database, "Push-ups", d("2025-01-06"), true, nil)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if !strings.Contains(w.Body.String(), "Push-ups") {
		t.Error("expected habit name in response")
	}
}

func TestIndex_ShowsNoHistory(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	service.CreateHabit(database, "Meditation", d("2025-01-06"), true, nil)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if !strings.Contains(w.Body.String(), "(no history)") {
		t.Error("expected '(no history)' for habit with no entries")
	}
}

func TestIndex_UncheckedHighlighting_PastDays(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	// Create a habit that started 3 days ago with one entry (creates a gap)
	startDate := time.Now().UTC().AddDate(0, 0, -3)
	startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, time.UTC)
	h, _ := service.CreateHabit(database, "Running", startDate, true, nil)
	service.RecordEntry(database, h.ID, startDate, true) // only day 1

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "day-unchecked") {
		t.Error("expected day-unchecked class for days with gaps")
	}
	if !strings.Contains(body, "sq-unchecked") {
		t.Error("expected sq-unchecked class for gray cells with gaps")
	}
}

func TestIndex_UncheckedHighlighting_TodayExcluded(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	// Habit started today, no entries → today is unknown but should NOT be unchecked
	td := today()
	h, _ := service.CreateHabit(database, "NewHabit", td, true, nil)
	service.RecordEntry(database, h.ID, td, true) // has history but only today

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, "day-unchecked") {
		t.Error("did not expect day-unchecked when only today exists")
	}
	if strings.Contains(body, "sq-unchecked") {
		t.Error("did not expect sq-unchecked when only today exists")
	}
}

func TestIndex_UncheckedHighlighting_AllFilled(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	// Habit started 2 days ago, all days filled
	td := today()
	startDate := td.AddDate(0, 0, -2)
	h, _ := service.CreateHabit(database, "FilledHabit", startDate, true, nil)
	service.RecordEntry(database, h.ID, startDate, true)
	service.RecordEntry(database, h.ID, startDate.AddDate(0, 0, 1), true)
	service.RecordEntry(database, h.ID, td, true)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, "day-unchecked") {
		t.Error("did not expect day-unchecked when all days are filled")
	}
	if strings.Contains(body, "sq-unchecked") {
		t.Error("did not expect sq-unchecked when all days are filled")
	}
}

func TestIndex_UncheckedHighlighting_InactiveIgnored(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	// Habit with last entry >42 days ago → inactive, should not produce unchecked
	h, _ := service.CreateHabit(database, "OldHabit", d("2025-01-01"), true, nil)
	service.RecordEntry(database, h.ID, d("2025-01-01"), true)
	// No other entries, but habit is inactive (>42 days since last entry)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, "sq-unchecked") {
		t.Error("did not expect sq-unchecked for inactive habit")
	}
}

func TestIndex_ShowsPromotionBanner(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	// Create a non-obligated habit
	service.CreateHabit(database, "Drawing", d("2025-01-06"), false, nil)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if !strings.Contains(w.Body.String(), "WEEKLY PROMOTION") {
		t.Error("expected promotion banner for non-obligated habit")
	}
}

func TestIndex_NormalMode(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	// Only obligated habit (start today → no backfill) + promotion already recorded
	td := today()
	service.CreateHabit(database, "Reading", td, true, nil)
	service.RecordWeeklyPromotion(database, td)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, "WEEKLY PROMOTION") {
		t.Error("did not expect promotion banner")
	}
	if strings.Contains(body, "day-unchecked") {
		t.Error("did not expect unchecked highlighting")
	}
	if !strings.Contains(body, "Reading") {
		t.Error("expected habit name")
	}
}

// --- Entry recording tests ---

func TestRecordEntry_Done(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	h, _ := service.CreateHabit(database, "Coding", d("2025-01-06"), true, nil)

	req := httptest.NewRequest("POST", "/habits/"+itoa(h.ID)+"/entry?did_it=true", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	td := today()
	e, _ := service.GetEntryForDay(database, h.ID, td)
	if e == nil {
		t.Fatal("expected entry to be created")
	}
	if !e.DidIt {
		t.Error("expected did_it=true")
	}
}

func TestRecordEntry_Skip(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	h, _ := service.CreateHabit(database, "Coding", d("2025-01-06"), true, nil)

	req := httptest.NewRequest("POST", "/habits/"+itoa(h.ID)+"/entry?did_it=false", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	td := today()
	e, _ := service.GetEntryForDay(database, h.ID, td)
	if e == nil {
		t.Fatal("expected entry to be created")
	}
	if e.DidIt {
		t.Error("expected did_it=false")
	}
}

func TestRecordEntry_ReturnsUpdatedRow(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	h, _ := service.CreateHabit(database, "Yoga", d("2025-01-06"), true, nil)

	req := httptest.NewRequest("POST", "/habits/"+itoa(h.ID)+"/entry?did_it=true", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Yoga") {
		t.Error("expected updated habit row with name")
	}
	if !strings.Contains(body, "sq-green") {
		t.Error("expected green square in updated row")
	}
}

// --- Promotion tests ---

func TestPromote_SetsObligated(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	h, _ := service.CreateHabit(database, "Piano", d("2025-01-06"), false, nil)

	req := httptest.NewRequest("POST", "/promotion/"+itoa(h.ID)+"/confirm", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}

	updated, _ := service.GetHabit(database, h.ID)
	if !updated.IsObligated {
		t.Error("expected habit to be obligated after promotion")
	}
}

func TestSkipPromotion_RecordsWeekly(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	service.CreateHabit(database, "Painting", d("2025-01-06"), false, nil)

	req := httptest.NewRequest("POST", "/promotion/skip", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}

	// Verify promotion was recorded (no longer needed)
	needs, _ := service.NeedsWeeklyPromotion(database, time.Now().UTC())
	if needs {
		t.Error("expected promotion to be recorded (NeedsWeeklyPromotion should be false)")
	}
}

// --- Create habit tests ---

func TestCreateHabit_Valid(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	form := url.Values{
		"name":       {"Swimming"},
		"start_date": {"2025-02-01"},
	}
	req := httptest.NewRequest("POST", "/habits", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}

	habits, _ := service.ListHabits(database, false)
	if len(habits) != 1 {
		t.Fatalf("expected 1 habit, got %d", len(habits))
	}
	if habits[0].Name != "Swimming" {
		t.Errorf("name = %q, want %q", habits[0].Name, "Swimming")
	}
}

func TestCreateHabit_Obligated(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	form := url.Values{
		"name":            {"Gym"},
		"start_date":      {"2025-02-01"},
		"is_obligated":    {"1"},
		"obligated_since": {"2025-02-10"},
	}
	req := httptest.NewRequest("POST", "/habits", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}

	habits, _ := service.ListHabits(database, false)
	if len(habits) != 1 {
		t.Fatalf("expected 1 habit, got %d", len(habits))
	}
	if !habits[0].IsObligated {
		t.Error("expected obligated")
	}
	if habits[0].ObligatedSinceDate == nil {
		t.Fatal("expected ObligatedSinceDate")
	}
	if habits[0].ObligatedSinceDate.Format("2006-01-02") != "2025-02-10" {
		t.Errorf("ObligatedSinceDate = %v, want 2025-02-10", habits[0].ObligatedSinceDate)
	}
}

func TestCreateHabit_EmptyName(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	form := url.Values{
		"name":       {""},
		"start_date": {"2025-02-01"},
	}
	req := httptest.NewRequest("POST", "/habits", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 422 {
		t.Fatalf("status = %d, want 422", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Name is required") {
		t.Error("expected error message about name")
	}
}

func TestCreateHabit_InvalidDate(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	form := url.Values{
		"name":       {"Test"},
		"start_date": {"not-a-date"},
	}
	req := httptest.NewRequest("POST", "/habits", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 422 {
		t.Fatalf("status = %d, want 422", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Invalid start date") {
		t.Error("expected error message about date")
	}
}

// --- Archive tests ---

func TestArchiveHabit(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	h, _ := service.CreateHabit(database, "Walking", d("2025-01-06"), true, nil)

	form := url.Values{"comment": {"no time"}}
	req := httptest.NewRequest("POST", "/habits/"+itoa(h.ID)+"/archive", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}

	updated, _ := service.GetHabit(database, h.ID)
	if !updated.Archived {
		t.Error("expected habit to be archived")
	}
	if updated.ArchiveComment != "no time" {
		t.Errorf("comment = %q, want %q", updated.ArchiveComment, "no time")
	}
}

func TestArchiveHabit_EmptyComment(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	h, _ := service.CreateHabit(database, "Walking", d("2025-01-06"), true, nil)

	form := url.Values{"comment": {""}}
	req := httptest.NewRequest("POST", "/habits/"+itoa(h.ID)+"/archive", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}

	updated, _ := service.GetHabit(database, h.ID)
	if !updated.Archived {
		t.Error("expected habit to be archived")
	}
}

// --- Stats tests ---

func TestStats_ShowsHabitStats(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	h, _ := service.CreateHabit(database, "Cycling", d("2025-01-06"), true, nil)
	service.RecordEntry(database, h.ID, d("2025-01-06"), true)

	req := httptest.NewRequest("GET", "/stats", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Cycling") {
		t.Error("expected habit name in stats page")
	}
	if !strings.Contains(body, "sq-green") {
		t.Error("expected green square for done entry")
	}
}

func TestStats_NoHabits(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	req := httptest.NewRequest("GET", "/stats", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "No habits") {
		t.Error("expected 'No habits' message on empty stats page")
	}
}

// --- Helper tests ---

func TestFmtPct(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{1.0, "100%"},
		{0.5, "50%"},
		{0.0, "0%"},
		{0.753, "75%"},
	}
	for _, tt := range tests {
		got := fmtPct(tt.in)
		if got != tt.want {
			t.Errorf("fmtPct(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSquareClass(t *testing.T) {
	tests := []struct {
		in   model.DayStatus
		want string
	}{
		{model.DayDone, "sq-green"},
		{model.DayYellow, "sq-yellow"},
		{model.DayRed, "sq-red"},
		{model.DayFuture, "sq-future"},
		{model.DayUnknown, "sq-gray"},
	}
	for _, tt := range tests {
		got := squareClass(tt.in)
		if got != tt.want {
			t.Errorf("squareClass(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// --- Archive form test ---

func TestArchiveForm_ShowsHabitName(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	h, _ := service.CreateHabit(database, "Cooking", d("2025-01-06"), true, nil)

	req := httptest.NewRequest("GET", "/habits/"+itoa(h.ID)+"/archive", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Cooking") {
		t.Error("expected habit name in archive form")
	}
}

// --- Promotion confirm tests ---

func TestPromotionConfirm_ShowsHabitName(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	h, _ := service.CreateHabit(database, "Drawing", d("2025-01-06"), false, nil)

	req := httptest.NewRequest("GET", "/promotion/"+itoa(h.ID)+"/confirm", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Drawing") {
		t.Error("expected habit name in promotion confirm page")
	}
	if !strings.Contains(body, "Confirm Promotion") {
		t.Error("expected 'Confirm Promotion' button")
	}
}

// --- Archived page tests ---

func TestArchived_EmptyState(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	req := httptest.NewRequest("GET", "/archived", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "No archived habits") {
		t.Error("expected 'No archived habits' message")
	}
}

func TestArchived_ShowsArchivedHabit(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	h, _ := service.CreateHabit(database, "Walking", d("2025-01-06"), true, nil)
	service.ArchiveHabit(database, h.ID, "no time")

	req := httptest.NewRequest("GET", "/archived", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Walking") {
		t.Error("expected archived habit name")
	}
	if !strings.Contains(body, "no time") {
		t.Error("expected archive comment")
	}
}

// --- Add habit form test ---

func TestAddHabitForm_Renders(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	req := httptest.NewRequest("GET", "/habits/new", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Add Habit") {
		t.Error("expected 'Add Habit' title")
	}
}

// --- buildHabitRow tests ---

func TestBuildHabitRow_NoHistory(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	h, _ := service.CreateHabit(database, "Meditation", d("2025-02-03"), true, nil)

	row := srv.buildHabitRow(h, d("2025-02-03"))

	if row.HasHistory {
		t.Error("expected HasHistory=false")
	}
	if row.DisplayCols != 28 {
		t.Errorf("DisplayCols = %d, want 28", row.DisplayCols)
	}
	if len(row.Squares) != 0 {
		t.Errorf("len(Squares) = %d, want 0", len(row.Squares))
	}
}

func TestBuildHabitRow_Inactive(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	h, _ := service.CreateHabit(database, "Running", d("2025-01-06"), true, nil)
	service.RecordEntry(database, h.ID, d("2025-01-06"), true)

	row := srv.buildHabitRow(h, d("2025-03-15")) // 68 days later → >42 threshold

	if !row.Inactive {
		t.Error("expected Inactive=true")
	}
	if !strings.Contains(row.InactiveMsg, "inactive") {
		t.Errorf("InactiveMsg = %q, want to contain 'inactive'", row.InactiveMsg)
	}
	if row.RollingGreen != 0 {
		t.Errorf("RollingGreen = %d, want 0", row.RollingGreen)
	}
	if row.RollingRed != 0 {
		t.Errorf("RollingRed = %d, want 0", row.RollingRed)
	}
	if row.RollingRate != -1 {
		t.Errorf("RollingRate = %f, want -1", row.RollingRate)
	}
}

func TestBuildHabitRow_RollingWindow(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	h, _ := service.CreateHabit(database, "Exercise", d("2025-01-06"), true, nil)
	service.RecordEntry(database, h.ID, d("2025-01-06"), true)  // before 28-day window
	service.RecordEntry(database, h.ID, d("2025-01-14"), true)  // in window → green
	service.RecordEntry(database, h.ID, d("2025-01-15"), false) // consecutive skip → red
	service.RecordEntry(database, h.ID, d("2025-01-16"), false) // consecutive skip → red

	td := d("2025-02-10")
	row := srv.buildHabitRow(h, td)

	if row.RollingGreen != 1 {
		t.Errorf("RollingGreen = %d, want 1", row.RollingGreen)
	}
	if row.RollingRed != 2 {
		t.Errorf("RollingRed = %d, want 2", row.RollingRed)
	}
	// Rate = 1 / (1+2) ≈ 0.333
	if row.RollingRate < 0.33 || row.RollingRate > 0.34 {
		t.Errorf("RollingRate = %f, want ~0.333", row.RollingRate)
	}
	// Squares from Jan 6 to Feb 10 = 36 days
	if len(row.Squares) != 36 {
		t.Errorf("len(Squares) = %d, want 36", len(row.Squares))
	}
	if row.DisplayCols != 36 {
		t.Errorf("DisplayCols = %d, want 36", row.DisplayCols)
	}
}

func TestBuildHabitRow_DisplayColsMin28(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	h, _ := service.CreateHabit(database, "Yoga", d("2025-02-05"), true, nil)
	service.RecordEntry(database, h.ID, d("2025-02-05"), true)

	row := srv.buildHabitRow(h, d("2025-02-10"))

	// Squares from Feb 5 to Feb 10 = 6 days
	if len(row.Squares) != 6 {
		t.Errorf("len(Squares) = %d, want 6", len(row.Squares))
	}
	if row.DisplayCols != 28 {
		t.Errorf("DisplayCols = %d, want 28", row.DisplayCols)
	}
}

// --- computeRollingData tests ---

func TestComputeRollingData_EmptyRows(t *testing.T) {
	rd := computeRollingData(d("2025-02-10"), []habitRowData{})

	if rd.Label != "Last 4 Weeks" {
		t.Errorf("Label = %q, want %q", rd.Label, "Last 4 Weeks")
	}
	if len(rd.DayLabels) != 28 {
		t.Errorf("len(DayLabels) = %d, want 28", len(rd.DayLabels))
	}
	if rd.OverallPct != -1 {
		t.Errorf("OverallPct = %f, want -1", rd.OverallPct)
	}
	if rd.TotalGreen != 0 {
		t.Errorf("TotalGreen = %d, want 0", rd.TotalGreen)
	}

	totalDays := 0
	for _, wg := range rd.WeekGroups {
		totalDays += len(wg.Days)
	}
	if totalDays != 28 {
		t.Errorf("total week group days = %d, want 28", totalDays)
	}
}

func TestComputeRollingData_WeekGroups(t *testing.T) {
	rd := computeRollingData(d("2025-02-12"), []habitRowData{}) // Wednesday

	// Last week group's last day should be "12" (Feb 12)
	lastWG := rd.WeekGroups[len(rd.WeekGroups)-1]
	lastDay := lastWG.Days[len(lastWG.Days)-1]
	if lastDay.Num != "12" {
		t.Errorf("last day = %q, want %q", lastDay.Num, "12")
	}

	// First week group's label should start with "Jan"
	if !strings.HasPrefix(rd.WeekGroups[0].Label, "Jan") {
		t.Errorf("first week group label = %q, want prefix 'Jan'", rd.WeekGroups[0].Label)
	}

	// Sum of all week group days should be 28
	totalDays := 0
	for _, wg := range rd.WeekGroups {
		totalDays += len(wg.Days)
	}
	if totalDays != 28 {
		t.Errorf("total days = %d, want 28", totalDays)
	}
}

func TestComputeRollingData_UncheckedDates(t *testing.T) {
	td := d("2025-02-10")

	row := habitRowData{
		HasHistory: true,
		Inactive:   false,
		Squares: []daySquare{
			{Status: model.DayDone, Date: d("2025-02-08"), IsToday: false},
			{Status: model.DayUnknown, Date: d("2025-02-09"), IsToday: false},  // unchecked
			{Status: model.DayUnknown, Date: d("2025-02-10"), IsToday: true},   // today — excluded
		},
	}
	rd := computeRollingData(td, []habitRowData{row})

	if !rd.UncheckedDates["2025-02-09"] {
		t.Error("expected 2025-02-09 to be unchecked")
	}
	if rd.UncheckedDates["2025-02-10"] {
		t.Error("did not expect today (2025-02-10) to be unchecked")
	}
	if rd.UncheckedDates["2025-02-08"] {
		t.Error("did not expect 2025-02-08 (done) to be unchecked")
	}

	// Verify header days have Unchecked set
	found := false
	for _, wg := range rd.WeekGroups {
		for _, hd := range wg.Days {
			if hd.Num == "9" && hd.Unchecked {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected header day '9' to have Unchecked=true")
	}
}

func TestComputeRollingData_FiltersInactive(t *testing.T) {
	td := d("2025-02-10")

	activeRow := habitRowData{
		HasHistory:   true,
		Inactive:     false,
		RollingGreen: 1,
		Squares:      []daySquare{{Status: model.DayDone, Date: td}},
	}
	inactiveRow := habitRowData{
		HasHistory:   true,
		Inactive:     true,
		RollingGreen: 0, // inactive habits have 0 rolling stats from buildHabitRow
		Squares:      []daySquare{{Status: model.DayDone, Date: td}},
	}
	noHistoryRow := habitRowData{
		HasHistory: false,
	}

	rows := []habitRowData{activeRow, inactiveRow, noHistoryRow}
	rd := computeRollingData(td, rows)

	if rd.TotalGreen != 1 {
		t.Errorf("TotalGreen = %d, want 1", rd.TotalGreen)
	}
	// Last daily percentage = 100% (only active row counted in per-day stats)
	lastPct := rd.DailyPcts[len(rd.DailyPcts)-1]
	if lastPct != 100.0 {
		t.Errorf("last DailyPcts = %f, want 100.0", lastPct)
	}
}

// --- computeDisplayCols tests ---

func TestComputeDisplayCols_NoHabits(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	cols := srv.computeDisplayCols(d("2025-02-10"))
	if cols != 28 {
		t.Errorf("computeDisplayCols = %d, want 28", cols)
	}
}

func TestComputeDisplayCols_OldHabit(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	service.CreateHabit(database, "Running", d("2024-01-01"), true, nil)

	cols := srv.computeDisplayCols(d("2025-02-10"))
	if cols <= 400 {
		t.Errorf("computeDisplayCols = %d, want >400", cols)
	}
}

// --- handleToggleEntry tests ---

func TestToggleEntry_NilToDone(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	h, _ := service.CreateHabit(database, "Yoga", d("2025-01-06"), true, nil)

	req := httptest.NewRequest("POST", "/habits/"+itoa(h.ID)+"/entry/toggle?date=2025-01-10", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	e, _ := service.GetEntryForDay(database, h.ID, d("2025-01-10"))
	if e == nil {
		t.Fatal("expected entry to be created")
	}
	if !e.DidIt {
		t.Error("expected did_it=true")
	}
	if !strings.Contains(w.Body.String(), "Yoga") {
		t.Error("expected habit name in response body")
	}
}

func TestToggleEntry_DoneToSkip(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	h, _ := service.CreateHabit(database, "Yoga", d("2025-01-06"), true, nil)
	service.RecordEntry(database, h.ID, d("2025-01-10"), true)

	req := httptest.NewRequest("POST", "/habits/"+itoa(h.ID)+"/entry/toggle?date=2025-01-10", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	e, _ := service.GetEntryForDay(database, h.ID, d("2025-01-10"))
	if e == nil {
		t.Fatal("expected entry to exist")
	}
	if e.DidIt {
		t.Error("expected did_it=false")
	}
}

func TestToggleEntry_SkipToDelete(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	h, _ := service.CreateHabit(database, "Yoga", d("2025-01-06"), true, nil)
	service.RecordEntry(database, h.ID, d("2025-01-10"), false)

	req := httptest.NewRequest("POST", "/habits/"+itoa(h.ID)+"/entry/toggle?date=2025-01-10", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	e, _ := service.GetEntryForDay(database, h.ID, d("2025-01-10"))
	if e != nil {
		t.Error("expected entry to be deleted (nil)")
	}
}

func TestToggleEntry_InvalidDate(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	h, _ := service.CreateHabit(database, "Yoga", d("2025-01-06"), true, nil)

	req := httptest.NewRequest("POST", "/habits/"+itoa(h.ID)+"/entry/toggle?date=not-a-date", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "invalid date") {
		t.Error("expected 'invalid date' error message")
	}
}

func TestToggleEntry_InvalidID(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	req := httptest.NewRequest("POST", "/habits/abc/entry/toggle?date=2025-01-10", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "invalid id") {
		t.Error("expected 'invalid id' error message")
	}
}
