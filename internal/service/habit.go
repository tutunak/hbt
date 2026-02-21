package service

import (
	"database/sql"
	"fmt"
	"hbt/internal/model"
	"time"
)

const dateLayout = "2006-01-02"

func parseDate(s string) (time.Time, error) {
	return time.ParseInLocation(dateLayout, s, time.UTC)
}

func fmtDate(t time.Time) string {
	return t.UTC().Format(dateLayout)
}

// today returns the current date truncated to midnight UTC.
func today() time.Time {
	t := time.Now().UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// scanHabit reads a habit row from a *sql.Rows or *sql.Row scanner.
func scanHabit(scan func(...any) error) (model.Habit, error) {
	var h model.Habit
	var startDate, createdAt string
	var obligatedSince sql.NullString
	var isObligated, archived int

	err := scan(
		&h.ID,
		&h.Name,
		&startDate,
		&isObligated,
		&obligatedSince,
		&archived,
		&h.ArchiveComment,
		&createdAt,
	)
	if err != nil {
		return h, err
	}

	h.StartDate, err = parseDate(startDate)
	if err != nil {
		return h, fmt.Errorf("parse start_date: %w", err)
	}
	h.IsObligated = isObligated != 0
	h.Archived = archived != 0
	if obligatedSince.Valid {
		t, err := parseDate(obligatedSince.String)
		if err != nil {
			return h, fmt.Errorf("parse obligated_since_date: %w", err)
		}
		h.ObligatedSinceDate = &t
	}
	h.CreatedAt, _ = time.Parse("2006-01-02", createdAt)
	return h, nil
}

// CreateHabit inserts a new habit.
func CreateHabit(db *sql.DB, name string, startDate time.Time, isObligated bool, obligatedSince *time.Time) (model.Habit, error) {
	var oblSince sql.NullString
	if obligatedSince != nil {
		oblSince = sql.NullString{String: fmtDate(*obligatedSince), Valid: true}
	}
	oblInt := 0
	if isObligated {
		oblInt = 1
	}
	res, err := db.Exec(`
		INSERT INTO habits (name, start_date, is_obligated, obligated_since_date)
		VALUES (?, ?, ?, ?)`,
		name, fmtDate(startDate), oblInt, oblSince,
	)
	if err != nil {
		return model.Habit{}, fmt.Errorf("insert habit: %w", err)
	}
	id, _ := res.LastInsertId()
	return GetHabit(db, id)
}

// GetHabit fetches a single habit by ID.
func GetHabit(db *sql.DB, id int64) (model.Habit, error) {
	row := db.QueryRow(`
		SELECT id, name, start_date, is_obligated, obligated_since_date,
		       archived, COALESCE(archive_comment,''), created_at
		FROM habits WHERE id = ?`, id)
	h, err := scanHabit(row.Scan)
	if err == sql.ErrNoRows {
		return model.Habit{}, fmt.Errorf("habit %d not found", id)
	}
	return h, err
}

// ListHabits returns all non-archived habits sorted: obligated first, then optional.
// When includeArchived is true, archived habits are appended at the end.
func ListHabits(db *sql.DB, includeArchived bool) ([]model.Habit, error) {
	query := `
		SELECT id, name, start_date, is_obligated, obligated_since_date,
		       archived, COALESCE(archive_comment,''), created_at
		FROM habits`
	if !includeArchived {
		query += ` WHERE archived = 0`
	}
	query += ` ORDER BY
		archived ASC,
		is_obligated DESC,
		COALESCE(obligated_since_date, '9999-99-99') ASC,
		start_date ASC`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("list habits: %w", err)
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

// ArchiveHabit marks a habit as archived with a comment.
func ArchiveHabit(db *sql.DB, id int64, comment string) error {
	_, err := db.Exec(
		`UPDATE habits SET archived = 1, archive_comment = ? WHERE id = ?`,
		comment, id,
	)
	return err
}

// SetObligated updates is_obligated and obligated_since_date.
func SetObligated(db *sql.DB, id int64, obligated bool, since time.Time) error {
	oblInt := 0
	if obligated {
		oblInt = 1
	}
	_, err := db.Exec(
		`UPDATE habits SET is_obligated = ?, obligated_since_date = ? WHERE id = ?`,
		oblInt, fmtDate(since), id,
	)
	return err
}
