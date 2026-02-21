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

// ObligationPromotionModel handles the weekly "choose a habit to promote" screen.
type ObligationPromotionModel struct {
	db      *sql.DB
	habits  []model.Habit
	cursor  int
	today   time.Time
	errMsg  string
}

func newObligationPromotionModel(db *sql.DB, habits []model.Habit, today time.Time) ObligationPromotionModel {
	return ObligationPromotionModel{db: db, habits: habits, today: today}
}

func (m ObligationPromotionModel) Init() tea.Cmd { return nil }

func (m ObligationPromotionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.habits)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.habits) == 0 {
				return m, m.recordAndContinue()
			}
			habit := m.habits[m.cursor]
			if err := service.SetObligated(m.db, habit.ID, true, m.today); err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
			return m, m.recordAndContinue()
		case "s":
			return m, m.recordAndContinue()
		case "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m ObligationPromotionModel) recordAndContinue() tea.Cmd {
	_ = service.RecordWeeklyPromotion(m.db, m.today)
	return func() tea.Msg { return switchToBackfillMsg{} }
}

func (m ObligationPromotionModel) View() string {
	monday := mondayOfExported(m.today)
	header := styleTitle.Render(fmt.Sprintf("Weekly Habit Promotion — Week of %s", monday.Format("Jan 2, 2006")))

	if m.errMsg != "" {
		return header + "\n\n" + styleError.Render("Error: "+m.errMsg) + "\n"
	}

	if len(m.habits) == 0 {
		msg := styleNormal.Render("No non-obligated habits to promote.")
		help := styleHelp.Render("[enter] continue   [q] quit")
		return header + "\n\n" + msg + "\n" + help
	}

	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString("\n\n")
	sb.WriteString(styleQuestion.Render("Choose one habit to promote to obligated this week:"))
	sb.WriteString("\n\n")

	for i, h := range m.habits {
		since := h.StartDate.Format("Jan 2, 2006")
		name := h.Name + strings.Repeat(" ", max(0, 24-len(h.Name)))
		row := fmt.Sprintf("%s  since %s", name, since)
		if i == m.cursor {
			cursor := lipgloss.NewStyle().Foreground(colorAccent).Render("> ")
			sb.WriteString(cursor + styleSelected.Render(row))
		} else {
			sb.WriteString("  " + styleNormal.Render(row))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(styleHelp.Render("[enter] promote   [s] skip this week   [↑/↓] navigate   [q] quit"))

	return sb.String()
}

// mondayOfExported wraps the unexported mondayOf for use in this file.
func mondayOfExported(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return t.AddDate(0, 0, -(weekday - 1))
}
