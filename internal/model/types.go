package model

import "time"

// Habit represents a tracked habit.
type Habit struct {
	ID                 int64
	Name               string
	StartDate          time.Time
	IsObligated        bool
	ObligatedSinceDate *time.Time
	Archived           bool
	ArchiveComment     string
	CreatedAt          time.Time
}

// Entry records whether a habit was done or skipped on a specific date.
type Entry struct {
	ID        int64
	HabitID   int64
	EntryDate time.Time
	DidIt     bool
	CreatedAt time.Time
}

// DayStatus is the computed display color for one calendar day.
type DayStatus int

const (
	DayUnknown DayStatus = iota // No entry; not yet asked (gray)
	DayDone                     // GREEN: did_it = true
	DayYellow                   // YELLOW: exactly 1 isolated skip
	DayRed                      // RED: 2+ consecutive skips
	DayFuture                   // Beyond today (no square drawn)
)

// DayResult pairs a date with its computed display status.
type DayResult struct {
	Date   time.Time
	Status DayStatus
}

// WeekStats holds computed statistics for one calendar week (Mon–Sun).
type WeekStats struct {
	WeekStart   time.Time
	Days        []DayResult
	GreenCount  int
	YellowCount int
	RedCount    int
	TotalDays   int     // days with a real entry (green or red); excludes yellow and unknown
	SuccessRate float64 // GreenCount / (counted days); -1 if no data
}

// HabitStats is the full statistics for a single habit.
type HabitStats struct {
	Habit      Habit
	Weeks      []WeekStats
	GlobalRate float64 // weighted average SuccessRate across weeks that have data
}

// GlobalStats is the app-wide summary.
type GlobalStats struct {
	AverageRate float64
	TotalGreen  int
	TotalYellow int
	TotalRed    int
}

// BackfillItem is one pending "did you do X on [date]?" question.
type BackfillItem struct {
	Habit Habit
	Date  time.Time
}
