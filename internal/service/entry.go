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

// GetPendingBackfill returns BackfillItems: habits with at least one past entry
// where specific dates (from earliest entry to yesterday) have no row at all.
func GetPendingBackfill(db *sql.DB, todayDate time.Time) ([]model.BackfillItem, error) {
	yesterday := todayDate.AddDate(0, 0, -1)

	// Find non-archived habits that have at least one entry before today.
	rows, err := db.Query(`
		SELECT h.id, h.name, h.start_date, h.is_obligated, h.obligated_since_date,
		       h.archived, COALESCE(h.archive_comment,''), h.created_at,
		       MIN(e.entry_date) AS min_date
		FROM habits h
		JOIN entries e ON e.habit_id = h.id AND e.entry_date < ?
		WHERE h.archived = 0
		GROUP BY h.id
		ORDER BY h.is_obligated DESC, h.start_date ASC`,
		fmtDate(todayDate),
	)
	if err != nil {
		return nil, fmt.Errorf("backfill query: %w", err)
	}
	defer rows.Close()

	type habitMin struct {
		habit   model.Habit
		minDate time.Time
	}
	var candidates []habitMin

	for rows.Next() {
		var h model.Habit
		var startDate, createdAt, minDateStr string
		var oblSince sql.NullString
		var isObligated, archived int

		err := rows.Scan(
			&h.ID, &h.Name, &startDate, &isObligated, &oblSince,
			&archived, &h.ArchiveComment, &createdAt, &minDateStr,
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
		minDate, err := parseDate(minDateStr)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, habitMin{habit: h, minDate: minDate})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var items []model.BackfillItem
	for _, c := range candidates {
		// Load existing entry dates for this habit in the range.
		entryRows, err := db.Query(`
			SELECT entry_date FROM entries
			WHERE habit_id = ? AND entry_date >= ? AND entry_date <= ?
			ORDER BY entry_date ASC`,
			c.habit.ID, fmtDate(c.minDate), fmtDate(yesterday),
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

		// Walk each date from minDate to yesterday, collect gaps.
		for d := c.minDate; !d.After(yesterday); d = d.AddDate(0, 0, 1) {
			if !existing[fmtDate(d)] {
				items = append(items, model.BackfillItem{Habit: c.habit, Date: d})
			}
		}
	}

	return items, nil
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
