package service

import (
	"database/sql"
	"hbt/internal/model"
	"time"
)

// mondayOf returns the Monday of the ISO week containing t.
func mondayOf(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday = 7 in ISO
	}
	return t.AddDate(0, 0, -(weekday - 1))
}

// truncDay normalises a time to midnight UTC.
func truncDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// isNonObligatedDay returns true if the given date should be treated as non-obligated,
// meaning all skips on that day are yellow regardless of adjacency.
// A day is non-obligated if the habit is not obligated, or if it has an ObligatedSinceDate
// and the date falls before that date.
func isNonObligatedDay(h model.Habit, date time.Time) bool {
	if !h.IsObligated {
		return true
	}
	if h.ObligatedSinceDate == nil {
		return false // always strict
	}
	return date.Before(*h.ObligatedSinceDate)
}

// ComputeDayStatuses assigns a DayStatus to every day from start_date to today (inclusive).
// Days before the first recorded entry are DayUnknown.
// Coloring rules:
//   - GREEN:  did_it = true
//   - candidate for YELLOW/RED: did_it = false
//   - YELLOW: exactly 1 isolated skip (no adjacent skip on either side)
//   - RED:    part of a run of 2+ consecutive skips
//
// The result is computed globally (not per-week) so that skip runs crossing week
// boundaries are detected correctly.
func ComputeDayStatuses(habit model.Habit, entries []model.Entry, todayDate time.Time) map[time.Time]model.DayStatus {
	result := map[time.Time]model.DayStatus{}

	// Build a lookup: date -> entry
	byDate := map[time.Time]model.Entry{}
	for _, e := range entries {
		byDate[truncDay(e.EntryDate)] = e
	}

	start := truncDay(habit.StartDate)
	end := truncDay(todayDate)

	// Collect all candidate skip dates (did_it=false) in order.
	// We also need to know which days have a "done" entry to detect isolation.

	// Build ordered day list from start to today.
	type dayInfo struct {
		date  time.Time
		entry *model.Entry // nil = no row
	}
	var days []dayInfo
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		d := truncDay(d)
		if e, ok := byDate[d]; ok {
			eCopy := e
			days = append(days, dayInfo{date: d, entry: &eCopy})
		} else {
			days = append(days, dayInfo{date: d, entry: nil})
		}
	}

	// Assign statuses.
	for i, di := range days {
		if di.entry == nil {
			result[di.date] = model.DayUnknown
			continue
		}
		if di.entry.DidIt {
			result[di.date] = model.DayDone
			continue
		}
		// This day is a skip (did_it=false).
		// Non-obligated days are always yellow regardless of adjacency.
		if isNonObligatedDay(habit, di.date) {
			result[di.date] = model.DayYellow
			continue
		}
		// Obligated skip: check neighbours, but only count adjacent obligated skips.
		prevSkip := i > 0 && days[i-1].entry != nil && !days[i-1].entry.DidIt &&
			!isNonObligatedDay(habit, days[i-1].date)
		nextSkip := i < len(days)-1 && days[i+1].entry != nil && !days[i+1].entry.DidIt &&
			!isNonObligatedDay(habit, days[i+1].date)
		if prevSkip || nextSkip {
			result[di.date] = model.DayRed
		} else {
			result[di.date] = model.DayYellow
		}
	}

	return result
}

// ComputeWeekStats groups day statuses into calendar weeks (Mon–Sun) and computes
// per-week success rates. Returns a HabitStats for the given habit.
func ComputeWeekStats(habit model.Habit, entries []model.Entry, todayDate time.Time) model.HabitStats {
	todayDate = truncDay(todayDate)
	statuses := ComputeDayStatuses(habit, entries, todayDate)

	start := truncDay(habit.StartDate)
	if start.After(todayDate) {
		return model.HabitStats{Habit: habit}
	}

	// Group by week (Monday as week start).
	type weekKey = time.Time
	weekOrder := []weekKey{}
	weekDays := map[weekKey][]model.DayResult{}

	for d := start; !d.After(todayDate); d = d.AddDate(0, 0, 1) {
		day := truncDay(d)
		wk := truncDay(mondayOf(day))
		st, ok := statuses[day]
		if !ok {
			st = model.DayUnknown
		}
		if _, exists := weekDays[wk]; !exists {
			weekOrder = append(weekOrder, wk)
		}
		weekDays[wk] = append(weekDays[wk], model.DayResult{Date: day, Status: st})
	}

	var weeks []model.WeekStats
	var totalGreen, totalYellow, totalRed int
	var totalRate float64
	var rateCount int

	for _, wk := range weekOrder {
		days := weekDays[wk]
		var green, yellow, red int
		for _, dr := range days {
			switch dr.Status {
			case model.DayDone:
				green++
			case model.DayYellow:
				yellow++
			case model.DayRed:
				red++
			}
		}
		denom := len(days) - yellow
		var rate float64 = -1
		if denom > 0 && (green+red) > 0 {
			rate = float64(green) / float64(denom)
			totalRate += rate
			rateCount++
		}
		totalGreen += green
		totalYellow += yellow
		totalRed += red
		weeks = append(weeks, model.WeekStats{
			WeekStart:   wk,
			Days:        days,
			GreenCount:  green,
			YellowCount: yellow,
			RedCount:    red,
			TotalDays:   green + red,
			SuccessRate: rate,
		})
	}

	globalRate := -1.0
	if rateCount > 0 {
		globalRate = totalRate / float64(rateCount)
	}

	return model.HabitStats{
		Habit:      habit,
		Weeks:      weeks,
		GlobalRate: globalRate,
	}
}

// ComputeGlobalStats aggregates stats across all non-archived habits.
func ComputeGlobalStats(db *sql.DB, todayDate time.Time) (model.GlobalStats, error) {
	rows, err := db.Query(`
		SELECT id, name, start_date, is_obligated, obligated_since_date,
		       archived, COALESCE(archive_comment,''), created_at
		FROM habits WHERE archived = 0 AND is_obligated = 1`)
	if err != nil {
		return model.GlobalStats{}, err
	}
	defer rows.Close()

	var habits []model.Habit
	for rows.Next() {
		var h model.Habit
		var startDate, createdAt string
		var oblSince sql.NullString
		var isObligated, archived int
		if err := rows.Scan(&h.ID, &h.Name, &startDate, &isObligated, &oblSince, &archived, &h.ArchiveComment, &createdAt); err != nil {
			return model.GlobalStats{}, err
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
		return model.GlobalStats{}, err
	}

	var totalGreen, totalYellow, totalRed int
	var totalRate float64
	var rateCount int

	for _, h := range habits {
		entries, err := GetEntriesForHabit(db, h.ID)
		if err != nil {
			return model.GlobalStats{}, err
		}
		hs := ComputeWeekStats(h, entries, todayDate)
		for _, w := range hs.Weeks {
			totalGreen += w.GreenCount
			totalYellow += w.YellowCount
			totalRed += w.RedCount
			if w.SuccessRate >= 0 {
				totalRate += w.SuccessRate
				rateCount++
			}
		}
	}

	avg := -1.0
	if rateCount > 0 {
		avg = totalRate / float64(rateCount)
	}

	return model.GlobalStats{
		AverageRate: avg,
		TotalGreen:  totalGreen,
		TotalYellow: totalYellow,
		TotalRed:    totalRed,
	}, nil
}
