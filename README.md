# hbt — Habit Tracker

A web-based habit tracker with color-coded calendar squares, backfill support, weekly obligation reviews, and per-habit statistics charts.

## Install

```bash
go install .
```

Or build locally:

```bash
go build -o hbt .
```

Single binary — templates and static assets are embedded. Data is stored at `~/.local/share/hbt/hbt.db` (XDG).

## Usage

Run `hbt` to start the web server on port `8080` (override with `HBT_PORT` env var). Open `http://localhost:8080` in your browser.

The main screen shows each habit with 3 rolling weeks of colored squares:

```
  ✅ Morning run     ■ ■ ■  ■ □ ■ ■  ■ ■ ■   20/21 (95%)
  ⬜ Read            ■ ■ ▪  ■ ■ ■ ■  ■ ■ ■   19/20 (95%)
```

**Square colors:**
- Green `■` — done
- Yellow `▪` — single isolated skip (not counted against success rate)
- Red `□` — two or more consecutive skips
- Gray — no entry recorded

## Features

**Backfill** — if you have unrecorded days in the past, a banner appears on the main page asking you to fill them in one by one.

**Weekly promotion** — each Monday, a banner prompts you to promote one non-obligated habit to obligated. Selecting a habit takes you to a confirmation page before applying the change. You can also skip the week.

**Statistics** — per-habit breakdown by week with colored squares, success rates, and bar+line charts showing weekly trends (powered by Chart.js). A global average for obligated habits is shown at the bottom.

**Archived habits** — archived habits are hidden from the main list. Visit the "Archived" page from the nav bar to view them with their full stats history and archive comments.

## Habits: Obligated vs Non-Obligated

- **Obligated** (`✅`) — two or more consecutive skips turn red. Missing days hurt your score.
- **Non-obligated** (`⬜`) — all skips stay yellow; never red. Good for habits you're still building.

When you promote a non-obligated habit, today becomes its `obligated_since_date` — days before that date remain yellow.

## Requirements

- Go 1.24+
- No CGO required (SQLite via `modernc.org/sqlite`)
