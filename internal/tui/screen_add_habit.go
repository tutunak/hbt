package tui

import (
	"database/sql"
	"fmt"
	"hbt/internal/service"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type addStep int

const (
	addStepName addStep = iota
	addStepStartDate
	addStepObligated
	addStepObligatedSince
	addStepDone
)

// AddHabitModel is the multi-step "add a habit" form.
type AddHabitModel struct {
	db    *sql.DB
	today time.Time
	step  addStep

	nameInput    textinput.Model
	dateInput    textinput.Model
	oblInput     textinput.Model
	oblSince     textinput.Model
	isObligated  bool
	errMsg       string
}

func newAddHabitModel(db *sql.DB, today time.Time) AddHabitModel {
	name := textinput.New()
	name.Placeholder = "e.g. Morning run"
	name.Focus()

	date := textinput.New()
	date.Placeholder = today.Format("2006-01-02") + " (Enter for today)"

	obl := textinput.New()
	obl.Placeholder = "y/n"

	oblS := textinput.New()
	oblS.Placeholder = today.Format("2006-01-02") + " (Enter for today)"

	return AddHabitModel{
		db:          db,
		today:       today,
		nameInput:   name,
		dateInput:   date,
		oblInput:    obl,
		oblSince:    oblS,
	}
}

func (m AddHabitModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m AddHabitModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return switchToHabitListMsg{} }
		case "enter":
			return m.handleEnter()
		}
	}

	var cmd tea.Cmd
	switch m.step {
	case addStepName:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case addStepStartDate:
		m.dateInput, cmd = m.dateInput.Update(msg)
	case addStepObligated:
		m.oblInput, cmd = m.oblInput.Update(msg)
	case addStepObligatedSince:
		m.oblSince, cmd = m.oblSince.Update(msg)
	}
	return m, cmd
}

func (m AddHabitModel) handleEnter() (tea.Model, tea.Cmd) {
	m.errMsg = ""
	switch m.step {
	case addStepName:
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			m.errMsg = "Name cannot be empty."
			return m, nil
		}
		m.step = addStepStartDate
		m.nameInput.Blur()
		m.dateInput.Focus()

	case addStepStartDate:
		val := strings.TrimSpace(m.dateInput.Value())
		if val != "" {
			_, err := time.ParseInLocation("2006-01-02", val, time.UTC)
			if err != nil {
				m.errMsg = "Invalid date. Use YYYY-MM-DD format."
				return m, nil
			}
		}
		m.step = addStepObligated
		m.dateInput.Blur()
		m.oblInput.Focus()

	case addStepObligated:
		val := strings.ToLower(strings.TrimSpace(m.oblInput.Value()))
		if val != "y" && val != "n" {
			m.errMsg = "Please enter y or n."
			return m, nil
		}
		m.isObligated = val == "y"
		if m.isObligated {
			m.step = addStepObligatedSince
			m.oblInput.Blur()
			m.oblSince.Focus()
		} else {
			return m.save()
		}

	case addStepObligatedSince:
		val := strings.TrimSpace(m.oblSince.Value())
		if val != "" {
			_, err := time.ParseInLocation("2006-01-02", val, time.UTC)
			if err != nil {
				m.errMsg = "Invalid date. Use YYYY-MM-DD format."
				return m, nil
			}
		}
		return m.save()
	}
	return m, textinput.Blink
}

func (m AddHabitModel) save() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.nameInput.Value())

	startDate := m.today
	if v := strings.TrimSpace(m.dateInput.Value()); v != "" {
		startDate, _ = time.ParseInLocation("2006-01-02", v, time.UTC)
	}

	var oblSince *time.Time
	if m.isObligated {
		t := m.today
		if v := strings.TrimSpace(m.oblSince.Value()); v != "" {
			t, _ = time.ParseInLocation("2006-01-02", v, time.UTC)
		}
		oblSince = &t
	}

	_, err := service.CreateHabit(m.db, name, startDate, m.isObligated, oblSince)
	if err != nil {
		m.errMsg = "Failed to save: " + err.Error()
		return m, nil
	}
	m.step = addStepDone
	return m, func() tea.Msg { return switchToHabitListMsg{} }
}

func (m AddHabitModel) View() string {
	header := styleTitle.Render("Add New Habit")

	var body strings.Builder
	body.WriteString(header + "\n\n")

	switch m.step {
	case addStepName:
		body.WriteString(styleQuestion.Render("Habit name:") + "\n")
		body.WriteString(m.nameInput.View() + "\n")
	case addStepStartDate:
		body.WriteString(styleNormal.Render("Name: "+m.nameInput.Value()) + "\n\n")
		body.WriteString(styleQuestion.Render("Start date (YYYY-MM-DD, or Enter for today):") + "\n")
		body.WriteString(m.dateInput.View() + "\n")
	case addStepObligated:
		body.WriteString(styleNormal.Render(fmt.Sprintf("Name: %s", m.nameInput.Value())) + "\n\n")
		body.WriteString(styleQuestion.Render("Is this an obligated habit? (y/n):") + "\n")
		body.WriteString(m.oblInput.View() + "\n")
	case addStepObligatedSince:
		body.WriteString(styleNormal.Render(fmt.Sprintf("Name: %s", m.nameInput.Value())) + "\n\n")
		body.WriteString(styleQuestion.Render("Obligated since (YYYY-MM-DD, or Enter for today):") + "\n")
		body.WriteString(m.oblSince.View() + "\n")
	}

	if m.errMsg != "" {
		body.WriteString("\n" + styleError.Render(m.errMsg) + "\n")
	}

	body.WriteString(styleHelp.Render("[enter] confirm   [esc] cancel"))
	return body.String()
}
