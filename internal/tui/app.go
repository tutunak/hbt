package tui

import (
	"database/sql"
	"fmt"
	"hbt/internal/model"
	"hbt/internal/service"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type screen int

const (
	screenObligationPromotion screen = iota
	screenBackfill
	screenHabitList
	screenTrack
	screenAddHabit
	screenArchive
	screenStats
)

// --- Messages for screen transitions ---

type switchToHabitListMsg struct{}
type switchToTrackMsg struct{ habit model.Habit }
type switchToAddHabitMsg struct{}
type switchToArchiveMsg struct{ habit model.Habit }
type switchToStatsMsg struct{}
type switchToBackfillMsg struct{}
type switchToObligationPromotionMsg struct{}

// AppModel is the root TUI model.
type AppModel struct {
	db     *sql.DB
	screen screen
	today  time.Time
	width  int
	height int

	obligationPromotion ObligationPromotionModel
	backfill            BackfillModel
	habitList           HabitListModel
	track               TrackModel
	addHabit            AddHabitModel
	archive             ArchiveModel
	stats               StatsModel

	err string
}

// New creates a new AppModel and loads the initial state.
func New(db *sql.DB) (AppModel, error) {
	today := time.Now().UTC()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	var m AppModel
	m.db = db
	m.today = today

	needsPromotion, err := service.NeedsWeeklyPromotion(db, today)
	if err != nil {
		return AppModel{}, fmt.Errorf("check promotion: %w", err)
	}

	if needsPromotion {
		habits, err := service.ListNonObligatedHabits(db)
		if err != nil {
			return AppModel{}, fmt.Errorf("list non-obligated habits: %w", err)
		}
		m.screen = screenObligationPromotion
		m.obligationPromotion = newObligationPromotionModel(db, habits, today)
		return m, nil
	}

	items, err := service.GetPendingBackfill(db, today)
	if err != nil {
		return AppModel{}, fmt.Errorf("load backfill: %w", err)
	}

	if len(items) > 0 {
		m.screen = screenBackfill
		m.backfill = newBackfillModel(db, items, today)
	} else {
		m.screen = screenHabitList
		m.habitList = newHabitListModel(db, today)
	}

	return m, nil
}

func (m AppModel) Init() tea.Cmd {
	switch m.screen {
	case screenObligationPromotion:
		return m.obligationPromotion.Init()
	case screenBackfill:
		return m.backfill.Init()
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

	case switchToTrackMsg:
		m.screen = screenTrack
		m.track = newTrackModel(m.db, msg.habit, m.today)
		return m, m.track.Init()

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

	case switchToBackfillMsg:
		items, err := service.GetPendingBackfill(m.db, m.today)
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		if len(items) == 0 {
			m.screen = screenHabitList
			m.habitList = newHabitListModel(m.db, m.today)
			m.habitList.width = m.width
			m.habitList.height = m.height
			return m, m.habitList.Init()
		}
		m.screen = screenBackfill
		m.backfill = newBackfillModel(m.db, items, m.today)
		return m, m.backfill.Init()

	case switchToObligationPromotionMsg:
		habits, err := service.ListNonObligatedHabits(m.db)
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.screen = screenObligationPromotion
		m.obligationPromotion = newObligationPromotionModel(m.db, habits, m.today)
		return m, m.obligationPromotion.Init()
	}

	// Delegate to active sub-model.
	var cmd tea.Cmd
	switch m.screen {
	case screenObligationPromotion:
		var sub tea.Model
		sub, cmd = m.obligationPromotion.Update(msg)
		m.obligationPromotion = sub.(ObligationPromotionModel)
	case screenBackfill:
		var sub tea.Model
		sub, cmd = m.backfill.Update(msg)
		m.backfill = sub.(BackfillModel)
	case screenHabitList:
		var sub tea.Model
		sub, cmd = m.habitList.Update(msg)
		m.habitList = sub.(HabitListModel)
	case screenTrack:
		var sub tea.Model
		sub, cmd = m.track.Update(msg)
		m.track = sub.(TrackModel)
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
	case screenObligationPromotion:
		return m.obligationPromotion.View()
	case screenBackfill:
		return m.backfill.View()
	case screenHabitList:
		return m.habitList.View()
	case screenTrack:
		return m.track.View()
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
