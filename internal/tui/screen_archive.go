package tui

import (
	"database/sql"
	"fmt"
	"hbt/internal/model"
	"hbt/internal/service"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ArchiveModel handles archiving a habit with a comment.
type ArchiveModel struct {
	db     *sql.DB
	habit  model.Habit
	input  textinput.Model
	errMsg string
	done   bool
}

func newArchiveModel(db *sql.DB, habit model.Habit) ArchiveModel {
	ti := textinput.New()
	ti.Placeholder = "Why are you archiving this habit?"
	ti.Focus()
	return ArchiveModel{db: db, habit: habit, input: ti}
}

func (m ArchiveModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m ArchiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.done {
		return m, func() tea.Msg { return switchToHabitListMsg{} }
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return switchToHabitListMsg{} }
		case "enter":
			comment := strings.TrimSpace(m.input.Value())
			if err := service.ArchiveHabit(m.db, m.habit.ID, comment); err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
			m.done = true
			return m, func() tea.Msg { return switchToHabitListMsg{} }
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m ArchiveModel) View() string {
	header := styleTitle.Render("Archive Habit")
	habitName := styleBold.Render(m.habit.Name)
	warning := styleError.Render(fmt.Sprintf("Archive \"%s\"?", habitName))
	note := styleNormal.Render("Archived habits are hidden from the main list.")
	question := styleQuestion.Render("Add a comment (optional):")

	var errLine string
	if m.errMsg != "" {
		errLine = "\n" + styleError.Render(m.errMsg)
	}

	help := styleHelp.Render("[enter] archive   [esc] cancel")

	return fmt.Sprintf("%s\n\n%s\n%s\n\n%s\n%s%s\n%s\n",
		header, warning, note, question, m.input.View(), errLine, help)
}
