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

	// Day square background styles — rendered as solid 2-space colour blocks.
	sqBgDone    = lipgloss.NewStyle().Background(colorGreen)
	sqBgYellow  = lipgloss.NewStyle().Background(colorYellow)
	sqBgRed     = lipgloss.NewStyle().Background(colorRed)
	sqBgUnknown = lipgloss.NewStyle().Background(colorGray)

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

// renderSquare returns a 3-char-wide visual slot: 2-space background colour block + 1 space separator.
func renderSquare(status model.DayStatus, date, today time.Time) string {
	const blk = "  " // solid 2-space block (background colour fills it)
	const sep = " "  // plain space separator between squares
	switch status {
	case model.DayDone:
		return sqBgDone.Render(blk) + sep
	case model.DayYellow:
		return sqBgYellow.Render(blk) + sep
	case model.DayRed:
		return sqBgRed.Render(blk) + sep
	case model.DayFuture:
		return "   " // 3 plain spaces, no colour
	default: // DayUnknown (past gaps and today not yet tracked)
		return sqBgUnknown.Render(blk) + sep
	}
}

func fmtPct(r float64) string {
	return fmt.Sprintf("%.0f%%", r*100)
}
