# Agent guide for JobPilot AI

Goal: ship features that increase activation, retention, and conversion to paid.

## Codebase tour

- `cmd/srv/main.go` — entrypoint.
- `srv/server.go` — server struct, routing, auth helpers (`currentUser`, `requireAdmin`).
- `srv/config.go` — hot-reloadable Config; persisted to `jobpilot.config.json`.
- `srv/ai.go` — OpenAI-compatible client. Falls back to demo text if no key.
- `srv/handlers_app.go` — user-facing routes (`/app/*`).
- `srv/handlers_admin.go` — admin-only routes (`/admin/*`).
- `srv/templates/` — `layout.html` is the base; each page defines `title` and `content` blocks.
- `db/migrations/NNN-*.sql` — append a new file (e.g. `002-foo.sql`) to extend schema.
- `db/queries/*.sql` — sqlc queries. Re-run `go generate ./db/...` after changes.

## Conventions

- One Go binary, no external services. Keep it that way.
- HTML templates over JSON APIs for v1; add `/api/v1/*` JSON endpoints when an integration needs them.
- All admin actions must call `s.requireAdmin(w, r)`; user actions `s.requireUser`.
- Quota check + bump live in `handlers_app.go` (`checkQuota`, `bumpQuota`).
- Don't break the systemd service contract: binary at `./jobpilot`, listens on `:8000` by default.

## How to add a feature

1. Migration if needed (`db/migrations/NNN-*.sql`).
2. Query in `db/queries/*.sql`; `go generate ./db/...`.
3. Handler in `srv/handlers_*.go`.
4. Register route in `Serve()`.
5. Template in `srv/templates/`.
6. `make build` and run.

## Done = good

- `go build ./...` clean.
- Manual smoke: `/`, `/pricing`, `/app`, `/admin` return 200 with proper auth.
- `make build && sudo systemctl restart jobpilot` deploys.
