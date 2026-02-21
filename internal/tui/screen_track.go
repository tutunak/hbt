package tui

import (
	"database/sql"
	"fmt"
	"hbt/internal/model"
	"hbt/internal/service"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TrackModel handles "Did you do [habit] today?"
type TrackModel struct {
	db      *sql.DB
	habit   model.Habit
	today   time.Time
	current *model.Entry // existing entry for today, if any
	msg     string
	done    bool
}

func newTrackModel(db *sql.DB, habit model.Habit, today time.Time) TrackModel {
	existing, _ := service.GetEntryForDay(db, habit.ID, today)
	return TrackModel{db: db, habit: habit, today: today, current: existing}
}

func (m TrackModel) Init() tea.Cmd { return nil }

func (m TrackModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.done {
		return m, func() tea.Msg { return switchToHabitListMsg{} }
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y", "enter":
			_ = service.RecordEntry(m.db, m.habit.ID, m.today, true)
			m.msg = styleSuccess.Render("Recorded as done!")
			m.done = true
			return m, tea.Tick(0, func(t time.Time) tea.Msg { return switchToHabitListMsg{} })
		case "n", "N":
			_ = service.RecordEntry(m.db, m.habit.ID, m.today, false)
			m.msg = styleNormal.Render("Recorded as skipped.")
			m.done = true
			return m, tea.Tick(0, func(t time.Time) tea.Msg { return switchToHabitListMsg{} })
		case "esc", "q":
			return m, func() tea.Msg { return switchToHabitListMsg{} }
		}
	}
	return m, nil
}

func (m TrackModel) View() string {
	header := styleTitle.Render("Track Habit")
	habitName := styleBold.Render(m.habit.Name)
	dateStr := styleDate.Render(m.today.Format("Monday, Jan 2 2006"))

	var currentStatus string
	if m.current != nil {
		if m.current.DidIt {
			currentStatus = "\n" + styleSuccess.Render("Already recorded as done today.")
		} else {
			currentStatus = "\n" + styleNormal.Render("Already recorded as skipped today.")
		}
		currentStatus += " " + styleNormal.Render("(You can change it below)")
	}

	question := styleQuestion.Render(fmt.Sprintf("Did you do %s today (%s)?", habitName, dateStr))
	help := styleHelp.Render("[y/enter] yes   [n] no   [esc] back")

	if m.msg != "" {
		return fmt.Sprintf("%s\n\n%s%s\n\n%s\n", header, question, currentStatus, m.msg)
	}
	return fmt.Sprintf("%s\n\n%s%s\n%s\n", header, question, currentStatus, help)
}
