package web

import (
	"fmt"
	"hbt/internal/model"
	"hbt/internal/service"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	td := today()

	habits, err := service.ListHabits(s.db, false)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	var nonOblCount int
	for _, h := range habits {
		if !h.IsObligated {
			nonOblCount++
		}
	}

	rows := make([]habitRowData, 0, len(habits))
	for _, h := range habits {
		rows = append(rows, s.buildHabitRow(h, td))
	}

	gs, _ := service.ComputeGlobalStats(s.db, td)

	needPromo, _ := service.NeedsWeeklyPromotion(s.db, td)
	var promoHabits []model.Habit
	if needPromo {
		promoHabits, _ = service.ListNonObligatedHabits(s.db)
	}

	rd := computeRollingData(td, rows)

	// Compute total display columns from the rolling data's week groups.
	totalCols := 0
	for _, wg := range rd.WeekGroups {
		totalCols += len(wg.Days)
	}
	// Set DisplayCols and Unchecked on every row.
	for i := range rows {
		rows[i].DisplayCols = totalCols
		for j := range rows[i].Squares {
			key := rows[i].Squares[j].Date.Format("2006-01-02")
			rows[i].Squares[j].Unchecked = rd.UncheckedDates[key]
		}
	}

	data := indexData{
		Today:             td,
		DateStr:           td.Format("Mon Jan 2 2006"),
		Habits:            rows,
		GlobalStats:       gs,
		NeedPromotion:     needPromo,
		PromoHabits:       promoHabits,
		NonObligatedCount: nonOblCount,
		HasHabits:         len(habits) > 0,
		RollingData:       rd,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.index.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func (s *Server) buildHabitRow(h model.Habit, td time.Time) habitRowData {
	rd := habitRowData{Habit: h}

	lastEntry, _ := service.GetLastEntryDate(s.db, h.ID)
	if lastEntry == nil {
		rd.HasHistory = false
		rd.DisplayCols = 28
		return rd
	}

	rd.HasHistory = true
	daysAgo := int(td.Sub(*lastEntry).Hours() / 24)
	if daysAgo > 42 {
		rd.Inactive = true
		rd.InactiveMsg = fmt.Sprintf("(inactive %dw)", daysAgo/7)
	}

	entries, _ := service.GetEntriesForHabit(s.db, h.ID)
	statuses := service.ComputeDayStatuses(h, entries, td)

	// Build squares from habit start date to today (chronological).
	start := h.StartDate
	rollingStart := td.AddDate(0, 0, -27) // last 28 days

	for d := start; !d.After(td); d = d.AddDate(0, 0, 1) {
		status := statuses[d]
		sq := daySquare{
			Status:  status,
			Date:    d,
			IsToday: d.Equal(td),
		}
		rd.Squares = append(rd.Squares, sq)

		// Count rolling stats only for last 28 days and non-inactive habits.
		if !rd.Inactive && !d.Before(rollingStart) {
			switch status {
			case model.DayDone:
				rd.RollingGreen++
			case model.DayRed:
				rd.RollingRed++
			}
		}
	}

	rd.DisplayCols = len(rd.Squares)
	if rd.DisplayCols < 28 {
		rd.DisplayCols = 28
	}

	denom := rd.RollingGreen + rd.RollingRed
	if denom > 0 {
		rd.RollingRate = float64(rd.RollingGreen) / float64(denom)
		rd.RateStr = fmt.Sprintf("%d/%d (%s)", rd.RollingGreen, denom, fmtPct(rd.RollingRate))
	} else {
		rd.RollingRate = -1
		rd.RateStr = "(no data)"
	}

	return rd
}

func mondayOf(t time.Time) time.Time {
	wd := t.Weekday()
	if wd == time.Sunday {
		wd = 7
	}
	return t.AddDate(0, 0, -int(wd-time.Monday))
}

func computeRollingData(td time.Time, rows []habitRowData) rollingViewData {
	// Find maxDays across all habits (minimum 28).
	maxDays := 28
	for _, row := range rows {
		if len(row.Squares) > maxDays {
			maxDays = len(row.Squares)
		}
	}

	// Compute the date range: oldest..today, spanning maxDays.
	oldest := td.AddDate(0, 0, -(maxDays - 1))

	// Compute unchecked dates: past days with DayUnknown for active habits with history.
	uncheckedDates := map[string]bool{}
	for _, row := range rows {
		if !row.HasHistory || row.Inactive {
			continue
		}
		for _, sq := range row.Squares {
			if sq.Status == model.DayUnknown && !sq.IsToday {
				uncheckedDates[sq.Date.Format("2006-01-02")] = true
			}
		}
	}

	// Build calendar week groups (Mon-Sun) in chronological order.
	weekStart := mondayOf(oldest)
	var weekGroups []weekGroup
	for !weekStart.After(td) {
		weekEnd := weekStart.AddDate(0, 0, 6)
		var days []headerDay
		for d := weekStart; !d.After(weekEnd); d = d.AddDate(0, 0, 1) {
			if !d.Before(oldest) && !d.After(td) {
				key := d.Format("2006-01-02")
				days = append(days, headerDay{
					Num:       strconv.Itoa(d.Day()),
					Unchecked: uncheckedDates[key],
				})
			}
		}
		if len(days) > 0 {
			label := weekStart.Format("Jan 2")
			weekGroups = append(weekGroups, weekGroup{
				Label: label,
				Days:  days,
			})
		}
		weekStart = weekStart.AddDate(0, 0, 7)
	}

	// Accumulate per-day green/red counts across all active habits (last 28 days only).
	rollingStart := td.AddDate(0, 0, -27)
	dayGreen := map[string]int{}
	dayRed := map[string]int{}
	for _, row := range rows {
		if !row.HasHistory || row.Inactive {
			continue
		}
		for _, sq := range row.Squares {
			if sq.Date.Before(rollingStart) {
				continue
			}
			key := sq.Date.Format("2006-01-02")
			switch sq.Status {
			case model.DayDone:
				dayGreen[key]++
			case model.DayRed:
				dayRed[key]++
			}
		}
	}

	// Daily labels and completion percentages for last 28 days (chart data).
	var dayLabels []string
	var dailyPcts []float64
	for d := rollingStart; !d.After(td); d = d.AddDate(0, 0, 1) {
		dayLabels = append(dayLabels, strconv.Itoa(d.Day()))
		key := d.Format("2006-01-02")
		g, r := dayGreen[key], dayRed[key]
		if g+r > 0 {
			dailyPcts = append(dailyPcts, float64(g)/float64(g+r)*100)
		} else {
			dailyPcts = append(dailyPcts, 0)
		}
	}

	// Weekly completion percentages for each week group.
	var weeklyPcts []float64
	wkMonday := mondayOf(oldest)
	for range weekGroups {
		wkEnd := wkMonday.AddDate(0, 0, 6)
		var g, r int
		for d := wkMonday; !d.After(wkEnd); d = d.AddDate(0, 0, 1) {
			if d.Before(rollingStart) || d.After(td) {
				continue
			}
			key := d.Format("2006-01-02")
			g += dayGreen[key]
			r += dayRed[key]
		}
		// Only include weeks that overlap with the 28-day rolling window.
		if wkEnd.Before(rollingStart) {
			weeklyPcts = append(weeklyPcts, -1)
		} else if g+r > 0 {
			weeklyPcts = append(weeklyPcts, float64(g)/float64(g+r)*100)
		} else {
			weeklyPcts = append(weeklyPcts, -1)
		}
		wkMonday = wkMonday.AddDate(0, 0, 7)
	}

	// Overall rolling percentage.
	var totalGreen, totalRed int
	for _, row := range rows {
		totalGreen += row.RollingGreen
		totalRed += row.RollingRed
	}
	overallPct := -1.0
	if totalGreen+totalRed > 0 {
		overallPct = float64(totalGreen) / float64(totalGreen+totalRed) * 100
	}

	return rollingViewData{
		Label:          "Last 4 Weeks",
		WeekGroups:     weekGroups,
		DayLabels:      dayLabels,
		DailyPcts:      dailyPcts,
		WeeklyPcts:     weeklyPcts,
		OverallPct:     overallPct,
		TotalGreen:     totalGreen,
		TotalPoss:      totalGreen + totalRed,
		UncheckedDates: uncheckedDates,
	}
}

func fmtPct(r float64) string {
	return fmt.Sprintf("%.0f%%", r*100)
}

// computeDisplayCols returns the total number of day columns for the grid header.
func (s *Server) computeDisplayCols(td time.Time) int {
	habits, _ := service.ListHabits(s.db, false)
	maxDays := 28
	for _, h := range habits {
		days := int(td.Sub(h.StartDate).Hours()/24) + 1
		if days > maxDays {
			maxDays = days
		}
	}
	oldest := td.AddDate(0, 0, -(maxDays - 1))
	wkStart := mondayOf(oldest)
	totalCols := 0
	for !wkStart.After(td) {
		wkEnd := wkStart.AddDate(0, 0, 6)
		for d := wkStart; !d.After(wkEnd); d = d.AddDate(0, 0, 1) {
			if !d.Before(oldest) && !d.After(td) {
				totalCols++
			}
		}
		wkStart = wkStart.AddDate(0, 0, 7)
	}
	return totalCols
}

func (s *Server) handleRecordEntry(w http.ResponseWriter, r *http.Request) {
	td := today()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", 400)
		return
	}

	didIt := r.URL.Query().Get("did_it") == "true"
	if err := service.RecordEntry(s.db, id, td, didIt); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	h, err := service.GetHabit(s.db, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	row := s.buildHabitRow(h, td)
	row.DisplayCols = s.computeDisplayCols(td)
	gs, _ := service.ComputeGlobalStats(s.db, td)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.habitRow.ExecuteTemplate(w, "habit_row", row); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// Out-of-band swap for global stats
	fmt.Fprint(w, `<div id="global-stats" hx-swap-oob="innerHTML">`)
	s.tmpl.globalLine.ExecuteTemplate(w, "global_stats", gs)
	fmt.Fprint(w, `</div>`)
}

func (s *Server) handleToggleEntry(w http.ResponseWriter, r *http.Request) {
	td := today()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", 400)
		return
	}

	dateStr := r.URL.Query().Get("date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		http.Error(w, "invalid date", 400)
		return
	}

	entry, err := service.GetEntryForDay(s.db, id, date)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	switch {
	case entry == nil:
		err = service.RecordEntry(s.db, id, date, true)
	case entry.DidIt:
		err = service.RecordEntry(s.db, id, date, false)
	default:
		err = service.DeleteEntry(s.db, id, date)
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	h, err := service.GetHabit(s.db, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	row := s.buildHabitRow(h, td)
	row.DisplayCols = s.computeDisplayCols(td)
	gs, _ := service.ComputeGlobalStats(s.db, td)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.habitRow.ExecuteTemplate(w, "habit_row", row); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	fmt.Fprint(w, `<div id="global-stats" hx-swap-oob="innerHTML">`)
	s.tmpl.globalLine.ExecuteTemplate(w, "global_stats", gs)
	fmt.Fprint(w, `</div>`)
}

func (s *Server) handlePromotionConfirm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", 400)
		return
	}

	h, err := service.GetHabit(s.db, id)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}

	data := promoConfirmData{Habit: h}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.tmpl.promoConfirm.ExecuteTemplate(w, "layout", data)
}

func (s *Server) handlePromote(w http.ResponseWriter, r *http.Request) {
	td := today()

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", 400)
		return
	}

	service.SetObligated(s.db, id, true, td)
	service.RecordWeeklyPromotion(s.db, td)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleSkipPromotion(w http.ResponseWriter, r *http.Request) {
	td := today()
	service.RecordWeeklyPromotion(s.db, td)

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/")
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleAddHabitForm(w http.ResponseWriter, r *http.Request) {
	td := today()
	data := addHabitData{Date: td.Format("2006-01-02")}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.tmpl.addHabit.ExecuteTemplate(w, "layout", data)
}

func (s *Server) handleCreateHabit(w http.ResponseWriter, r *http.Request) {
	td := today()
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	startStr := r.FormValue("start_date")
	isObligated := r.FormValue("is_obligated") == "1"
	obligatedSinceStr := r.FormValue("obligated_since")

	if name == "" {
		data := addHabitData{Error: "Name is required.", Name: name, Date: startStr}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(422)
		s.tmpl.addHabit.ExecuteTemplate(w, "layout", data)
		return
	}

	startDate, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		data := addHabitData{Error: "Invalid start date.", Name: name, Date: startStr}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(422)
		s.tmpl.addHabit.ExecuteTemplate(w, "layout", data)
		return
	}

	var obligatedSince *time.Time
	if isObligated && obligatedSinceStr != "" {
		t, err := time.Parse("2006-01-02", obligatedSinceStr)
		if err == nil {
			obligatedSince = &t
		}
	}

	if _, err := service.CreateHabit(s.db, name, startDate, isObligated, obligatedSince); err != nil {
		data := addHabitData{Error: err.Error(), Name: name, Date: td.Format("2006-01-02")}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(500)
		s.tmpl.addHabit.ExecuteTemplate(w, "layout", data)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleArchiveForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", 400)
		return
	}

	h, err := service.GetHabit(s.db, id)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}

	data := archiveData{Habit: h}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.tmpl.archive.ExecuteTemplate(w, "layout", data)
}

func (s *Server) handleArchiveHabit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", 400)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	comment := strings.TrimSpace(r.FormValue("comment"))
	if err := service.ArchiveHabit(s.db, id, comment); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleArchived(w http.ResponseWriter, r *http.Request) {
	td := today()

	habits, err := service.ListHabits(s.db, true)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	var archived []archivedHabitData
	for _, h := range habits {
		if !h.Archived {
			continue
		}
		entries, _ := service.GetEntriesForHabit(s.db, h.ID)
		hs := service.ComputeWeekStats(h, entries, td)
		archived = append(archived, archivedHabitData{Habit: h, Stats: hs})
	}

	data := archivedData{
		Habits:    archived,
		HasHabits: len(archived) > 0,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.archivedList.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	td := today()

	habits, err := service.ListHabits(s.db, false)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	var allStats []habitStatsWithChart
	for _, h := range habits {
		entries, _ := service.GetEntriesForHabit(s.db, h.ID)
		hs := service.ComputeWeekStats(h, entries, td)

		// Build chart data from last 12 weeks (or fewer)
		cd := chartData{}
		start := 0
		if len(hs.Weeks) > 12 {
			start = len(hs.Weeks) - 12
		}
		for _, w := range hs.Weeks[start:] {
			end := w.WeekStart.AddDate(0, 0, 6)
			label := w.WeekStart.Format("Jan 2") + "–" + end.Format("Jan 2")
			cd.Labels = append(cd.Labels, label)
			if w.SuccessRate >= 0 {
				cd.Values = append(cd.Values, w.SuccessRate*100)
			} else {
				cd.Values = append(cd.Values, 0)
			}
		}

		allStats = append(allStats, habitStatsWithChart{
			HabitStats: hs,
			Chart:      cd,
		})
	}

	gs, _ := service.ComputeGlobalStats(s.db, td)

	data := statsData{
		Today:       td,
		HabitStats:  allStats,
		GlobalStats: gs,
		HasHabits:   len(habits) > 0,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.stats.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}
