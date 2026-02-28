package service

import (
	"database/sql"
	"fmt"
	"hbt/internal/model"
	"time"
)

// RecordEntry upserts an entry for a habit on a date.
func RecordEntry(db *sql.DB, habitID int64, date time.Time, didIt bool) error {
	didItInt := 0
	if didIt {
		didItInt = 1
	}
	_, err := db.Exec(`
		INSERT INTO entries (habit_id, entry_date, did_it)
		VALUES (?, ?, ?)
		ON CONFLICT(habit_id, entry_date) DO UPDATE SET did_it = excluded.did_it`,
		habitID, fmtDate(date), didItInt,
	)
	return err
}

// DeleteEntry removes an entry for a habit on a date (no-op if none exists).
func DeleteEntry(db *sql.DB, habitID int64, date time.Time) error {
	_, err := db.Exec(`DELETE FROM entries WHERE habit_id = ? AND entry_date = ?`,
		habitID, fmtDate(date))
	return err
}

// GetEntryForDay returns the entry for a specific habit+date, or nil if none.
func GetEntryForDay(db *sql.DB, habitID int64, date time.Time) (*model.Entry, error) {
	row := db.QueryRow(`
		SELECT id, habit_id, entry_date, did_it, created_at
		FROM entries WHERE habit_id = ? AND entry_date = ?`,
		habitID, fmtDate(date),
	)
	e, err := scanEntry(row.Scan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// GetEntriesForHabit returns all entries for a habit ordered by entry_date ASC.
func GetEntriesForHabit(db *sql.DB, habitID int64) ([]model.Entry, error) {
	rows, err := db.Query(`
		SELECT id, habit_id, entry_date, did_it, created_at
		FROM entries WHERE habit_id = ? ORDER BY entry_date ASC`,
		habitID,
	)
	if err != nil {
		return nil, fmt.Errorf("get entries: %w", err)
	}
	defer rows.Close()

	var entries []model.Entry
	for rows.Next() {
		e, err := scanEntry(rows.Scan)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetPendingBackfill returns BackfillItems: non-archived habits whose start_date
// is before today, for each calendar day from start_date to yesterday that has no entry.
func GetPendingBackfill(db *sql.DB, todayDate time.Time) ([]model.BackfillItem, error) {
	today := truncDay(todayDate)
	yesterday := today.AddDate(0, 0, -1)

	// Find all non-archived habits that started before today.
	rows, err := db.Query(`
		SELECT h.id, h.name, h.start_date, h.is_obligated, h.obligated_since_date,
		       h.archived, COALESCE(h.archive_comment,''), h.created_at
		FROM habits h
		WHERE h.archived = 0 AND h.start_date < ?
		ORDER BY h.is_obligated DESC, h.start_date ASC`,
		fmtDate(today),
	)
	if err != nil {
		return nil, fmt.Errorf("backfill query: %w", err)
	}
	defer rows.Close()

	var habits []model.Habit
	for rows.Next() {
		var h model.Habit
		var startDate, createdAt string
		var oblSince sql.NullString
		var isObligated, archived int

		err := rows.Scan(
			&h.ID, &h.Name, &startDate, &isObligated, &oblSince,
			&archived, &h.ArchiveComment, &createdAt,
		)
		if err != nil {
			return nil, err
		}
		h.StartDate, _ = parseDate(startDate)
		h.IsObligated = isObligated != 0
		h.Archived = archived != 0
		if oblSince.Valid {
			t, _ := parseDate(oblSince.String)
			h.ObligatedSinceDate = &t
		}
		habits = append(habits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var items []model.BackfillItem
	for _, h := range habits {
		rangeStart := truncDay(h.StartDate)

		// Load existing entry dates for this habit in the range.
		entryRows, err := db.Query(`
			SELECT entry_date FROM entries
			WHERE habit_id = ? AND entry_date >= ? AND entry_date <= ?
			ORDER BY entry_date ASC`,
			h.ID, fmtDate(rangeStart), fmtDate(yesterday),
		)
		if err != nil {
			return nil, err
		}
		existing := map[string]bool{}
		for entryRows.Next() {
			var d string
			if err := entryRows.Scan(&d); err != nil {
				entryRows.Close()
				return nil, err
			}
			existing[d] = true
		}
		entryRows.Close()

		// Walk each date from start_date to yesterday, collect gaps.
		for d := rangeStart; !d.After(yesterday); d = d.AddDate(0, 0, 1) {
			if !existing[fmtDate(d)] {
				items = append(items, model.BackfillItem{Habit: h, Date: d})
			}
		}
	}

	return items, nil
}

// GetLastEntryDate returns the most recent entry_date for a habit, or nil if none.
func GetLastEntryDate(db *sql.DB, habitID int64) (*time.Time, error) {
	var dateStr sql.NullString
	err := db.QueryRow(`SELECT MAX(entry_date) FROM entries WHERE habit_id = ?`, habitID).Scan(&dateStr)
	if err != nil {
		return nil, err
	}
	if !dateStr.Valid {
		return nil, nil
	}
	t, err := parseDate(dateStr.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func scanEntry(scan func(...any) error) (model.Entry, error) {
	var e model.Entry
	var entryDate, createdAt string
	var didIt int
	err := scan(&e.ID, &e.HabitID, &entryDate, &didIt, &createdAt)
	if err != nil {
		return e, err
	}
	e.EntryDate, _ = parseDate(entryDate)
	e.DidIt = didIt != 0
	e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return e, nil
}
