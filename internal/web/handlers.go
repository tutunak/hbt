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

	var bf *backfillData
	items, _ := service.GetPendingBackfill(s.db, td)
	if len(items) > 0 {
		bf = &backfillData{
			HabitID:   items[0].Habit.ID,
			HabitName: items[0].Habit.Name,
			Date:      items[0].Date,
			DateStr:   items[0].Date.Format("Mon, Jan 2"),
			Idx:       1,
			Total:     len(items),
		}
	}

	md := computeMonthData(td, rows)

	data := indexData{
		Today:             td,
		DateStr:           td.Format("Mon Jan 2 2006"),
		Habits:            rows,
		GlobalStats:       gs,
		NeedPromotion:     needPromo,
		PromoHabits:       promoHabits,
		BackfillItem:      bf,
		NonObligatedCount: nonOblCount,
		HasHabits:         len(habits) > 0,
		MonthData:         md,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.index.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func (s *Server) buildHabitRow(h model.Habit, td time.Time) habitRowData {
	rd := habitRowData{Habit: h}

	year, month, _ := td.Date()
	daysInMonth := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	rd.MonthDayCols = daysInMonth

	lastEntry, _ := service.GetLastEntryDate(s.db, h.ID)
	if lastEntry == nil {
		rd.HasHistory = false
		return rd
	}

	rd.HasHistory = true
	daysAgo := int(td.Sub(*lastEntry).Hours() / 24)
	if daysAgo > 42 {
		rd.Inactive = true
		rd.InactiveMsg = fmt.Sprintf("(inactive %dw)", daysAgo/7)
		// Don't return early — still compute squares for the row.
	}

	entries, _ := service.GetEntriesForHabit(s.db, h.ID)
	statuses := service.ComputeDayStatuses(h, entries, td)

	for day := 1; day <= daysInMonth; day++ {
		d := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
		status := statuses[d]
		if d.After(td) {
			status = model.DayFuture
		}
		sq := daySquare{
			Status:  status,
			Date:    d,
			IsToday: day == td.Day(),
		}
		rd.MonthSquares = append(rd.MonthSquares, sq)

		if !rd.Inactive {
			switch status {
			case model.DayDone:
				rd.MonthGreen++
			case model.DayRed:
				rd.MonthRed++
			}
		}
	}

	denom := rd.MonthGreen + rd.MonthRed
	if denom > 0 {
		rd.MonthRate = float64(rd.MonthGreen) / float64(denom)
		rd.RateStr = fmt.Sprintf("%d/%d (%s)", rd.MonthGreen, denom, fmtPct(rd.MonthRate))
	} else {
		rd.MonthRate = -1
		rd.RateStr = "(no data)"
	}

	return rd
}

func computeMonthData(td time.Time, rows []habitRowData) monthViewData {
	year, month, _ := td.Date()
	daysInMonth := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	todayDay := td.Day()

	// Build week groups: 7-day chunks of the month.
	var weekGroups []weekGroup
	for start := 1; start <= daysInMonth; start += 7 {
		end := start + 6
		if end > daysInMonth {
			end = daysInMonth
		}
		var days []int
		for d := start; d <= end; d++ {
			days = append(days, d)
		}
		n := (start-1)/7 + 1
		weekGroups = append(weekGroups, weekGroup{
			Label: fmt.Sprintf("WEEK %d", n),
			Days:  days,
		})
	}

	// Accumulate per-day green/red counts across all active habits.
	dayGreen := make([]int, daysInMonth+1) // index 1..daysInMonth
	dayRed := make([]int, daysInMonth+1)
	for _, row := range rows {
		if !row.HasHistory || row.Inactive {
			continue
		}
		for _, sq := range row.MonthSquares {
			d := sq.Date.Day()
			switch sq.Status {
			case model.DayDone:
				dayGreen[d]++
			case model.DayRed:
				dayRed[d]++
			}
		}
	}

	// Daily labels and completion percentages (up to today).
	var dayLabels []string
	var dailyPcts []float64
	for d := 1; d <= todayDay; d++ {
		dayLabels = append(dayLabels, strconv.Itoa(d))
		g, r := dayGreen[d], dayRed[d]
		if g+r > 0 {
			dailyPcts = append(dailyPcts, float64(g)/float64(g+r)*100)
		} else {
			dailyPcts = append(dailyPcts, 0)
		}
	}

	// Weekly completion percentages.
	var weeklyPcts []float64
	for _, wg := range weekGroups {
		var g, r int
		for _, d := range wg.Days {
			if d <= todayDay {
				g += dayGreen[d]
				r += dayRed[d]
			}
		}
		if g+r > 0 {
			weeklyPcts = append(weeklyPcts, float64(g)/float64(g+r)*100)
		} else {
			weeklyPcts = append(weeklyPcts, -1)
		}
	}

	// Overall monthly percentage.
	var totalGreen, totalRed int
	for _, row := range rows {
		totalGreen += row.MonthGreen
		totalRed += row.MonthRed
	}
	overallPct := -1.0
	if totalGreen+totalRed > 0 {
		overallPct = float64(totalGreen) / float64(totalGreen+totalRed) * 100
	}

	return monthViewData{
		MonthLabel: td.Format("January 2006"),
		WeekGroups: weekGroups,
		DayLabels:  dayLabels,
		DailyPcts:  dailyPcts,
		WeeklyPcts: weeklyPcts,
		OverallPct: overallPct,
		TotalGreen: totalGreen,
		TotalPoss:  totalGreen + totalRed,
	}
}

func fmtPct(r float64) string {
	return fmt.Sprintf("%.0f%%", r*100)
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

func (s *Server) handleBackfillAnswer(w http.ResponseWriter, r *http.Request) {
	td := today()
	habitID, err := strconv.ParseInt(r.PathValue("habitID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid habit id", 400)
		return
	}
	date, err := time.Parse("2006-01-02", r.PathValue("date"))
	if err != nil {
		http.Error(w, "invalid date", 400)
		return
	}

	// "Yes" = did it
	if err := service.RecordEntry(s.db, habitID, date, true); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	s.renderNextBackfill(w, td)
}

func (s *Server) handleBackfillSkip(w http.ResponseWriter, r *http.Request) {
	td := today()
	habitID, err := strconv.ParseInt(r.PathValue("habitID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid habit id", 400)
		return
	}
	date, err := time.Parse("2006-01-02", r.PathValue("date"))
	if err != nil {
		http.Error(w, "invalid date", 400)
		return
	}

	// "No" = did not do it
	if err := service.RecordEntry(s.db, habitID, date, false); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	s.renderNextBackfill(w, td)
}

func (s *Server) renderNextBackfill(w http.ResponseWriter, td time.Time) {
	items, _ := service.GetPendingBackfill(s.db, td)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if len(items) == 0 {
		// Backfill done — trigger page reload to refresh habit rows
		w.Header().Set("HX-Refresh", "true")
		return
	}

	bf := &backfillData{
		HabitID:   items[0].Habit.ID,
		HabitName: items[0].Habit.Name,
		Date:      items[0].Date,
		DateStr:   items[0].Date.Format("Mon, Jan 2"),
		Idx:       1,
		Total:     len(items),
	}
	s.tmpl.backfill.ExecuteTemplate(w, "backfill_item", bf)
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
