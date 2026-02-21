package service

import (
	"hbt/internal/model"
	"testing"
	"time"
)

// d is a shorthand for building a UTC midnight time from a string.
func d(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		panic("bad date in test: " + s)
	}
	return t
}

// makeHabit returns an obligated Habit (ObligatedSinceDate=nil → always strict rules).
func makeHabit(startDate string) model.Habit {
	return model.Habit{StartDate: d(startDate), IsObligated: true}
}

// makeNonObligatedHabit returns a non-obligated Habit (all skips are yellow).
func makeNonObligatedHabit(startDate string) model.Habit {
	return model.Habit{StartDate: d(startDate), IsObligated: false}
}

// entry returns a model.Entry for a given date with did_it set.
func entry(date string, didIt bool) model.Entry {
	return model.Entry{EntryDate: d(date), DidIt: didIt}
}

// ------- mondayOf -------

func TestMondayOf_Monday(t *testing.T) {
	mon := d("2024-01-08") // Jan 8 2024 = Monday
	got := mondayOf(mon)
	if !got.Equal(mon) {
		t.Errorf("mondayOf(Monday) = %v, want %v", got, mon)
	}
}

func TestMondayOf_Tuesday(t *testing.T) {
	tue := d("2024-01-09")
	want := d("2024-01-08")
	if got := mondayOf(tue); !got.Equal(want) {
		t.Errorf("mondayOf(Tuesday) = %v, want %v", got, want)
	}
}

func TestMondayOf_Wednesday(t *testing.T) {
	if got := mondayOf(d("2024-01-10")); !got.Equal(d("2024-01-08")) {
		t.Errorf("mondayOf(Wednesday) = %v, want 2024-01-08", got)
	}
}

func TestMondayOf_Saturday(t *testing.T) {
	if got := mondayOf(d("2024-01-13")); !got.Equal(d("2024-01-08")) {
		t.Errorf("mondayOf(Saturday) = %v, want 2024-01-08", got)
	}
}

func TestMondayOf_Sunday(t *testing.T) {
	// Sunday is weekday 0, should map back to Monday of the SAME week.
	sun := d("2024-01-14") // Jan 14 2024 = Sunday
	want := d("2024-01-08")
	if got := mondayOf(sun); !got.Equal(want) {
		t.Errorf("mondayOf(Sunday %v) = %v, want %v", sun, got, want)
	}
}

// ------- truncDay -------

func TestTruncDay_RemovesTimeComponent(t *testing.T) {
	ts := time.Date(2024, 6, 15, 13, 45, 22, 999, time.UTC)
	want := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	if got := truncDay(ts); !got.Equal(want) {
		t.Errorf("truncDay() = %v, want %v", got, want)
	}
}

func TestTruncDay_MidnightUTCUnchanged(t *testing.T) {
	ts := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	if got := truncDay(ts); !got.Equal(ts) {
		t.Errorf("truncDay(midnight) = %v, want unchanged", got)
	}
}

// ------- ComputeDayStatuses -------

func TestComputeDayStatuses_NoEntries_AllUnknown(t *testing.T) {
	h := makeHabit("2024-01-08")
	today := d("2024-01-10")
	statuses := ComputeDayStatuses(h, nil, today)

	for _, date := range []string{"2024-01-08", "2024-01-09", "2024-01-10"} {
		if got := statuses[d(date)]; got != model.DayUnknown {
			t.Errorf("date %s: got %v, want DayUnknown", date, got)
		}
	}
}

func TestComputeDayStatuses_AllDone(t *testing.T) {
	h := makeHabit("2024-01-08")
	today := d("2024-01-10")
	entries := []model.Entry{
		entry("2024-01-08", true),
		entry("2024-01-09", true),
		entry("2024-01-10", true),
	}
	statuses := ComputeDayStatuses(h, entries, today)

	for _, date := range []string{"2024-01-08", "2024-01-09", "2024-01-10"} {
		if got := statuses[d(date)]; got != model.DayDone {
			t.Errorf("date %s: got %v, want DayDone", date, got)
		}
	}
}

func TestComputeDayStatuses_SingleIsolatedSkip_Yellow(t *testing.T) {
	// done, skip, done -> skip is yellow
	h := makeHabit("2024-01-08")
	today := d("2024-01-10")
	entries := []model.Entry{
		entry("2024-01-08", true),
		entry("2024-01-09", false), // isolated skip
		entry("2024-01-10", true),
	}
	statuses := ComputeDayStatuses(h, entries, today)

	if got := statuses[d("2024-01-08")]; got != model.DayDone {
		t.Errorf("Jan 8: got %v, want DayDone", got)
	}
	if got := statuses[d("2024-01-09")]; got != model.DayYellow {
		t.Errorf("Jan 9 (isolated skip): got %v, want DayYellow", got)
	}
	if got := statuses[d("2024-01-10")]; got != model.DayDone {
		t.Errorf("Jan 10: got %v, want DayDone", got)
	}
}

func TestComputeDayStatuses_TwoConsecutiveSkips_BothRed(t *testing.T) {
	h := makeHabit("2024-01-08")
	today := d("2024-01-11")
	entries := []model.Entry{
		entry("2024-01-08", true),
		entry("2024-01-09", false), // consecutive skip
		entry("2024-01-10", false), // consecutive skip
		entry("2024-01-11", true),
	}
	statuses := ComputeDayStatuses(h, entries, today)

	if got := statuses[d("2024-01-09")]; got != model.DayRed {
		t.Errorf("Jan 9 (first of 2 consecutive): got %v, want DayRed", got)
	}
	if got := statuses[d("2024-01-10")]; got != model.DayRed {
		t.Errorf("Jan 10 (second of 2 consecutive): got %v, want DayRed", got)
	}
}

func TestComputeDayStatuses_ThreeConsecutiveSkips_AllRed(t *testing.T) {
	h := makeHabit("2024-01-08")
	today := d("2024-01-12")
	entries := []model.Entry{
		entry("2024-01-08", true),
		entry("2024-01-09", false),
		entry("2024-01-10", false),
		entry("2024-01-11", false),
		entry("2024-01-12", true),
	}
	statuses := ComputeDayStatuses(h, entries, today)

	for _, date := range []string{"2024-01-09", "2024-01-10", "2024-01-11"} {
		if got := statuses[d(date)]; got != model.DayRed {
			t.Errorf("date %s: got %v, want DayRed", date, got)
		}
	}
}

func TestComputeDayStatuses_SkipAtStart_Yellow(t *testing.T) {
	// Skip on first day (no previous day) → isolated → yellow
	h := makeHabit("2024-01-08")
	today := d("2024-01-09")
	entries := []model.Entry{
		entry("2024-01-08", false), // first day, isolated skip
		entry("2024-01-09", true),
	}
	statuses := ComputeDayStatuses(h, entries, today)

	if got := statuses[d("2024-01-08")]; got != model.DayYellow {
		t.Errorf("skip at start: got %v, want DayYellow", got)
	}
}

func TestComputeDayStatuses_SkipAtEnd_Yellow(t *testing.T) {
	// Skip on last day (no next day entry) → isolated → yellow
	h := makeHabit("2024-01-08")
	today := d("2024-01-09")
	entries := []model.Entry{
		entry("2024-01-08", true),
		entry("2024-01-09", false), // isolated
	}
	statuses := ComputeDayStatuses(h, entries, today)

	if got := statuses[d("2024-01-09")]; got != model.DayYellow {
		t.Errorf("skip at end: got %v, want DayYellow", got)
	}
}

func TestComputeDayStatuses_TwoSkipsAtEnd_BothRed(t *testing.T) {
	h := makeHabit("2024-01-08")
	today := d("2024-01-10")
	entries := []model.Entry{
		entry("2024-01-08", true),
		entry("2024-01-09", false),
		entry("2024-01-10", false),
	}
	statuses := ComputeDayStatuses(h, entries, today)

	if got := statuses[d("2024-01-09")]; got != model.DayRed {
		t.Errorf("Jan 9: got %v, want DayRed", got)
	}
	if got := statuses[d("2024-01-10")]; got != model.DayRed {
		t.Errorf("Jan 10: got %v, want DayRed", got)
	}
}

func TestComputeDayStatuses_UnknownDayBreaksConsecutiveRun(t *testing.T) {
	// Skip, no-entry (unknown), skip → each skip is isolated → both yellow
	h := makeHabit("2024-01-08")
	today := d("2024-01-10")
	entries := []model.Entry{
		entry("2024-01-08", false), // skip, but Jan 9 has no entry
		// Jan 9: no entry (DayUnknown)
		entry("2024-01-10", false), // skip, but Jan 9 has no entry
	}
	statuses := ComputeDayStatuses(h, entries, today)

	if got := statuses[d("2024-01-08")]; got != model.DayYellow {
		t.Errorf("Jan 8: got %v, want DayYellow (unknown breaks run)", got)
	}
	if got := statuses[d("2024-01-09")]; got != model.DayUnknown {
		t.Errorf("Jan 9: got %v, want DayUnknown", got)
	}
	if got := statuses[d("2024-01-10")]; got != model.DayYellow {
		t.Errorf("Jan 10: got %v, want DayYellow (unknown breaks run)", got)
	}
}

func TestComputeDayStatuses_SkipSpanningWeekBoundary_BothRed(t *testing.T) {
	// Jan 14 (Sunday, end of week 2) and Jan 15 (Monday, start of week 3) both skipped
	h := makeHabit("2024-01-08")
	today := d("2024-01-15")
	entries := []model.Entry{
		entry("2024-01-08", true),
		entry("2024-01-09", true),
		entry("2024-01-10", true),
		entry("2024-01-11", true),
		entry("2024-01-12", true),
		entry("2024-01-13", true),
		entry("2024-01-14", false), // Sunday
		entry("2024-01-15", false), // Monday (next week)
	}
	statuses := ComputeDayStatuses(h, entries, today)

	if got := statuses[d("2024-01-14")]; got != model.DayRed {
		t.Errorf("Sunday (cross-week skip): got %v, want DayRed", got)
	}
	if got := statuses[d("2024-01-15")]; got != model.DayRed {
		t.Errorf("Monday (cross-week skip): got %v, want DayRed", got)
	}
}

func TestComputeDayStatuses_StartDateAfterToday_EmptyResult(t *testing.T) {
	h := makeHabit("2024-12-31")
	today := d("2024-01-01")
	statuses := ComputeDayStatuses(h, nil, today)
	if len(statuses) != 0 {
		t.Errorf("expected empty map for future start date, got %v", statuses)
	}
}

// ------- ComputeWeekStats -------

func TestComputeWeekStats_NoEntries(t *testing.T) {
	h := makeHabit("2024-01-08")
	today := d("2024-01-14")
	hs := ComputeWeekStats(h, nil, today)

	if len(hs.Weeks) != 1 {
		t.Fatalf("expected 1 week, got %d", len(hs.Weeks))
	}
	w := hs.Weeks[0]
	if w.GreenCount != 0 || w.RedCount != 0 || w.YellowCount != 0 {
		t.Errorf("expected all zero counts, got green=%d red=%d yellow=%d", w.GreenCount, w.RedCount, w.YellowCount)
	}
	if w.SuccessRate != -1 {
		t.Errorf("SuccessRate = %v, want -1 (no data)", w.SuccessRate)
	}
}

func TestComputeWeekStats_AllDone_100Percent(t *testing.T) {
	h := makeHabit("2024-01-08")
	today := d("2024-01-14") // full Mon-Sun week

	var entries []model.Entry
	for _, date := range []string{
		"2024-01-08", "2024-01-09", "2024-01-10", "2024-01-11",
		"2024-01-12", "2024-01-13", "2024-01-14",
	} {
		entries = append(entries, entry(date, true))
	}

	hs := ComputeWeekStats(h, entries, today)

	if len(hs.Weeks) != 1 {
		t.Fatalf("expected 1 week, got %d", len(hs.Weeks))
	}
	w := hs.Weeks[0]
	if w.GreenCount != 7 {
		t.Errorf("GreenCount = %d, want 7", w.GreenCount)
	}
	if w.SuccessRate != 1.0 {
		t.Errorf("SuccessRate = %v, want 1.0", w.SuccessRate)
	}
}

func TestComputeWeekStats_OneYellowExcluded_100Percent(t *testing.T) {
	// 6 green + 1 isolated skip = 6/6 = 100%
	// This matches the task description: "the tasks 6 from 7 will be 100 too"
	h := makeHabit("2024-01-08")
	today := d("2024-01-14")
	entries := []model.Entry{
		entry("2024-01-08", true),
		entry("2024-01-09", true),
		entry("2024-01-10", true),
		entry("2024-01-11", false), // isolated skip → yellow
		entry("2024-01-12", true),
		entry("2024-01-13", true),
		entry("2024-01-14", true),
	}
	hs := ComputeWeekStats(h, entries, today)

	w := hs.Weeks[0]
	if w.GreenCount != 6 {
		t.Errorf("GreenCount = %d, want 6", w.GreenCount)
	}
	if w.YellowCount != 1 {
		t.Errorf("YellowCount = %d, want 1", w.YellowCount)
	}
	if w.RedCount != 0 {
		t.Errorf("RedCount = %d, want 0", w.RedCount)
	}
	if w.SuccessRate != 1.0 {
		t.Errorf("SuccessRate = %v, want 1.0 (100%%)", w.SuccessRate)
	}
}

func TestComputeWeekStats_OneRed_86Percent(t *testing.T) {
	// 6 green + 1 red = 6/7 ≈ 85.7%
	h := makeHabit("2024-01-08")
	today := d("2024-01-14")
	entries := []model.Entry{
		entry("2024-01-08", true),
		entry("2024-01-09", true),
		entry("2024-01-10", true),
		entry("2024-01-11", true),
		entry("2024-01-12", true),
		entry("2024-01-13", true),
		entry("2024-01-14", false), // will be yellow (isolated end)
	}
	// Note: last day skip at end is YELLOW (isolated), not red.
	// To get a red, we need a second red in the run.
	// Instead test with a pair of reds spanning two weeks (task spec example).
	_ = entries // we need a different setup for red

	// Build week 1 + partial week 2 with a cross-boundary red run
	h = makeHabit("2024-01-08")
	today = d("2024-01-15") // Monday of week 2

	var fullEntries []model.Entry
	// Week 1: Jan 8-13 done, Jan 14 (Sunday) skipped
	for _, date := range []string{
		"2024-01-08", "2024-01-09", "2024-01-10",
		"2024-01-11", "2024-01-12", "2024-01-13",
	} {
		fullEntries = append(fullEntries, entry(date, true))
	}
	fullEntries = append(fullEntries, entry("2024-01-14", false)) // Sunday
	fullEntries = append(fullEntries, entry("2024-01-15", false)) // Monday week 2 - makes both RED

	hs := ComputeWeekStats(h, fullEntries, today)

	if len(hs.Weeks) != 2 {
		t.Fatalf("expected 2 weeks, got %d", len(hs.Weeks))
	}
	week1 := hs.Weeks[0]
	if week1.GreenCount != 6 {
		t.Errorf("week1 GreenCount = %d, want 6", week1.GreenCount)
	}
	if week1.RedCount != 1 {
		t.Errorf("week1 RedCount = %d, want 1 (Sunday is red due to cross-week run)", week1.RedCount)
	}
	if week1.YellowCount != 0 {
		t.Errorf("week1 YellowCount = %d, want 0", week1.YellowCount)
	}
	// rate = 6 / (7 - 0) = 6/7 ≈ 0.857
	wantRate := 6.0 / 7.0
	if abs(week1.SuccessRate-wantRate) > 0.001 {
		t.Errorf("week1 SuccessRate = %v, want ~%v (6/7 ~86%%)", week1.SuccessRate, wantRate)
	}
}

func TestComputeWeekStats_TaskSpecScenario(t *testing.T) {
	// Reproduces the exact task description scenario:
	// Week N: 6/7 done with 1 yellow => 100%
	// Then next week first day also skipped => that skip + Sunday become RED
	// Week N becomes 6/7 ≈ 86%, as described.
	h := makeHabit("2024-01-08")
	today := d("2024-01-22") // after the cross-week skip

	var entries []model.Entry
	// Week 1 (Jan 8-14): Mon-Sat done, Sun skipped
	for _, date := range []string{
		"2024-01-08", "2024-01-09", "2024-01-10",
		"2024-01-11", "2024-01-12", "2024-01-13",
	} {
		entries = append(entries, entry(date, true))
	}
	entries = append(entries, entry("2024-01-14", false)) // Sunday - will become RED
	// Week 2 (Jan 15-21): Mon skipped (makes Sunday RED too), rest done
	entries = append(entries, entry("2024-01-15", false)) // Monday - RED
	for _, date := range []string{
		"2024-01-16", "2024-01-17", "2024-01-18", "2024-01-19", "2024-01-20", "2024-01-21",
	} {
		entries = append(entries, entry(date, true))
	}
	entries = append(entries, entry("2024-01-22", true))

	hs := ComputeWeekStats(h, entries, today)

	if len(hs.Weeks) < 2 {
		t.Fatalf("expected at least 2 weeks, got %d", len(hs.Weeks))
	}
	week1 := hs.Weeks[0]
	// Week 1: 6 green, 1 red (Sunday becomes red) → 6/7 ≈ 86%
	if week1.RedCount != 1 {
		t.Errorf("week1 RedCount = %d, want 1", week1.RedCount)
	}
	wantRate := 6.0 / 7.0
	if abs(week1.SuccessRate-wantRate) > 0.001 {
		t.Errorf("week1 SuccessRate = %v, want ~%.3f", week1.SuccessRate, wantRate)
	}
}

func TestComputeWeekStats_PartialFirstWeek(t *testing.T) {
	// Habit starts on Wednesday, first "week" has 5 days (Wed-Sun)
	h := makeHabit("2024-01-10") // Wednesday
	today := d("2024-01-14")     // Sunday same week

	entries := []model.Entry{
		entry("2024-01-10", true),
		entry("2024-01-11", true),
		entry("2024-01-12", true),
		entry("2024-01-13", true),
		entry("2024-01-14", true),
	}
	hs := ComputeWeekStats(h, entries, today)

	if len(hs.Weeks) != 1 {
		t.Fatalf("expected 1 week, got %d", len(hs.Weeks))
	}
	w := hs.Weeks[0]
	if len(w.Days) != 5 {
		t.Errorf("partial first week has %d days, want 5", len(w.Days))
	}
	if w.GreenCount != 5 {
		t.Errorf("GreenCount = %d, want 5", w.GreenCount)
	}
	if w.SuccessRate != 1.0 {
		t.Errorf("SuccessRate = %v, want 1.0", w.SuccessRate)
	}
}

func TestComputeWeekStats_MultipleWeeks(t *testing.T) {
	h := makeHabit("2024-01-08")
	today := d("2024-01-21") // 2 full weeks

	var entries []model.Entry
	for _, date := range []string{
		"2024-01-08", "2024-01-09", "2024-01-10", "2024-01-11",
		"2024-01-12", "2024-01-13", "2024-01-14", // week 1 all done
		"2024-01-15", "2024-01-16", "2024-01-17", "2024-01-18",
		"2024-01-19", "2024-01-20", "2024-01-21", // week 2 all done
	} {
		entries = append(entries, entry(date, true))
	}
	hs := ComputeWeekStats(h, entries, today)

	if len(hs.Weeks) != 2 {
		t.Fatalf("expected 2 weeks, got %d", len(hs.Weeks))
	}
	for i, w := range hs.Weeks {
		if w.SuccessRate != 1.0 {
			t.Errorf("week %d SuccessRate = %v, want 1.0", i+1, w.SuccessRate)
		}
	}
	if hs.GlobalRate != 1.0 {
		t.Errorf("GlobalRate = %v, want 1.0", hs.GlobalRate)
	}
}

func TestComputeWeekStats_StartDateAfterToday(t *testing.T) {
	h := makeHabit("2025-12-31")
	today := d("2024-01-01")
	hs := ComputeWeekStats(h, nil, today)
	if len(hs.Weeks) != 0 {
		t.Errorf("expected 0 weeks for future start date, got %d", len(hs.Weeks))
	}
}

func TestComputeWeekStats_AllSkipsInWeek_MinusOne(t *testing.T) {
	// If all days are skipped (red), rate = 0/7 = 0
	h := makeHabit("2024-01-08")
	today := d("2024-01-14")
	entries := []model.Entry{
		entry("2024-01-08", false),
		entry("2024-01-09", false),
		entry("2024-01-10", false),
		entry("2024-01-11", false),
		entry("2024-01-12", false),
		entry("2024-01-13", false),
		entry("2024-01-14", false),
	}
	hs := ComputeWeekStats(h, entries, today)
	w := hs.Weeks[0]
	if w.SuccessRate != 0.0 {
		t.Errorf("SuccessRate = %v, want 0.0 (all skips)", w.SuccessRate)
	}
}

func TestComputeWeekStats_WeekStartIsMonday(t *testing.T) {
	h := makeHabit("2024-01-10") // Wednesday
	today := d("2024-01-14")
	hs := ComputeWeekStats(h, nil, today)

	if len(hs.Weeks) == 0 {
		t.Fatal("expected at least 1 week")
	}
	wk := hs.Weeks[0].WeekStart
	if wk.Weekday() != time.Monday {
		t.Errorf("WeekStart weekday = %v, want Monday", wk.Weekday())
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// ------- Non-obligated skip rule -------

func TestComputeDayStatuses_NonObligated_ConsecutiveSkips_AllYellow(t *testing.T) {
	h := makeNonObligatedHabit("2024-01-08")
	today := d("2024-01-12")
	entries := []model.Entry{
		entry("2024-01-08", true),
		entry("2024-01-09", false), // consecutive
		entry("2024-01-10", false), // consecutive
		entry("2024-01-11", false), // consecutive
		entry("2024-01-12", true),
	}
	statuses := ComputeDayStatuses(h, entries, today)

	for _, date := range []string{"2024-01-09", "2024-01-10", "2024-01-11"} {
		if got := statuses[d(date)]; got != model.DayYellow {
			t.Errorf("non-obligated consecutive skip %s: got %v, want DayYellow", date, got)
		}
	}
}

func TestComputeDayStatuses_NonObligated_SingleSkip_Yellow(t *testing.T) {
	h := makeNonObligatedHabit("2024-01-08")
	today := d("2024-01-10")
	entries := []model.Entry{
		entry("2024-01-08", true),
		entry("2024-01-09", false),
		entry("2024-01-10", true),
	}
	statuses := ComputeDayStatuses(h, entries, today)

	if got := statuses[d("2024-01-09")]; got != model.DayYellow {
		t.Errorf("non-obligated single skip: got %v, want DayYellow", got)
	}
}

func TestComputeDayStatuses_ObligationTransition(t *testing.T) {
	// Habit becomes obligated on Jan 10. Skips before that are yellow, after are red.
	since := d("2024-01-10")
	h := model.Habit{
		StartDate:          d("2024-01-08"),
		IsObligated:        true,
		ObligatedSinceDate: &since,
	}
	today := d("2024-01-13")
	entries := []model.Entry{
		entry("2024-01-08", false), // pre-obligated: yellow
		entry("2024-01-09", false), // pre-obligated: yellow (consecutive with Jan 8 but both non-obligated)
		entry("2024-01-10", true),
		entry("2024-01-11", false), // obligated: red (adjacent to Jan 12)
		entry("2024-01-12", false), // obligated: red (adjacent to Jan 11)
		entry("2024-01-13", true),
	}
	statuses := ComputeDayStatuses(h, entries, today)

	if got := statuses[d("2024-01-08")]; got != model.DayYellow {
		t.Errorf("Jan 8 (pre-obligated): got %v, want DayYellow", got)
	}
	if got := statuses[d("2024-01-09")]; got != model.DayYellow {
		t.Errorf("Jan 9 (pre-obligated): got %v, want DayYellow", got)
	}
	if got := statuses[d("2024-01-10")]; got != model.DayDone {
		t.Errorf("Jan 10: got %v, want DayDone", got)
	}
	if got := statuses[d("2024-01-11")]; got != model.DayRed {
		t.Errorf("Jan 11 (obligated consecutive): got %v, want DayRed", got)
	}
	if got := statuses[d("2024-01-12")]; got != model.DayRed {
		t.Errorf("Jan 12 (obligated consecutive): got %v, want DayRed", got)
	}
}

func TestComputeWeekStats_NonObligated_OneGreenSixSkips_100Percent(t *testing.T) {
	// Non-obligated: 1 green + 6 skips → all skips are yellow → denom=1, rate=100%
	h := makeNonObligatedHabit("2024-01-08")
	today := d("2024-01-14")
	entries := []model.Entry{
		entry("2024-01-08", true),
		entry("2024-01-09", false),
		entry("2024-01-10", false),
		entry("2024-01-11", false),
		entry("2024-01-12", false),
		entry("2024-01-13", false),
		entry("2024-01-14", false),
	}
	hs := ComputeWeekStats(h, entries, today)

	if len(hs.Weeks) != 1 {
		t.Fatalf("expected 1 week, got %d", len(hs.Weeks))
	}
	w := hs.Weeks[0]
	if w.GreenCount != 1 {
		t.Errorf("GreenCount = %d, want 1", w.GreenCount)
	}
	if w.YellowCount != 6 {
		t.Errorf("YellowCount = %d, want 6", w.YellowCount)
	}
	if w.RedCount != 0 {
		t.Errorf("RedCount = %d, want 0", w.RedCount)
	}
	if w.SuccessRate != 1.0 {
		t.Errorf("SuccessRate = %v, want 1.0 (100%% for non-obligated)", w.SuccessRate)
	}
}
