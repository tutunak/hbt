package tui

import (
	"database/sql"
	"hbt/internal/db"
	"hbt/internal/model"
	"hbt/internal/service"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ---- helpers ----

func tuiTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("tuiTestDB: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func td(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		panic("bad date: " + s)
	}
	return t
}

// key constructs a simple rune key message.
func key(r rune) tea.Msg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// keyStr constructs a named key message (e.g. "enter", "esc").
func keyStr(s string) tea.Msg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// runCmd executes a tea.Cmd and returns the resulting message (or nil).
func runCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

// ---- BackfillModel tests ----

func TestBackfillModel_SKey_SkipsWithoutRecording(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-10")
	h, _ := service.CreateHabit(database, "Run", td("2024-01-08"), false, nil)
	_ = service.RecordEntry(database, h.ID, td("2024-01-08"), true)

	items := []model.BackfillItem{
		{Habit: h, Date: td("2024-01-09")},
	}
	m := newBackfillModel(database, items, today)

	updated, cmd := m.Update(key('s'))
	bm := updated.(BackfillModel)

	// Should advance to end
	if bm.index != 1 {
		t.Errorf("index = %d, want 1 after skip", bm.index)
	}

	// No entry should have been recorded
	e, _ := service.GetEntryForDay(database, h.ID, td("2024-01-09"))
	if e != nil {
		t.Error("skip should not record an entry")
	}

	// Should emit switchToHabitListMsg
	msg := runCmd(cmd)
	if _, ok := msg.(switchToHabitListMsg); !ok {
		t.Errorf("expected switchToHabitListMsg after last item, got %T", msg)
	}
}

func TestBackfillModel_YKey_RecordsDoneAndAdvances(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-10")
	h, _ := service.CreateHabit(database, "Run", td("2024-01-08"), false, nil)
	_ = service.RecordEntry(database, h.ID, td("2024-01-08"), true)

	items := []model.BackfillItem{
		{Habit: h, Date: td("2024-01-09")},
	}
	m := newBackfillModel(database, items, today)

	updated, cmd := m.Update(key('y'))
	bm := updated.(BackfillModel)

	if bm.index != 1 {
		t.Errorf("index = %d, want 1 after yes", bm.index)
	}

	e, _ := service.GetEntryForDay(database, h.ID, td("2024-01-09"))
	if e == nil || !e.DidIt {
		t.Error("y key should record entry as done")
	}

	msg := runCmd(cmd)
	if _, ok := msg.(switchToHabitListMsg); !ok {
		t.Errorf("expected switchToHabitListMsg, got %T", msg)
	}
}

func TestBackfillModel_NKey_RecordsSkippedAndAdvances(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-10")
	h, _ := service.CreateHabit(database, "Run", td("2024-01-08"), false, nil)
	_ = service.RecordEntry(database, h.ID, td("2024-01-08"), true)

	items := []model.BackfillItem{
		{Habit: h, Date: td("2024-01-09")},
	}
	m := newBackfillModel(database, items, today)

	updated, _ := m.Update(key('n'))
	bm := updated.(BackfillModel)

	e, _ := service.GetEntryForDay(database, h.ID, td("2024-01-09"))
	if e == nil || e.DidIt {
		t.Error("n key should record entry as skipped (did_it=false)")
	}
	_ = bm
}

func TestBackfillModel_MultipleItems_IteratesThroughAll(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-12")
	h, _ := service.CreateHabit(database, "Run", td("2024-01-08"), false, nil)
	_ = service.RecordEntry(database, h.ID, td("2024-01-08"), true)

	items := []model.BackfillItem{
		{Habit: h, Date: td("2024-01-09")},
		{Habit: h, Date: td("2024-01-10")},
		{Habit: h, Date: td("2024-01-11")},
	}
	m := newBackfillModel(database, items, today)

	// Answer 'y' to first two
	updated, _ := m.Update(key('y'))
	m = updated.(BackfillModel)
	updated, _ = m.Update(key('y'))
	m = updated.(BackfillModel)

	if m.index != 2 {
		t.Errorf("index = %d after 2 answers, want 2", m.index)
	}

	// Answer last one
	_, cmd := m.Update(key('s'))
	msg := runCmd(cmd)
	if _, ok := msg.(switchToHabitListMsg); !ok {
		t.Errorf("expected switchToHabitListMsg after all items, got %T", msg)
	}
}

func TestBackfillModel_View_ShowsHabitNameAndDate(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-10")
	h, _ := service.CreateHabit(database, "Morning run", td("2024-01-08"), false, nil)
	_ = service.RecordEntry(database, h.ID, td("2024-01-08"), true)

	items := []model.BackfillItem{
		{Habit: h, Date: td("2024-01-09")},
	}
	m := newBackfillModel(database, items, today)

	view := m.View()
	if !strings.Contains(view, "Morning run") {
		t.Errorf("view should contain habit name, got: %s", view)
	}
	if !strings.Contains(view, "Jan 9") {
		t.Errorf("view should contain date, got: %s", view)
	}
}

func TestBackfillModel_View_ShowsProgress(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-11")
	h, _ := service.CreateHabit(database, "Run", td("2024-01-08"), false, nil)
	_ = service.RecordEntry(database, h.ID, td("2024-01-08"), true)

	items := []model.BackfillItem{
		{Habit: h, Date: td("2024-01-09")},
		{Habit: h, Date: td("2024-01-10")},
	}
	m := newBackfillModel(database, items, today)

	view := m.View()
	if !strings.Contains(view, "1 of 2") {
		t.Errorf("view should show progress, got: %s", view)
	}
}

// ---- TrackModel tests ----

func TestTrackModel_YKey_RecordsDone(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-08")
	h, _ := service.CreateHabit(database, "Run", today, false, nil)

	m := newTrackModel(database, h, today)

	updated, _ := m.Update(key('y'))
	tm := updated.(TrackModel)

	if !tm.done {
		t.Error("done flag should be set after y key")
	}

	e, _ := service.GetEntryForDay(database, h.ID, today)
	if e == nil || !e.DidIt {
		t.Error("y key should record entry as done")
	}
}

func TestTrackModel_EnterKey_RecordsDone(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-08")
	h, _ := service.CreateHabit(database, "Run", today, false, nil)

	m := newTrackModel(database, h, today)
	updated, _ := m.Update(keyStr("enter"))
	tm := updated.(TrackModel)

	if !tm.done {
		t.Error("done flag should be set after enter key")
	}
	e, _ := service.GetEntryForDay(database, h.ID, today)
	if e == nil || !e.DidIt {
		t.Error("enter key should record entry as done")
	}
}

func TestTrackModel_NKey_RecordsSkipped(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-08")
	h, _ := service.CreateHabit(database, "Run", today, false, nil)

	m := newTrackModel(database, h, today)
	updated, _ := m.Update(key('n'))
	tm := updated.(TrackModel)

	if !tm.done {
		t.Error("done flag should be set after n key")
	}
	e, _ := service.GetEntryForDay(database, h.ID, today)
	if e == nil || e.DidIt {
		t.Error("n key should record entry as skipped")
	}
}

func TestTrackModel_EscKey_NavigatesBack(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-08")
	h, _ := service.CreateHabit(database, "Run", today, false, nil)

	m := newTrackModel(database, h, today)
	_, cmd := m.Update(keyStr("esc"))

	msg := runCmd(cmd)
	if _, ok := msg.(switchToHabitListMsg); !ok {
		t.Errorf("esc should emit switchToHabitListMsg, got %T", msg)
	}
}

func TestTrackModel_ShowsExistingEntryStatus(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-08")
	h, _ := service.CreateHabit(database, "Run", today, false, nil)
	_ = service.RecordEntry(database, h.ID, today, true)

	m := newTrackModel(database, h, today)

	if m.current == nil {
		t.Error("current should be set when an entry exists for today")
	}
	if !m.current.DidIt {
		t.Error("current.DidIt should be true")
	}
}

func TestTrackModel_View_ContainsHabitName(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-08")
	h, _ := service.CreateHabit(database, "Drink water", today, false, nil)

	m := newTrackModel(database, h, today)
	view := m.View()

	if !strings.Contains(view, "Drink water") {
		t.Errorf("view should contain habit name, got: %s", view)
	}
}

// ---- AddHabitModel tests ----

func TestAddHabitModel_EmptyName_ShowsError(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-08")
	m := newAddHabitModel(database, today)

	// Don't type anything, press enter
	updated, _ := m.Update(keyStr("enter"))
	am := updated.(AddHabitModel)

	if am.errMsg == "" {
		t.Error("expected error for empty name")
	}
	if am.step != addStepName {
		t.Errorf("step = %v, should remain at addStepName after validation error", am.step)
	}
}

func TestAddHabitModel_EscKey_NavigatesBack(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-08")
	m := newAddHabitModel(database, today)

	_, cmd := m.Update(keyStr("esc"))
	msg := runCmd(cmd)
	if _, ok := msg.(switchToHabitListMsg); !ok {
		t.Errorf("esc should emit switchToHabitListMsg, got %T", msg)
	}
}

func TestAddHabitModel_InvalidDateFormat_ShowsError(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-08")
	m := newAddHabitModel(database, today)

	// Set a valid name
	m.nameInput.SetValue("Test habit")
	// Advance to date step manually
	updated, _ := m.Update(keyStr("enter"))
	am := updated.(AddHabitModel)
	if am.step != addStepStartDate {
		t.Fatalf("expected addStepStartDate, got %v", am.step)
	}

	// Set an invalid date
	am.dateInput.SetValue("not-a-date")
	updated, _ = am.Update(keyStr("enter"))
	am = updated.(AddHabitModel)

	if am.errMsg == "" {
		t.Error("expected error for invalid date format")
	}
	if am.step != addStepStartDate {
		t.Errorf("step should remain at addStepStartDate after validation error, got %v", am.step)
	}
}

func TestAddHabitModel_InvalidObligated_ShowsError(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-08")
	m := newAddHabitModel(database, today)

	// Get through name and date steps
	m.nameInput.SetValue("Test habit")
	updated, _ := m.Update(keyStr("enter"))
	am := updated.(AddHabitModel)

	am.dateInput.SetValue("") // empty = use today
	updated, _ = am.Update(keyStr("enter"))
	am = updated.(AddHabitModel)

	if am.step != addStepObligated {
		t.Fatalf("expected addStepObligated, got %v", am.step)
	}

	// Enter invalid value
	am.oblInput.SetValue("maybe")
	updated, _ = am.Update(keyStr("enter"))
	am = updated.(AddHabitModel)

	if am.errMsg == "" {
		t.Error("expected error for invalid y/n input")
	}
}

func TestAddHabitModel_CompleteFlow_NotObligated(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-08")
	m := newAddHabitModel(database, today)

	// Step 1: name
	m.nameInput.SetValue("Evening walk")
	updated, _ := m.Update(keyStr("enter"))
	am := updated.(AddHabitModel)

	// Step 2: date (empty = today)
	updated, _ = am.Update(keyStr("enter"))
	am = updated.(AddHabitModel)

	// Step 3: not obligated
	am.oblInput.SetValue("n")
	updated, cmd := am.Update(keyStr("enter"))
	am = updated.(AddHabitModel)

	if am.step != addStepDone {
		t.Errorf("step = %v, want addStepDone after completing form", am.step)
	}

	msg := runCmd(cmd)
	if _, ok := msg.(switchToHabitListMsg); !ok {
		t.Errorf("expected switchToHabitListMsg after save, got %T", msg)
	}

	// Verify habit was created
	habits, _ := service.ListHabits(database, false)
	if len(habits) != 1 {
		t.Fatalf("expected 1 habit, got %d", len(habits))
	}
	if habits[0].Name != "Evening walk" {
		t.Errorf("habit name = %q, want 'Evening walk'", habits[0].Name)
	}
	if habits[0].IsObligated {
		t.Error("habit should not be obligated")
	}
}

func TestAddHabitModel_CompleteFlow_Obligated(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-08")
	m := newAddHabitModel(database, today)

	m.nameInput.SetValue("Meditate")
	updated, _ := m.Update(keyStr("enter"))
	am := updated.(AddHabitModel)

	updated, _ = am.Update(keyStr("enter")) // empty date = today
	am = updated.(AddHabitModel)

	am.oblInput.SetValue("y")
	updated, _ = am.Update(keyStr("enter"))
	am = updated.(AddHabitModel)

	if am.step != addStepObligatedSince {
		t.Fatalf("step = %v, want addStepObligatedSince", am.step)
	}

	// Enter obligated-since date
	am.oblSince.SetValue("2024-01-01")
	updated, cmd := am.Update(keyStr("enter"))
	am = updated.(AddHabitModel)

	msg := runCmd(cmd)
	if _, ok := msg.(switchToHabitListMsg); !ok {
		t.Errorf("expected switchToHabitListMsg, got %T", msg)
	}

	habits, _ := service.ListHabits(database, false)
	if len(habits) != 1 {
		t.Fatalf("expected 1 habit, got %d", len(habits))
	}
	if !habits[0].IsObligated {
		t.Error("habit should be obligated")
	}
	if habits[0].ObligatedSinceDate == nil {
		t.Error("ObligatedSinceDate should be set")
	}
}

func TestAddHabitModel_ValidDate_Accepted(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-08")
	m := newAddHabitModel(database, today)

	m.nameInput.SetValue("Run")
	updated, _ := m.Update(keyStr("enter"))
	am := updated.(AddHabitModel)

	am.dateInput.SetValue("2023-12-01")
	updated, _ = am.Update(keyStr("enter"))
	am = updated.(AddHabitModel)

	if am.step != addStepObligated {
		t.Errorf("valid date should advance to obligated step, got step=%v", am.step)
	}
}

// ---- ArchiveModel tests ----

func TestArchiveModel_EnterKey_Archives(t *testing.T) {
	database := tuiTestDB(t)
	h, _ := service.CreateHabit(database, "Old habit", td("2024-01-01"), false, nil)

	m := newArchiveModel(database, h)
	m.input.SetValue("Completed this challenge")

	updated, cmd := m.Update(keyStr("enter"))
	am := updated.(ArchiveModel)

	if !am.done {
		t.Error("done should be true after archiving")
	}

	msg := runCmd(cmd)
	if _, ok := msg.(switchToHabitListMsg); !ok {
		t.Errorf("expected switchToHabitListMsg, got %T", msg)
	}

	got, _ := service.GetHabit(database, h.ID)
	if !got.Archived {
		t.Error("habit should be archived")
	}
	if got.ArchiveComment != "Completed this challenge" {
		t.Errorf("comment = %q, want 'Completed this challenge'", got.ArchiveComment)
	}
}

func TestArchiveModel_EmptyComment_StillArchives(t *testing.T) {
	database := tuiTestDB(t)
	h, _ := service.CreateHabit(database, "Old habit", td("2024-01-01"), false, nil)

	m := newArchiveModel(database, h)
	// No comment entered, press enter
	_, _ = m.Update(keyStr("enter"))

	got, _ := service.GetHabit(database, h.ID)
	if !got.Archived {
		t.Error("habit should be archived even with empty comment")
	}
}

func TestArchiveModel_EscKey_NavigatesBack(t *testing.T) {
	database := tuiTestDB(t)
	h, _ := service.CreateHabit(database, "Test", td("2024-01-01"), false, nil)

	m := newArchiveModel(database, h)
	_, cmd := m.Update(keyStr("esc"))

	msg := runCmd(cmd)
	if _, ok := msg.(switchToHabitListMsg); !ok {
		t.Errorf("esc should emit switchToHabitListMsg, got %T", msg)
	}

	// Habit should NOT be archived
	got, _ := service.GetHabit(database, h.ID)
	if got.Archived {
		t.Error("habit should NOT be archived after esc")
	}
}

func TestArchiveModel_View_ContainsHabitName(t *testing.T) {
	database := tuiTestDB(t)
	h, _ := service.CreateHabit(database, "Morning stretch", td("2024-01-01"), false, nil)

	m := newArchiveModel(database, h)
	view := m.View()

	if !strings.Contains(view, "Morning stretch") {
		t.Errorf("view should contain habit name, got: %s", view)
	}
}

// ---- StatsModel tests ----

func TestStatsModel_EscKey_NavigatesBack(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-08")

	m := newStatsModel(database, today)
	_, cmd := m.Update(keyStr("esc"))

	msg := runCmd(cmd)
	if _, ok := msg.(switchToHabitListMsg); !ok {
		t.Errorf("esc should emit switchToHabitListMsg, got %T", msg)
	}
}

func TestStatsModel_QKey_NavigatesBack(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-08")

	m := newStatsModel(database, today)
	_, cmd := m.Update(key('q'))

	msg := runCmd(cmd)
	if _, ok := msg.(switchToHabitListMsg); !ok {
		t.Errorf("q should emit switchToHabitListMsg, got %T", msg)
	}
}

func TestStatsModel_LoadedMsg_SetsContent(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-08")

	m := newStatsModel(database, today)
	m.width = 80
	m.height = 24
	updated, _ := m.Update(statsLoadedMsg{content: "test content"})
	sm := updated.(StatsModel)

	if !sm.ready {
		t.Error("ready should be true after statsLoadedMsg with valid dimensions")
	}
}

func TestStatsModel_BuildContent_NoHabits(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-08")

	m := newStatsModel(database, today)
	content := m.buildContent()

	if !strings.Contains(content, "No habits") {
		t.Errorf("expected 'No habits' message, got: %s", content)
	}
}

func TestStatsModel_BuildContent_WithHabit(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-08")
	h, _ := service.CreateHabit(database, "Morning run", td("2024-01-08"), false, nil)
	_ = service.RecordEntry(database, h.ID, td("2024-01-08"), true)

	m := newStatsModel(database, today)
	content := m.buildContent()

	if !strings.Contains(content, "Morning run") {
		t.Errorf("expected habit name in content, got: %s", content)
	}
}

// ---- HabitListModel tests ----

func TestHabitListModel_EmptyState_ShowsMessage(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-08")

	m := newHabitListModel(database, today)
	view := m.View()

	if !strings.Contains(view, "No habits") {
		t.Errorf("expected 'No habits' message for empty list, got: %s", view)
	}
}

func TestHabitListModel_AKey_EmitsAddHabitMsg(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-08")

	m := newHabitListModel(database, today)
	_, cmd := m.Update(key('a'))

	msg := runCmd(cmd)
	if _, ok := msg.(switchToAddHabitMsg); !ok {
		t.Errorf("a key should emit switchToAddHabitMsg, got %T", msg)
	}
}

func TestHabitListModel_SKey_EmitsStatsMsg(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-08")

	m := newHabitListModel(database, today)
	_, cmd := m.Update(key('s'))

	msg := runCmd(cmd)
	if _, ok := msg.(switchToStatsMsg); !ok {
		t.Errorf("s key should emit switchToStatsMsg, got %T", msg)
	}
}

func TestHabitListModel_QKey_Quits(t *testing.T) {
	database := tuiTestDB(t)
	today := td("2024-01-08")

	m := newHabitListModel(database, today)
	_, cmd := m.Update(key('q'))

	if cmd == nil {
		t.Fatal("q key should return a non-nil cmd")
	}
	// tea.Quit returns a specific command; check it's a quit command
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("q key should return tea.Quit command, got %T", msg)
	}
}

// ---- AppModel tests ----

func TestAppModel_New_NoPendingBackfill_StartsAtHabitList(t *testing.T) {
	database := tuiTestDB(t)

	app, err := New(database)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if app.screen != screenHabitList {
		t.Errorf("screen = %v, want screenHabitList when no backfill", app.screen)
	}
}

func TestAppModel_New_PendingBackfill_StartsAtBackfill(t *testing.T) {
	database := tuiTestDB(t)
	today := time.Now().UTC()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	yesterday := today.AddDate(0, 0, -1)

	h, _ := service.CreateHabit(database, "Run", yesterday.AddDate(0, 0, -1), false, nil)
	_ = service.RecordEntry(database, h.ID, yesterday.AddDate(0, 0, -1), true)
	// gap on yesterday — should trigger backfill

	app, err := New(database)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if app.screen != screenBackfill {
		t.Errorf("screen = %v, want screenBackfill when pending items exist", app.screen)
	}
}

func TestAppModel_CtrlC_Quits(t *testing.T) {
	database := tuiTestDB(t)
	app, _ := New(database)

	_, cmd := app.Update(keyStr("ctrl+c"))
	if cmd == nil {
		t.Fatal("ctrl+c should return non-nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("ctrl+c should quit, got %T", msg)
	}
}

func TestAppModel_WindowSizeMsg_UpdatesDimensions(t *testing.T) {
	database := tuiTestDB(t)
	app, _ := New(database)

	updated, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	am := updated.(AppModel)

	if am.width != 120 || am.height != 40 {
		t.Errorf("dimensions = %dx%d, want 120x40", am.width, am.height)
	}
}

func TestAppModel_SwitchToHabitList_UpdatesScreen(t *testing.T) {
	database := tuiTestDB(t)
	app, _ := New(database)
	app.screen = screenBackfill // force a different screen

	updated, _ := app.Update(switchToHabitListMsg{})
	am := updated.(AppModel)

	if am.screen != screenHabitList {
		t.Errorf("screen = %v, want screenHabitList after switchToHabitListMsg", am.screen)
	}
}

func TestAppModel_SwitchToAddHabit_UpdatesScreen(t *testing.T) {
	database := tuiTestDB(t)
	app, _ := New(database)

	updated, _ := app.Update(switchToAddHabitMsg{})
	am := updated.(AppModel)

	if am.screen != screenAddHabit {
		t.Errorf("screen = %v, want screenAddHabit", am.screen)
	}
}

func TestAppModel_SwitchToStats_UpdatesScreen(t *testing.T) {
	database := tuiTestDB(t)
	app, _ := New(database)

	updated, _ := app.Update(switchToStatsMsg{})
	am := updated.(AppModel)

	if am.screen != screenStats {
		t.Errorf("screen = %v, want screenStats", am.screen)
	}
}

// ---- renderSquare tests ----

func TestRenderSquare_Done_GreenSymbol(t *testing.T) {
	date := td("2024-01-08")
	result := renderSquare(model.DayDone, date, date)
	if !strings.Contains(result, "■") {
		t.Errorf("DayDone square should contain ■, got: %q", result)
	}
}

func TestRenderSquare_Yellow_YellowSymbol(t *testing.T) {
	date := td("2024-01-08")
	today := td("2024-01-09")
	result := renderSquare(model.DayYellow, date, today)
	if !strings.Contains(result, "▪") {
		t.Errorf("DayYellow square should contain ▪, got: %q", result)
	}
}

func TestRenderSquare_Red_RedSymbol(t *testing.T) {
	date := td("2024-01-08")
	today := td("2024-01-09")
	result := renderSquare(model.DayRed, date, today)
	if !strings.Contains(result, "□") {
		t.Errorf("DayRed square should contain □, got: %q", result)
	}
}

func TestRenderSquare_Unknown_Today_QuestionMark(t *testing.T) {
	today := td("2024-01-08")
	result := renderSquare(model.DayUnknown, today, today)
	if !strings.Contains(result, "?") {
		t.Errorf("DayUnknown for today should contain ?, got: %q", result)
	}
}

func TestRenderSquare_Unknown_PastDate_EmptyBrackets(t *testing.T) {
	date := td("2024-01-07")
	today := td("2024-01-08")
	result := renderSquare(model.DayUnknown, date, today)
	if !strings.Contains(result, "[ ]") {
		t.Errorf("DayUnknown for past date should contain '[ ]', got: %q", result)
	}
}

// ---- fmtPct tests ----

func TestFmtPct_100Percent(t *testing.T) {
	if got := fmtPct(1.0); got != "100%" {
		t.Errorf("fmtPct(1.0) = %q, want '100%%'", got)
	}
}

func TestFmtPct_0Percent(t *testing.T) {
	if got := fmtPct(0.0); got != "0%" {
		t.Errorf("fmtPct(0.0) = %q, want '0%%'", got)
	}
}

func TestFmtPct_50Percent(t *testing.T) {
	if got := fmtPct(0.5); got != "50%" {
		t.Errorf("fmtPct(0.5) = %q, want '50%%'", got)
	}
}

func TestFmtPct_RoundsCorrectly(t *testing.T) {
	// 6/7 ≈ 0.857... rounds to 86%
	rate := 6.0 / 7.0
	got := fmtPct(rate)
	if got != "86%" {
		t.Errorf("fmtPct(6/7) = %q, want '86%%'", got)
	}
}
