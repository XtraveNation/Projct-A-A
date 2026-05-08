# ✈️ JobPilot AI

> **Land more interviews. Without the writing.**
> AI-powered job-hunt copilot: tailored resumes, cover letters, interview prep, and an application tracker — in a single Go binary.

JobPilot is built to be **profitable**, **easy to deploy on any VPS**, and **easy to market**. It's a thin, fast, opinionated wrapper around any OpenAI-compatible LLM (OpenAI, OpenRouter, Together, Groq, local Ollama, etc.).

---

## ✨ What it does

| For users | For the operator (you) |
|---|---|
| 🎯 Tailor a resume to any job posting | 🛠️ Built-in admin dashboard at `/admin` |
| ✍️ Generate cover letters | 📊 Live stats: users, paid users, generations, DB size |
| 🧠 Interview prep packs (likely Qs + STAR answers) | 📣 **Marketing studio** — generate X/LinkedIn/Reddit/TikTok/SEO copy on demand |
| 📊 Track applications (saved → applied → interview → offer) | 🖥️ **Hosting page** — disk, memory, systemd state, one-click restart |
| 📜 Save & re-export past generations | ⚙️ Hot-reloadable config (no restarts to change pricing, prompts, keys) |
| 💳 Free tier + Pro / Lifetime upgrades | 👥 User management, plan overrides, license-code redemption |

---

## 🐳 Run with Docker (recommended)

```bash
# Build it yourself (any VPS with Docker):
git clone https://github.com/XtraveNation/Projct-A-A.git jobpilot && cd jobpilot
cp .env.example .env  # edit with your keys
docker compose up -d
```

Or pull the prebuilt multi-arch image once GitHub Actions has built it (after enabling `workflows-templates/docker.yml`):

```bash
docker run -d --name jobpilot \
  -p 8000:8000 \
  -v jobpilot_data:/data \
  -e ADMIN_EMAILS=you@example.com \
  -e OPENAI_API_KEY=sk-... \
  --restart unless-stopped \
  ghcr.io/xtravenation/projct-a-a:latest
```

The image is **~25 MB** (distroless, static binary, no shell). Templates and static assets are embedded — `/data` is the only thing you need to persist (SQLite + config).

## 🚀 One-command install on a fresh VPS (no Docker)

```bash
curl -fsSL https://raw.githubusercontent.com/XtraveNation/Projct-A-A/main/install.sh | \
  ADMIN_EMAIL=you@example.com bash
```

That's it. The installer will:
1. Install Go (if missing).
2. Clone this repo to `~/jobpilot`.
3. Build the binary.
4. Install + start a `systemd` service (`jobpilot.service`).
5. Bind port `8000`.

Then put nginx/Caddy in front for HTTPS and you're live.

### Manual install

```bash
git clone https://github.com/XtraveNation/Projct-A-A.git jobpilot
cd jobpilot
make build
./jobpilot -listen :8000
```

### Reverse proxy (Caddy — easiest)

```caddy
jobpilot.example.com {
    reverse_proxy localhost:8000
}
```

---

## 🗺️ Architecture

```
                ┌─────────────────────────────────────────┐
                │              Internet                   │
                └─────────────────┬───────────────────────┘
                                  │ HTTPS
                          ┌───────▼────────┐
                          │ Caddy / Nginx  │  TLS, gzip, caching
                          └───────┬────────┘
                                  │  :8000
                          ┌───────▼────────┐
                          │  jobpilot bin  │  single Go binary
                          │  (this repo)   │
                          ├────────────────┤
                          │  /            │ landing
                          │  /pricing     │ pricing
                          │  /app/*       │ user app (auth)
                          │  /admin/*     │ ── admin only ──┐
                          │  /healthz     │                 │
                          └───┬─────┬─────┘                 │
                              │     │                       │
                         ┌────▼─┐ ┌─▼──────────────┐  ┌─────▼──────┐
                         │SQLite│ │ OpenAI-compat  │  │ Marketing  │
                         │ WAL  │ │ LLM API        │  │ studio     │
                         └──────┘ └────────────────┘  └────────────┘
                          ↑ persisted on the VPS disk
```

Everything ships as **one ~15 MB binary** + a tiny SQLite file. No Redis, no Postgres, no Node, no Docker required.

---

## 💰 How to make money with it

JobPilot is built around a freemium funnel:

1. **Free tier** — N tailored resumes / cover letters / interview preps per month (configurable in `/admin/config`).
2. **Pro — $19/mo** (default) — unlimited everything.
3. **Lifetime — $99** — pay once, perfect for AppSumo / Product Hunt launches.

### Wire up payments in 5 minutes

Use any "checkout link" provider — no webhook required for v1:

- **Stripe Payment Links** → paste the URL into `Pro checkout URL` in `/admin/config`.
- **Lemon Squeezy** / **Gumroad** / **Polar** → same deal.
- After purchase, send the customer a license code matching the `Redeem secret` (e.g. `jobpilot-launch-CUST123` for monthly, `jobpilot-launch-CUST123-LIFETIME` for lifetime). They paste it at `/app/upgrade`.
- Or just go to `/admin/users` and flip their plan manually.

This sidesteps webhook infra and lets you launch the same day.

### Marketing studio (`/admin/marketing`)

Generates ready-to-paste copy for:

- X / Twitter (5 posts)
- LinkedIn (1 post)
- Reddit (value-first thread)
- Cold email outreach
- TikTok / YouTube Shorts scripts (3)
- SEO blog post outline

Plus 1-click share buttons (X, LinkedIn, FB, HN, Reddit) prefilled with your tagline + URL.

---

## 🔐 Auth model

This codebase uses **proxy-injected headers** (`X-ExeDev-UserID`, `X-ExeDev-Email`) for auth. On exe.dev, this works out of the box. On any other VPS, put it behind:

- **Cloudflare Access** (free tier, 50 users) — sets `Cf-Access-Authenticated-User-Email`. Adapt `currentUser()` to use it.
- **oauth2-proxy** + GitHub/Google.
- A 5-line basic-auth middleware.

See `srv/server.go → currentUser`.

---

## 🤖 Self-improving with AI agents

The repo is structured so an agent (Shelley, Claude Code, or similar) can iterate on it autonomously:

- `AGENTS.md` documents conventions.
- `srv/handlers_app.go` and `srv/handlers_admin.go` contain all routes — easy to extend.
- `db/migrations/` is numbered (`002-*.sql` will run automatically).
- `db/queries/*.sql` + `go generate ./db/...` (sqlc) = type-safe DB code.
- `.github/workflows/agent.yml` (see below) supports an "agent on commit" loop.

To add a feature: write a new handler, register it in `Serve()`, add a template, `make build`, `sudo systemctl restart jobpilot`. Done.

---

## ⚙️ Configuration

All config is hot-reloadable via `/admin/config`. Settings persist to `jobpilot.config.json`. Most can also be set via env vars on first boot:

| Var | Purpose |
|---|---|
| `OPENAI_API_KEY` | LLM API key |
| `OPENAI_MODEL` | default `gpt-4o-mini` (cheap & good) |
| `OPENAI_BASE_URL` | swap to OpenRouter/Together/Groq/Ollama |
| `ADMIN_EMAILS` | comma-separated allow-list for `/admin` |
| `PUBLIC_URL` | used in sitemap, share buttons, marketing CTAs |
| `BRAND_NAME`, `TAGLINE` | white-label easily |
| `JOBPILOT_CONFIG` | path to JSON config file |

---

## 🧰 Operating it

```bash
sudo systemctl status jobpilot      # state
sudo journalctl -u jobpilot -f      # live logs
make build && sudo systemctl restart jobpilot   # deploy update

# Backup (do this nightly via cron):
sqlite3 db.sqlite3 ".backup '/backup/jobpilot-$(date +%F).sqlite3'"
```

The admin dashboard exposes most of these directly (status, logs tail, one-click restart).

---

## 🛣️ Roadmap / API hooks for the future

The code intentionally leaves seams for these — open `srv/server.go` to wire them up:

- `POST /api/v1/tailor` — JSON API for browser extensions.
- Stripe webhook → plan flip (replaces redeem codes).
- Browser extension that scrapes LinkedIn/Indeed → tailors in one click.
- Team plans (`teams` table, share base resume across reviewers).
- Embeddings on past job descriptions → "jobs you'll love" recs.
- Email digest cron with new matching jobs (RemoteOK / Greenhouse public APIs).

---

## 📁 Layout

```
cmd/srv/         binary entrypoint
srv/             HTTP handlers, AI client, config
srv/templates/   Go HTML templates (layout + pages)
srv/static/      CSS, JS
db/              SQLite open + migrations + sqlc-generated queries
install.sh       one-shot installer
srv.service      systemd unit (also generated by install.sh)
```

---

## License

Use it, sell it, fork it. Build something profitable. 🚀
