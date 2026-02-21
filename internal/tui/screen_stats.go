package tui

import (
	"database/sql"
	"fmt"
	"hbt/internal/model"
	"hbt/internal/service"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// StatsModel shows per-habit statistics and global averages.
type StatsModel struct {
	db       *sql.DB
	today    time.Time
	viewport viewport.Model
	width    int
	height   int
	ready    bool
	content  string
}

func newStatsModel(db *sql.DB, today time.Time) StatsModel {
	return StatsModel{db: db, today: today}
}

func (m StatsModel) Init() tea.Cmd {
	return func() tea.Msg {
		return statsLoadedMsg{content: m.buildContent()}
	}
}

type statsLoadedMsg struct{ content string }

func (m StatsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statsLoadedMsg:
		m.content = msg.content
		if m.width > 0 && m.height > 0 {
			m.viewport = viewport.New(m.width, m.height-4)
			m.viewport.SetContent(m.content)
			m.ready = true
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.content != "" {
			m.viewport = viewport.New(m.width, m.height-4)
			m.viewport.SetContent(m.content)
			m.ready = true
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg { return switchToHabitListMsg{} }
		}
	}

	if m.ready {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m StatsModel) View() string {
	header := styleTitle.Render("Statistics")
	help := styleHelp.Render("[j/k ↑/↓] scroll   [q/esc] back")

	if !m.ready {
		return header + "\n\n" + styleNormal.Render("Loading...") + "\n" + help
	}

	return header + "\n" + m.viewport.View() + "\n" + help
}

func (m StatsModel) buildContent() string {
	habits, err := service.ListHabits(m.db, false)
	if err != nil || len(habits) == 0 {
		return styleNormal.Render("No habits to show statistics for.")
	}

	var sb strings.Builder

	for i, h := range habits {
		if i > 0 {
			sb.WriteString("\n" + styleSeparator.Render(strings.Repeat("─", 50)) + "\n\n")
		}
		entries, _ := service.GetEntriesForHabit(m.db, h.ID)
		hs := service.ComputeWeekStats(h, entries, m.today)
		sb.WriteString(renderHabitStats(hs, m.today))
	}

	global, _ := service.ComputeGlobalStats(m.db, m.today)
	sb.WriteString("\n" + styleSeparator.Render(strings.Repeat("═", 50)) + "\n")
	sb.WriteString(renderGlobalStats(global))

	return sb.String()
}

func renderHabitStats(hs model.HabitStats, today time.Time) string {
	var sb strings.Builder

	oblTag := ""
	if hs.Habit.IsObligated {
		oblTag = "  " + styleOblBadge.Render("[OBL]")
	}
	sb.WriteString(styleBold.Render(hs.Habit.Name) + oblTag + "\n")

	if len(hs.Weeks) == 0 {
		sb.WriteString(styleNormal.Render("  No data yet.\n"))
		return sb.String()
	}

	for _, w := range hs.Weeks {
		weekEnd := w.WeekStart.AddDate(0, 0, 6)
		weekLabel := w.WeekStart.Format("Jan 2") + "–" + weekEnd.Format("Jan 2")
		sb.WriteString("  " + styleNormal.Render(weekLabel) + ":  ")

		for _, dr := range w.Days {
			sb.WriteString(renderSquare(dr.Status, dr.Date, today))
		}

		// Pad to 7 squares if week is partial.
		for i := len(w.Days); i < 7; i++ {
			sb.WriteString(squareGray.Render("[ ]"))
		}

		sb.WriteString("  ")
		sb.WriteString(renderWeekRate(w))
		sb.WriteString("\n")
	}

	var globalLine string
	if hs.GlobalRate >= 0 {
		globalLine = fmt.Sprintf("  Overall: %s\n", styleGlobalRate.Render(fmtPct(hs.GlobalRate)))
	} else {
		globalLine = "  Overall: " + styleNormal.Render("no data") + "\n"
	}
	sb.WriteString(globalLine)

	return sb.String()
}

func renderWeekRate(w model.WeekStats) string {
	if w.SuccessRate < 0 {
		return styleWeekRate.Render("(no data)")
	}
	denom := w.TotalDays + w.GreenCount - w.GreenCount // just TotalDays but be clear
	denom = w.GreenCount + w.RedCount
	return styleWeekRate.Render(fmt.Sprintf("%d/%d (%s)", w.GreenCount, denom, fmtPct(w.SuccessRate)))
}

func renderGlobalStats(gs model.GlobalStats) string {
	if gs.AverageRate < 0 {
		return styleGlobalRate.Render("Global average: no data yet") + "\n"
	}
	return styleGlobalRate.Render(fmt.Sprintf("Global average: %s", fmtPct(gs.AverageRate))) +
		"  " + styleNormal.Render(fmt.Sprintf("(■ %d  ▪ %d  □ %d)", gs.TotalGreen, gs.TotalYellow, gs.TotalRed)) +
		"\n"
}

