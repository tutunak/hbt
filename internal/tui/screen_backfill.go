package tui

import (
	"database/sql"
	"fmt"
	"hbt/internal/model"
	"hbt/internal/service"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// BackfillModel handles the "Did you do X on [date]?" queue.
type BackfillModel struct {
	db    *sql.DB
	items []model.BackfillItem
	index int
	today time.Time
	msg   string
}

func newBackfillModel(db *sql.DB, items []model.BackfillItem, today time.Time) BackfillModel {
	return BackfillModel{db: db, items: items, today: today}
}

func (m BackfillModel) Init() tea.Cmd { return nil }

func (m BackfillModel) current() model.BackfillItem {
	return m.items[m.index]
}

func (m BackfillModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			item := m.current()
			_ = service.RecordEntry(m.db, item.Habit.ID, item.Date, true)
			return m.advance("Recorded as done.")
		case "n", "N":
			item := m.current()
			_ = service.RecordEntry(m.db, item.Habit.ID, item.Date, false)
			return m.advance("Recorded as skipped.")
		case "s", "S":
			return m.advance("Skipped (will ask again later).")
		case "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m BackfillModel) advance(feedback string) (tea.Model, tea.Cmd) {
	m.msg = feedback
	m.index++
	if m.index >= len(m.items) {
		return m, func() tea.Msg { return switchToHabitListMsg{} }
	}
	return m, nil
}

func (m BackfillModel) View() string {
	if m.index >= len(m.items) {
		return styleSuccess.Render("All caught up!") + "\n"
	}
	item := m.current()

	header := styleTitle.Render("Catching up on missed days")
	progress := styleNormal.Render(fmt.Sprintf("(%d of %d)", m.index+1, len(m.items)))

	habitName := styleBold.Render(item.Habit.Name)
	dateStr := styleDate.Render(item.Date.Format("Monday, Jan 2 2006"))

	question := styleQuestion.Render(fmt.Sprintf("Did you do %s on %s?", habitName, dateStr))

	help := styleHelp.Render("[y] yes   [n] no   [s] skip (ask again later)   [q] quit")

	var feedback string
	if m.msg != "" {
		feedback = "\n" + styleNormal.Render(m.msg)
	}

	return fmt.Sprintf("%s  %s\n\n%s\n%s%s\n", header, progress, question, help, feedback)
}
