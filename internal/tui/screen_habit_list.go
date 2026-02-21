package tui

import (
	"database/sql"
	"fmt"
	"hbt/internal/model"
	"hbt/internal/service"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// habitItem wraps model.Habit for the bubbles list.
type habitItem struct {
	habit model.Habit
	today time.Time
}

func (h habitItem) FilterValue() string { return h.habit.Name }
func (h habitItem) Title() string       { return h.habit.Name }
func (h habitItem) Description() string {
	var parts []string
	if h.habit.IsObligated {
		parts = append(parts, styleOblBadge.Render("[OBL]"))
	}
	since := h.habit.StartDate.Format("Jan 2, 2006")
	parts = append(parts, styleNormal.Render("since "+since))
	return strings.Join(parts, "  ")
}

// habitDelegate is a custom list item renderer.
type habitDelegate struct{}

func (d habitDelegate) Height() int                             { return 2 }
func (d habitDelegate) Spacing() int                            { return 0 }
func (d habitDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d habitDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	hi, ok := item.(habitItem)
	if !ok {
		return
	}
	isSelected := index == m.Index()

	var nameLine string
	if isSelected {
		cursor := lipgloss.NewStyle().Foreground(colorAccent).Render("> ")
		nameLine = cursor + styleSelected.Render(hi.habit.Name)
	} else {
		nameLine = "  " + styleNormal.Render(hi.habit.Name)
	}

	var descParts []string
	if hi.habit.IsObligated {
		descParts = append(descParts, styleOblBadge.Render("[OBL]"))
	}
	since := "   since " + hi.habit.StartDate.Format("Jan 2, 2006")
	descParts = append(descParts, styleNormal.Render(since))

	descLine := "  " + strings.Join(descParts, "  ")

	fmt.Fprintf(w, "%s\n%s\n", nameLine, descLine)
}

// HabitListModel is the main habit selection screen.
type HabitListModel struct {
	db                 *sql.DB
	list               list.Model
	today              time.Time
	width              int
	height             int
	nonObligatedCount  int
}

func newHabitListModel(db *sql.DB, today time.Time) HabitListModel {
	habits, _ := service.ListHabits(db, false)

	var nonObligatedCount int
	items := make([]list.Item, len(habits))
	for i, h := range habits {
		items[i] = habitItem{habit: h, today: today}
		if !h.IsObligated {
			nonObligatedCount++
		}
	}

	l := list.New(items, habitDelegate{}, 80, 24)
	l.Title = "Habit Tracker"
	l.Styles.Title = styleTitle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

	return HabitListModel{db: db, list: l, today: today, nonObligatedCount: nonObligatedCount}
}

func (m HabitListModel) Init() tea.Cmd { return nil }

func (m HabitListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-4)

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if item, ok := m.list.SelectedItem().(habitItem); ok {
				return m, func() tea.Msg { return switchToTrackMsg{habit: item.habit} }
			}
		case "a":
			return m, func() tea.Msg { return switchToAddHabitMsg{} }
		case "s":
			return m, func() tea.Msg { return switchToStatsMsg{} }
		case "r":
			if item, ok := m.list.SelectedItem().(habitItem); ok {
				return m, func() tea.Msg { return switchToArchiveMsg{habit: item.habit} }
			}
		case "q":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m HabitListModel) View() string {
	help := styleHelp.Render("[enter] track   [a] add   [s] stats   [r] archive   [↑/↓] navigate   [q] quit")
	if len(m.list.Items()) == 0 {
		empty := styleNormal.Render("No habits yet. Press [a] to add your first habit.")
		return styleTitle.Render("Habit Tracker") + "\n\n" + empty + "\n" + help
	}
	var warning string
	if m.nonObligatedCount < 4 {
		warning = styleWarning.Render(fmt.Sprintf("⚠ Only %d non-obligated habit(s). Consider adding more.", m.nonObligatedCount)) + "\n"
	}
	return warning + m.list.View() + "\n" + help
}
