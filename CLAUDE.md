# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build ./...

# Run all tests
go test ./...

# Run tests with verbose output
go test ./... -v

# Run a single test
go test ./internal/service -run TestComputeDayStatuses

# Run tests in a specific package
go test ./internal/tui
```

No Makefile or lint configuration is present. Use standard `gofmt` for formatting.

## Architecture

Three-layer architecture: `internal/db` → `internal/service` → `internal/tui`.

**db layer**: Opens/migrates SQLite. `db.SetMaxOpenConns(1)` is mandatory — SQLite requires single-writer. Schema in `migrations.go` (3 tables: `habits`, `entries`, `weekly_promotions`).

**service layer**: Pure business logic, no TUI awareness.
- `stats.go` — `ComputeDayStatuses` runs **globally across all history** (not per-week) so consecutive-skip detection works across week boundaries. Yellow vs red logic: non-obligated habits always get yellow skips; obligated habits get yellow only for isolated (single) skips, red for 2+ consecutive. Yellow days are **excluded from the success rate denominator** (rate = green / (green + red)).
- `entry.go` — `GetPendingBackfill` only fires for habits that already have at least one past entry.
- `promotion.go` — `NeedsWeeklyPromotion` checks if the current Monday is NOT yet in `weekly_promotions` AND non-obligated habits exist.

**tui layer**: Bubble Tea architecture. `app.go` is the root model routing between 4 screens. The main screen (`screen_habit_list.go`) has 3 modes set at construction time in `newHabitListModel`: `modePromotion` (highest priority) → `modeBackfill` → `modeNormal`.

## Key Invariants

- Dates stored as TEXT `"YYYY-MM-DD"` in SQLite; always compare/format consistently.
- `renderSquare()` and `fmtPct()` live in `styles.go`.
- TUI tests that create non-obligated habits must call `service.RecordWeeklyPromotion()` first, otherwise the model starts in `modePromotion` instead of `modeNormal`.
- Maps in model structs (e.g., `refreshHabitStats`) are safe to mutate via value receivers since maps are reference types.
- Obligated habit with `ObligatedSinceDate` set: strict red/yellow logic only applies **on or after** that date; days before it are treated as non-obligated (all yellow).
