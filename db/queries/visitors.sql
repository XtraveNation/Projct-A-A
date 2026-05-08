-- name: UpsertUser :exec
INSERT INTO users (id, email, created_at, last_seen)
VALUES (?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  email = excluded.email,
  last_seen = excluded.last_seen;

-- name: GetUser :one
SELECT * FROM users WHERE id = ?;

-- name: SetBaseResume :exec
UPDATE users SET base_resume = ?, profile_notes = ? WHERE id = ?;

-- name: SetPlan :exec
UPDATE users SET plan = ?, plan_until = ? WHERE id = ?;

-- name: BumpMonthlyCount :exec
UPDATE users SET monthly_count = ?, monthly_period = ? WHERE id = ?;

-- name: InsertGeneration :one
INSERT INTO generations (user_id, kind, job_title, company, job_description, output, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListGenerations :many
SELECT * FROM generations WHERE user_id = ? ORDER BY created_at DESC LIMIT 50;

-- name: GetGeneration :one
SELECT * FROM generations WHERE id = ? AND user_id = ?;

-- name: InsertApplication :one
INSERT INTO applications (user_id, company, role, status, url, notes, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListApplications :many
SELECT * FROM applications WHERE user_id = ? ORDER BY updated_at DESC;

-- name: UpdateApplicationStatus :exec
UPDATE applications SET status = ?, notes = ?, updated_at = ? WHERE id = ? AND user_id = ?;

-- name: DeleteApplication :exec
DELETE FROM applications WHERE id = ? AND user_id = ?;
