# hbt — Habit Tracker

A terminal habit tracker with a color-coded calendar view, backfill support, and weekly obligation reviews.

## Install

```bash
go install .
```

Or build locally:

```bash
go build -o hbt .
```

Data is stored at `~/.local/share/hbt/hbt.db` (XDG).

## Usage

Run `hbt` to open the TUI. The main screen shows each habit with 3 rolling weeks of colored squares:

```
  ✅ Morning run     ■ ■ ■  ■ □ ■ ■  ■ ■ ■   20/21 (95%)
  ⬜ Read            ■ ■ ▪  ■ ■ ■ ■  ■ ■ ■   19/20 (95%)
```

**Square colors:**
- Green `■` — done
- Yellow `▪` — single isolated skip (not counted against success rate)
- Red `□` — two or more consecutive skips
- Gray — no entry recorded

**Normal mode keys:**

| Key | Action |
|-----|--------|
| `↑` / `k`, `↓` / `j` | Navigate habits |
| `y` | Record today as done |
| `n` | Record today as skipped |
| `a` | Add a new habit |
| `s` | Open statistics view |
| `r` | Archive selected habit |
| `q` | Quit |

## Screens

**Backfill** — on startup, if you have unrecorded days in the past, hbt asks you to fill them in. Use `y` / `n` to answer, `s` to skip a question.

**Weekly promotion** — each Monday, hbt prompts you to promote one non-obligated habit (`⬜`) to obligated (`✅`). Use `↑`/`↓` to select, `enter` to promote, `s` to skip the week.

**Statistics** — scrollable per-habit breakdown by week with a global average for obligated habits only.

## Habits: Obligated vs Non-Obligated

- **Obligated** (`✅`) — two or more consecutive skips turn red. Missing days hurt your score.
- **Non-obligated** (`⬜`) — all skips stay yellow; never red. Good for habits you're still building.

When you promote a non-obligated habit, today becomes its `obligated_since_date` — days before that date remain yellow.

## Requirements

- Go 1.24+
- No CGO required (SQLite via `modernc.org/sqlite`)
