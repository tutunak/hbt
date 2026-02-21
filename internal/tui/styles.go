package tui

import (
	"fmt"
	"hbt/internal/model"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	colorGreen  = lipgloss.Color("#22C55E")
	colorYellow = lipgloss.Color("#EAB308")
	colorRed    = lipgloss.Color("#EF4444")
	colorGray   = lipgloss.Color("#6B7280")
	colorBlue   = lipgloss.Color("#3B82F6")
	colorWhite  = lipgloss.Color("#F9FAFB")
	colorDim    = lipgloss.Color("#9CA3AF")
	colorAccent = lipgloss.Color("#A78BFA")

	styleBold = lipgloss.NewStyle().Bold(true)

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			PaddingBottom(1)

	styleSelected = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite)

	styleNormal = lipgloss.NewStyle().
			Foreground(colorDim)

	styleOblBadge = lipgloss.NewStyle().
			Foreground(colorBlue).
			Bold(true)

	styleHelp = lipgloss.NewStyle().
			Foreground(colorDim).
			PaddingTop(1)

	styleError = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	styleSuccess = lipgloss.NewStyle().
			Foreground(colorGreen)

	styleQuestion = lipgloss.NewStyle().
			Foreground(colorWhite).
			Bold(true)

	styleDate = lipgloss.NewStyle().
			Foreground(colorYellow)

	// Day square styles
	squareDone = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	squareYellow = lipgloss.NewStyle().
			Foreground(colorYellow).
			Bold(true)

	squareRed = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	squareGray = lipgloss.NewStyle().
			Foreground(colorGray)

	styleWeekRate = lipgloss.NewStyle().
			Foreground(colorDim)

	styleGlobalRate = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	styleSeparator = lipgloss.NewStyle().
			Foreground(colorGray)

	colorWarning = lipgloss.Color("#F97316")
	styleWarning = lipgloss.NewStyle().Foreground(colorWarning).Bold(true)

	styleBanner = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
)

func renderSquare(status model.DayStatus, date, today time.Time) string {
	switch status {
	case model.DayDone:
		return squareDone.Render("[■]")
	case model.DayYellow:
		return squareYellow.Render("[▪]")
	case model.DayRed:
		return squareRed.Render("[□]")
	case model.DayFuture:
		return squareGray.Render("   ")
	default: // DayUnknown
		if date.Equal(today) {
			return squareGray.Render("[?]")
		}
		return squareGray.Render("[ ]")
	}
}

func fmtPct(r float64) string {
	return fmt.Sprintf("%.0f%%", r*100)
}
