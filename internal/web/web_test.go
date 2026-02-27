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

func TestIndex_ShowsBackfillBanner(t *testing.T) {
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

	if !strings.Contains(w.Body.String(), "BACKFILL") {
		t.Error("expected backfill banner for habit with gap")
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
	if strings.Contains(body, "BACKFILL") {
		t.Error("did not expect backfill banner")
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

// --- Backfill tests ---

func TestBackfillAnswer_Yes(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	yesterday := today().AddDate(0, 0, -1)
	h, _ := service.CreateHabit(database, "Jogging", yesterday.AddDate(0, 0, -1), true, nil)
	// Record entry for start date so backfill triggers for yesterday
	service.RecordEntry(database, h.ID, yesterday.AddDate(0, 0, -1), true)

	dateStr := yesterday.Format("2006-01-02")
	req := httptest.NewRequest("POST", "/backfill/"+itoa(h.ID)+"/"+dateStr, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	e, _ := service.GetEntryForDay(database, h.ID, yesterday)
	if e == nil {
		t.Fatal("expected entry to be created")
	}
	if !e.DidIt {
		t.Error("expected did_it=true for backfill yes")
	}
}

func TestBackfillAnswer_No(t *testing.T) {
	database := openTestDB(t)
	srv := newTestServer(t, database)

	yesterday := today().AddDate(0, 0, -1)
	h, _ := service.CreateHabit(database, "Stretching", yesterday.AddDate(0, 0, -1), true, nil)
	service.RecordEntry(database, h.ID, yesterday.AddDate(0, 0, -1), true)

	dateStr := yesterday.Format("2006-01-02")
	req := httptest.NewRequest("POST", "/backfill/skip/"+itoa(h.ID)+"/"+dateStr, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	e, _ := service.GetEntryForDay(database, h.ID, yesterday)
	if e == nil {
		t.Fatal("expected entry to be created")
	}
	if e.DidIt {
		t.Error("expected did_it=false for backfill no")
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
