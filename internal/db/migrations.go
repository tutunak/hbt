package db

const schema = `
CREATE TABLE IF NOT EXISTS habits (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    name                 TEXT    NOT NULL,
    start_date           TEXT    NOT NULL,
    is_obligated         INTEGER NOT NULL DEFAULT 0,
    obligated_since_date TEXT,
    archived             INTEGER NOT NULL DEFAULT 0,
    archive_comment      TEXT,
    created_at           TEXT    NOT NULL DEFAULT (date('now'))
);

CREATE TABLE IF NOT EXISTS entries (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    habit_id   INTEGER NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
    entry_date TEXT    NOT NULL,
    did_it     INTEGER NOT NULL,
    created_at TEXT    NOT NULL DEFAULT (datetime('now')),
    UNIQUE(habit_id, entry_date)
);

CREATE INDEX IF NOT EXISTS idx_entries_habit_date ON entries(habit_id, entry_date);
CREATE INDEX IF NOT EXISTS idx_habits_archived ON habits(archived);
`
