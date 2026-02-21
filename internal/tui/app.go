package tui

import (
	"database/sql"
	"hbt/internal/model"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type screen int

const (
	screenHabitList screen = iota
	screenAddHabit
	screenArchive
	screenStats
)

// --- Messages for screen transitions ---

type switchToHabitListMsg struct{}
type switchToAddHabitMsg struct{}
type switchToArchiveMsg struct{ habit model.Habit }
type switchToStatsMsg struct{}

// AppModel is the root TUI model.
type AppModel struct {
	db     *sql.DB
	screen screen
	today  time.Time
	width  int
	height int

	habitList HabitListModel
	addHabit  AddHabitModel
	archive   ArchiveModel
	stats     StatsModel

	err string
}

// New creates a new AppModel starting at the habit list screen.
// Backfill and promotion state are loaded inside newHabitListModel.
func New(db *sql.DB) (AppModel, error) {
	today := time.Now().UTC()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	var m AppModel
	m.db = db
	m.today = today
	m.screen = screenHabitList
	m.habitList = newHabitListModel(db, today)

	return m, nil
}

func (m AppModel) Init() tea.Cmd {
	switch m.screen {
	case screenHabitList:
		return m.habitList.Init()
	}
	return nil
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.propagateSize()
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case switchToHabitListMsg:
		m.screen = screenHabitList
		m.habitList = newHabitListModel(m.db, m.today)
		m.habitList.width = m.width
		m.habitList.height = m.height
		return m, m.habitList.Init()

	case switchToAddHabitMsg:
		m.screen = screenAddHabit
		m.addHabit = newAddHabitModel(m.db, m.today)
		return m, m.addHabit.Init()

	case switchToArchiveMsg:
		m.screen = screenArchive
		m.archive = newArchiveModel(m.db, msg.habit)
		return m, m.archive.Init()

	case switchToStatsMsg:
		m.screen = screenStats
		m.stats = newStatsModel(m.db, m.today)
		m.stats.width = m.width
		m.stats.height = m.height
		return m, m.stats.Init()
	}

	// Delegate to active sub-model.
	var cmd tea.Cmd
	switch m.screen {
	case screenHabitList:
		var sub tea.Model
		sub, cmd = m.habitList.Update(msg)
		m.habitList = sub.(HabitListModel)
	case screenAddHabit:
		var sub tea.Model
		sub, cmd = m.addHabit.Update(msg)
		m.addHabit = sub.(AddHabitModel)
	case screenArchive:
		var sub tea.Model
		sub, cmd = m.archive.Update(msg)
		m.archive = sub.(ArchiveModel)
	case screenStats:
		var sub tea.Model
		sub, cmd = m.stats.Update(msg)
		m.stats = sub.(StatsModel)
	}
	return m, cmd
}

func (m AppModel) View() string {
	if m.err != "" {
		return styleError.Render("Error: "+m.err) + "\n" + styleHelp.Render("Press q to quit")
	}
	switch m.screen {
	case screenHabitList:
		return m.habitList.View()
	case screenAddHabit:
		return m.addHabit.View()
	case screenArchive:
		return m.archive.View()
	case screenStats:
		return m.stats.View()
	}
	return ""
}

func (m *AppModel) propagateSize() {
	m.habitList.width = m.width
	m.habitList.height = m.height
	m.stats.width = m.width
	m.stats.height = m.height
}
