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
go test ./internal/web

# Run the server (default port 8080, override with HBT_PORT)
go run .
```

No Makefile or lint configuration is present. Use standard `gofmt` for formatting.

## Architecture

Three-layer architecture: `internal/db` → `internal/service` → `internal/web`.

**db layer**: Opens/migrates SQLite. `db.SetMaxOpenConns(1)` is mandatory — SQLite requires single-writer. Schema in `migrations.go` (3 tables: `habits`, `entries`, `weekly_promotions`).

**service layer**: Pure business logic, no UI awareness.
- `stats.go` — `ComputeDayStatuses` runs **globally across all history** (not per-week) so consecutive-skip detection works across week boundaries. Yellow vs red logic: non-obligated habits always get yellow skips; obligated habits get yellow only for isolated (single) skips, red for 2+ consecutive. Yellow days are **excluded from the success rate denominator** (rate = green / (green + red)).
- `entry.go` — `GetPendingBackfill` only fires for habits that already have at least one past entry.
- `promotion.go` — `NeedsWeeklyPromotion` checks if the current Monday is NOT yet in `weekly_promotions` AND non-obligated habits exist.

**web layer**: Go `net/http` (Go 1.22+ routing) + `html/template` + `embed` + htmx (CDN). Single binary with embedded templates/static assets.
- `server.go` — `Server` struct, route registration, `ServeHTTP`.
- `handlers.go` — all HTTP handlers calling service layer functions.
- `templates.go` — template loading via `embed.FS`, `FuncMap` helpers, template data types.
- `templates/` — HTML templates (layout, index, stats, add_habit, archive) + `partials/` (habit_row, backfill_item, global_stats).
- `static/style.css` — dark theme CSS with colored squares.
- htmx handles partial updates: entry recording swaps the habit row + global stats (oob), backfill answers swap the next question.

## Key Invariants

- Dates stored as TEXT `"YYYY-MM-DD"` in SQLite; always compare/format consistently.
- `squareClass()` and `fmtPct()` live in `templates.go` / `handlers.go`.
- Obligated habit with `ObligatedSinceDate` set: strict red/yellow logic only applies **on or after** that date; days before it are treated as non-obligated (all yellow).
- The promotion/backfill banners appear as conditional sections on the index page (not separate modes).
- Web tests use `httptest` + temp SQLite DB (same pattern as service tests).
