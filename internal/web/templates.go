package web

import (
	"embed"
	"fmt"
	"hbt/internal/model"
	"html/template"
	"time"
)

//go:embed templates/* static/*
var content embed.FS

type templateSet struct {
	index      *template.Template
	stats      *template.Template
	addHabit   *template.Template
	archive    *template.Template
	habitRow   *template.Template
	backfill   *template.Template
	globalLine *template.Template
}

func squareClass(status model.DayStatus) string {
	switch status {
	case model.DayDone:
		return "sq-green"
	case model.DayYellow:
		return "sq-yellow"
	case model.DayRed:
		return "sq-red"
	case model.DayFuture:
		return "sq-future"
	default:
		return "sq-gray"
	}
}

func loadTemplates() (*templateSet, error) {
	funcs := template.FuncMap{
		"fmtPct": func(r float64) string {
			return fmt.Sprintf("%.0f%%", r*100)
		},
		"squareClass": func(status model.DayStatus) string {
			return squareClass(status)
		},
		"fmtDate": func(t time.Time) string {
			return t.Format("2006-01-02")
		},
		"fmtDateHuman": func(t time.Time) string {
			return t.Format("Mon, Jan 2")
		},
		"fmtDateLong": func(t time.Time) string {
			return t.Format("Mon Jan 2 2006")
		},
		"fmtWeekRange": func(t time.Time) string {
			end := t.AddDate(0, 0, 6)
			return t.Format("Jan 2") + "–" + end.Format("Jan 2")
		},
		"obligatedEmoji": func(h model.Habit) string {
			if h.IsObligated {
				return "✅"
			}
			return "⬜"
		},
		"isToday": func(t, today time.Time) bool {
			return t.Year() == today.Year() && t.YearDay() == today.YearDay()
		},
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i
			}
			return s
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"add": func(a, b int) int {
			return a + b
		},
	}

	parse := func(files ...string) (*template.Template, error) {
		return template.New("").Funcs(funcs).ParseFS(content, files...)
	}

	layout := "templates/layout.html"

	index, err := parse(layout, "templates/index.html",
		"templates/partials/habit_row.html",
		"templates/partials/backfill_item.html",
		"templates/partials/global_stats.html")
	if err != nil {
		return nil, fmt.Errorf("parse index: %w", err)
	}

	stats, err := parse(layout, "templates/stats.html", "templates/partials/global_stats.html")
	if err != nil {
		return nil, fmt.Errorf("parse stats: %w", err)
	}

	addHabit, err := parse(layout, "templates/add_habit.html")
	if err != nil {
		return nil, fmt.Errorf("parse add_habit: %w", err)
	}

	archive, err := parse(layout, "templates/archive.html")
	if err != nil {
		return nil, fmt.Errorf("parse archive: %w", err)
	}

	habitRow, err := parse("templates/partials/habit_row.html", "templates/partials/global_stats.html")
	if err != nil {
		return nil, fmt.Errorf("parse habit_row partial: %w", err)
	}

	backfill, err := parse("templates/partials/backfill_item.html")
	if err != nil {
		return nil, fmt.Errorf("parse backfill partial: %w", err)
	}

	globalLine, err := parse("templates/partials/global_stats.html")
	if err != nil {
		return nil, fmt.Errorf("parse global_stats partial: %w", err)
	}

	return &templateSet{
		index:      index,
		stats:      stats,
		addHabit:   addHabit,
		archive:    archive,
		habitRow:   habitRow,
		backfill:   backfill,
		globalLine: globalLine,
	}, nil
}

// Data types for templates

type indexData struct {
	Today             time.Time
	DateStr           string
	Habits            []habitRowData
	GlobalStats       model.GlobalStats
	NeedPromotion     bool
	PromoHabits       []model.Habit
	BackfillItem      *backfillData
	BackfillTotal     int
	BackfillIdx       int
	NonObligatedCount int
	HasHabits         bool
}

type habitRowData struct {
	Habit       model.Habit
	Squares     []daySquare
	GreenCount  int
	RedCount    int
	HasHistory  bool
	Inactive    bool
	InactiveMsg string
	RateStr     string
}

type daySquare struct {
	Status model.DayStatus
	Date   time.Time
	WeekSep bool // insert extra space before this square (week separator)
}

type backfillData struct {
	HabitID   int64
	HabitName string
	Date      time.Time
	DateStr   string
	Idx       int
	Total     int
}

type statsData struct {
	Today       time.Time
	HabitStats  []model.HabitStats
	GlobalStats model.GlobalStats
	HasHabits   bool
}

type addHabitData struct {
	Error string
	Name  string
	Date  string
}

type archiveData struct {
	Habit model.Habit
}
