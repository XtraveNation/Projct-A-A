-- JobPilot AI base schema

CREATE TABLE IF NOT EXISTS migrations (
    migration_number INTEGER PRIMARY KEY,
    migration_name TEXT NOT NULL,
    executed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- One row per user (keyed on exe.dev user id, falls back to email).
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL,
    last_seen TIMESTAMP NOT NULL,
    plan TEXT NOT NULL DEFAULT 'free',          -- free | pro | lifetime
    plan_until TIMESTAMP,
    base_resume TEXT NOT NULL DEFAULT '',
    profile_notes TEXT NOT NULL DEFAULT '',
    monthly_count INTEGER NOT NULL DEFAULT 0,
    monthly_period TEXT NOT NULL DEFAULT ''    -- YYYY-MM
);

-- Generations: tailored resumes, cover letters, interview prep, etc.
CREATE TABLE IF NOT EXISTS generations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,                         -- tailor | cover | interview
    job_title TEXT NOT NULL DEFAULT '',
    company TEXT NOT NULL DEFAULT '',
    job_description TEXT NOT NULL DEFAULT '',
    output TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_generations_user ON generations(user_id, created_at DESC);

-- Application tracker.
CREATE TABLE IF NOT EXISTS applications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company TEXT NOT NULL,
    role TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'applied',     -- saved | applied | interview | offer | rejected
    url TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_applications_user ON applications(user_id, updated_at DESC);

INSERT OR IGNORE INTO migrations (migration_number, migration_name) VALUES (001, '001-base');
