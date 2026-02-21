package service

import (
	"database/sql"
	"hbt/internal/model"
	"time"
)

// NeedsWeeklyPromotion returns true if the Monday of the week containing today
// has not yet been recorded in weekly_promotions AND there is at least one
// non-obligated, non-archived habit to promote.
func NeedsWeeklyPromotion(db *sql.DB, today time.Time) (bool, error) {
	monday := truncDay(mondayOf(today))
	weekStr := fmtDate(monday)

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM weekly_promotions WHERE week_start = ?`, weekStr).Scan(&count); err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}

	var habitCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM habits WHERE is_obligated = 0 AND archived = 0`).Scan(&habitCount); err != nil {
		return false, err
	}
	return habitCount > 0, nil
}

// RecordWeeklyPromotion inserts the Monday of the week containing weekStart into
// weekly_promotions. Uses INSERT OR IGNORE so calling it twice is safe.
func RecordWeeklyPromotion(db *sql.DB, weekStart time.Time) error {
	monday := truncDay(mondayOf(weekStart))
	_, err := db.Exec(`INSERT OR IGNORE INTO weekly_promotions (week_start) VALUES (?)`, fmtDate(monday))
	return err
}

// ListNonObligatedHabits returns all non-archived, non-obligated habits ordered by start_date.
func ListNonObligatedHabits(db *sql.DB) ([]model.Habit, error) {
	rows, err := db.Query(`
		SELECT id, name, start_date, is_obligated, obligated_since_date,
		       archived, COALESCE(archive_comment,''), created_at
		FROM habits WHERE is_obligated = 0 AND archived = 0
		ORDER BY start_date ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var habits []model.Habit
	for rows.Next() {
		h, err := scanHabit(rows.Scan)
		if err != nil {
			return nil, err
		}
		habits = append(habits, h)
	}
	return habits, rows.Err()
}
