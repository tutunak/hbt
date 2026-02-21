package tui

import (
	"database/sql"
	"fmt"
	"hbt/internal/model"
	"hbt/internal/service"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type habitListMode int

const (
	modeNormal    habitListMode = iota
	modeBackfill                // y/n/s answers backfill question
	modePromotion               // ↑/↓ + enter selects habit to promote
)

// HabitListModel is the main screen with inline tracking, backfill banner, and promotion banner.
type HabitListModel struct {
	db     *sql.DB
	today  time.Time
	width  int
	height int

	habits            []model.Habit
	cursor            int
	nonObligatedCount int

	// Inline stats (loaded on init and refreshed after recording)
	habitStatuses map[int64]map[time.Time]model.DayStatus
	lastEntryDate map[int64]*time.Time

	// Backfill state
	backfillItems []model.BackfillItem
	backfillIdx   int

	// Promotion state
	needsPromotion bool
	promoHabits    []model.Habit
	promoCursor    int

	mode habitListMode
}

func newHabitListModel(db *sql.DB, today time.Time) HabitListModel {
	m := HabitListModel{
		db:            db,
		today:         today,
		habitStatuses: map[int64]map[time.Time]model.DayStatus{},
		lastEntryDate: map[int64]*time.Time{},
	}

	habits, _ := service.ListHabits(db, false)
	m.habits = habits

	for _, h := range habits {
		if !h.IsObligated {
			m.nonObligatedCount++
		}
		entries, _ := service.GetEntriesForHabit(db, h.ID)
		m.habitStatuses[h.ID] = service.ComputeDayStatuses(h, entries, today)
		led, _ := service.GetLastEntryDate(db, h.ID)
		m.lastEntryDate[h.ID] = led
	}

	backfillItems, _ := service.GetPendingBackfill(db, today)
	m.backfillItems = backfillItems

	needsPromotion, _ := service.NeedsWeeklyPromotion(db, today)
	m.needsPromotion = needsPromotion

	if needsPromotion {
		promoHabits, _ := service.ListNonObligatedHabits(db)
		m.promoHabits = promoHabits
		m.mode = modePromotion
	} else if len(backfillItems) > 0 {
		m.mode = modeBackfill
	} else {
		m.mode = modeNormal
	}

	return m
}

func (m HabitListModel) Init() tea.Cmd { return nil }

func (m HabitListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch m.mode {
		case modeNormal:
			switch msg.String() {
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.habits)-1 {
					m.cursor++
				}
			case "y":
				if m.cursor < len(m.habits) {
					h := m.habits[m.cursor]
					_ = service.RecordEntry(m.db, h.ID, m.today, true)
					m.refreshHabitStats(h)
				}
			case "n":
				if m.cursor < len(m.habits) {
					h := m.habits[m.cursor]
					_ = service.RecordEntry(m.db, h.ID, m.today, false)
					m.refreshHabitStats(h)
				}
			case "a":
				return m, func() tea.Msg { return switchToAddHabitMsg{} }
			case "r":
				if m.cursor < len(m.habits) {
					h := m.habits[m.cursor]
					return m, func() tea.Msg { return switchToArchiveMsg{habit: h} }
				}
			case "s":
				return m, func() tea.Msg { return switchToStatsMsg{} }
			case "q":
				return m, tea.Quit
			}

		case modeBackfill:
			switch msg.String() {
			case "y", "Y":
				item := m.backfillItems[m.backfillIdx]
				_ = service.RecordEntry(m.db, item.Habit.ID, item.Date, true)
				m.refreshHabitStats(item.Habit)
				return m.advanceBackfill()
			case "n", "N":
				item := m.backfillItems[m.backfillIdx]
				_ = service.RecordEntry(m.db, item.Habit.ID, item.Date, false)
				m.refreshHabitStats(item.Habit)
				return m.advanceBackfill()
			case "s", "S":
				return m.advanceBackfill()
			case "q":
				return m, tea.Quit
			}

		case modePromotion:
			switch msg.String() {
			case "up", "k":
				if m.promoCursor > 0 {
					m.promoCursor--
				}
			case "down", "j":
				if m.promoCursor < len(m.promoHabits)-1 {
					m.promoCursor++
				}
			case "enter":
				if len(m.promoHabits) > 0 {
					habit := m.promoHabits[m.promoCursor]
					_ = service.SetObligated(m.db, habit.ID, true, m.today)
				}
				_ = service.RecordWeeklyPromotion(m.db, m.today)
				return m, func() tea.Msg { return switchToHabitListMsg{} }
			case "s":
				_ = service.RecordWeeklyPromotion(m.db, m.today)
				return m, func() tea.Msg { return switchToHabitListMsg{} }
			case "q":
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

// refreshHabitStats reloads inline stats for a single habit.
// Because habitStatuses and lastEntryDate are maps (reference types),
// mutations here are visible in the value copy returned from Update.
func (m HabitListModel) refreshHabitStats(h model.Habit) {
	entries, _ := service.GetEntriesForHabit(m.db, h.ID)
	m.habitStatuses[h.ID] = service.ComputeDayStatuses(h, entries, m.today)
	led, _ := service.GetLastEntryDate(m.db, h.ID)
	m.lastEntryDate[h.ID] = led
}

// advanceBackfill moves to the next backfill item or switches to modeNormal when done.
func (m HabitListModel) advanceBackfill() (tea.Model, tea.Cmd) {
	m.backfillIdx++
	if m.backfillIdx >= len(m.backfillItems) {
		m.mode = modeNormal
	}
	return m, nil
}

// mondayOf returns the Monday of the ISO week containing t.
func mondayOf(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return t.AddDate(0, 0, -(weekday - 1))
}

func (m HabitListModel) View() string {
	var sb strings.Builder

	// Header: title + date
	dateStr := m.today.Format("Mon Jan 2 2006")
	title := styleTitle.Render("Habit Tracker")
	if m.width > 0 {
		padding := m.width - lipgloss.Width(title) - len(dateStr)
		if padding < 1 {
			padding = 1
		}
		sb.WriteString(title + strings.Repeat(" ", padding) + styleNormal.Render(dateStr) + "\n")
	} else {
		sb.WriteString(title + "  " + styleNormal.Render(dateStr) + "\n")
	}

	// Warning
	if m.nonObligatedCount < 4 {
		sb.WriteString(styleWarning.Render(fmt.Sprintf("⚠ Only %d non-obligated habit(s). Consider adding more.", m.nonObligatedCount)) + "\n")
	}

	// Banner
	switch m.mode {
	case modePromotion:
		sb.WriteString(m.renderPromotionBanner())
	case modeBackfill:
		if m.backfillIdx < len(m.backfillItems) {
			sb.WriteString(m.renderBackfillBanner())
		}
	}

	// Separator
	sepWidth := 54
	if m.width > 0 {
		sepWidth = m.width
	}
	sb.WriteString(styleSeparator.Render(strings.Repeat("─", sepWidth)) + "\n")

	// Habits
	if len(m.habits) == 0 {
		sb.WriteString(styleNormal.Render("No habits yet. Press [a] to add your first habit.") + "\n")
	} else {
		maxNameLen := 0
		for _, h := range m.habits {
			if len(h.Name) > maxNameLen {
				maxNameLen = len(h.Name)
			}
		}
		for i, h := range m.habits {
			sb.WriteString(m.renderHabitRow(i, h, maxNameLen) + "\n")
		}
	}

	// Separator
	sb.WriteString(styleSeparator.Render(strings.Repeat("─", sepWidth)) + "\n")

	// Help
	switch m.mode {
	case modeNormal:
		sb.WriteString(styleHelp.Render("[y] done  [n] skip  [a] add  [s] stats  [r] archive  [q] quit"))
	case modeBackfill:
		sb.WriteString(styleHelp.Render("[y] yes   [n] no   [s] skip   [q] quit"))
	case modePromotion:
		sb.WriteString(styleHelp.Render("[enter] promote   [s] skip week   [↑/↓] nav   [q] quit"))
	}

	return sb.String()
}

func (m HabitListModel) renderPromotionBanner() string {
	monday := mondayOf(m.today)
	var sb strings.Builder
	sb.WriteString(styleBanner.Render(fmt.Sprintf("WEEKLY PROMOTION — Week of %s", monday.Format("Jan 2, 2006"))) + "\n")
	for i, h := range m.promoHabits {
		since := h.StartDate.Format("Jan 2, 2006")
		const nameWidth = 20
		name := h.Name
		if len(name) < nameWidth {
			name += strings.Repeat(" ", nameWidth-len(name))
		}
		row := fmt.Sprintf("%s  since %s", name, since)
		if i == m.promoCursor {
			sb.WriteString(lipgloss.NewStyle().Foreground(colorAccent).Render("> ") + styleSelected.Render(row) + "\n")
		} else {
			sb.WriteString("  " + styleNormal.Render(row) + "\n")
		}
	}
	return sb.String()
}

func (m HabitListModel) renderBackfillBanner() string {
	item := m.backfillItems[m.backfillIdx]
	progress := fmt.Sprintf("BACKFILL (%d of %d)", m.backfillIdx+1, len(m.backfillItems))
	habitDate := fmt.Sprintf("%s — %s?", item.Habit.Name, item.Date.Format("Mon, Jan 2"))
	return styleBanner.Render(progress+"  "+habitDate) + "\n"
}

func (m HabitListModel) renderHabitRow(idx int, h model.Habit, maxNameLen int) string {
	isSelected := idx == m.cursor && m.mode == modeNormal

	// Pad name to maxNameLen so every row's stats start at the same column.
	name := h.Name
	paddedName := name + strings.Repeat(" ", maxNameLen-len(name))

	var namePart string
	if isSelected {
		namePart = lipgloss.NewStyle().Foreground(colorAccent).Render("> ") + styleSelected.Render(paddedName)
	} else {
		namePart = "  " + styleNormal.Render(paddedName)
	}

	// OBL badge (7 chars including surrounding spaces to keep column alignment)
	var oblPart string
	if h.IsObligated {
		oblPart = "  " + styleOblBadge.Render("[OBL]")
	} else {
		oblPart = "       "
	}

	// Stat squares or status message
	var statPart string
	lastEntry := m.lastEntryDate[h.ID]
	if lastEntry == nil {
		statPart = "  " + styleNormal.Render("(no history)")
	} else {
		daysAgo := int(m.today.Sub(*lastEntry).Hours() / 24)
		if daysAgo > 42 {
			weeks := daysAgo / 7
			statPart = "  " + styleNormal.Render(fmt.Sprintf("(inactive %dw)", weeks))
		} else {
			statuses := m.habitStatuses[h.ID]
			var squares strings.Builder
			var green, red int
			for i := -6; i <= 0; i++ {
				d := m.today.AddDate(0, 0, i)
				day := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
				status := statuses[day]
				squares.WriteString(renderSquare(status, day, m.today))
				switch status {
				case model.DayDone:
					green++
				case model.DayRed:
					red++
				}
			}
			denom := green + red
			var rateStr string
			if denom > 0 {
				rate := float64(green) / float64(denom)
				rateStr = fmt.Sprintf("  %d/%d (%s)", green, denom, fmtPct(rate))
			} else {
				rateStr = "  (no data)"
			}
			statPart = "  " + squares.String() + styleWeekRate.Render(rateStr)
		}
	}

	return namePart + oblPart + statPart
}
