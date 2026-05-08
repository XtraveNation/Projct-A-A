package srv

import (
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"srv.exe.dev/db"
	"srv.exe.dev/db/dbgen"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

type Server struct {
	DB        *sql.DB
	Hostname  string
	AI        *AIClient
	Config    *Config
	cfgMu     sync.RWMutex
	startedAt time.Time
}

func New(dbPath, hostname string) (*Server, error) {
	s := &Server{Hostname: hostname, startedAt: time.Now()}
	if err := s.setUpDatabase(dbPath); err != nil {
		return nil, err
	}
	s.Config = LoadConfig()
	s.AI = NewAIClient(s.Config)
	return s, nil
}

func (s *Server) setUpDatabase(dbPath string) error {
	wdb, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	s.DB = wdb
	return db.RunMigrations(wdb)
}

func (s *Server) q() *dbgen.Queries { return dbgen.New(s.DB) }

func (s *Server) cfg() *Config {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.Config
}

func (s *Server) setCfg(c *Config) {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()
	s.Config = c
	s.AI = NewAIClient(c)
}

// --- auth helpers ---

type userCtx struct {
	ID       string
	Email    string
	LoggedIn bool
	Admin    bool
	Row      *dbgen.User
}

func (s *Server) currentUser(r *http.Request) *userCtx {
	u := &userCtx{
		ID:    strings.TrimSpace(r.Header.Get("X-ExeDev-UserID")),
		Email: strings.TrimSpace(r.Header.Get("X-ExeDev-Email")),
	}
	u.LoggedIn = u.ID != ""
	if u.LoggedIn {
		now := time.Now()
		_ = s.q().UpsertUser(r.Context(), dbgen.UpsertUserParams{
			ID:        u.ID,
			Email:     u.Email,
			CreatedAt: now,
			LastSeen:  now,
		})
		if row, err := s.q().GetUser(r.Context(), u.ID); err == nil {
			u.Row = &row
		}
		u.Admin = s.isAdmin(u.Email)
	}
	return u
}

func (s *Server) isAdmin(email string) bool {
	if email == "" {
		return false
	}
	email = strings.ToLower(email)
	for _, a := range s.cfg().AdminEmails {
		if strings.ToLower(strings.TrimSpace(a)) == email {
			return true
		}
	}
	return false
}

func loginURLForRequest(r *http.Request) string {
	v := url.Values{}
	v.Set("redirect", r.URL.RequestURI())
	return "/__exe.dev/login?" + v.Encode()
}

func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) *userCtx {
	u := s.currentUser(r)
	if !u.LoggedIn {
		http.Redirect(w, r, loginURLForRequest(r), http.StatusSeeOther)
		return nil
	}
	return u
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) *userCtx {
	u := s.requireUser(w, r)
	if u == nil {
		return nil
	}
	if !u.Admin {
		http.Error(w, "forbidden — admin only", http.StatusForbidden)
		return nil
	}
	return u
}

// --- templates ---

func (s *Server) render(w http.ResponseWriter, name string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	data["Hostname"] = s.Hostname
	data["Brand"] = s.cfg().BrandName
	data["Tagline"] = s.cfg().Tagline
	funcs := template.FuncMap{
		"truncate": func(n int, s string) string {
			if len(s) <= n {
				return s
			}
			return s[:n] + "…"
		},
		"fmtTime": func(t time.Time) string { return t.Format("Jan 2, 2006 15:04") },
		"hasPrefix": strings.HasPrefix,
	}
	tmpl, err := template.New("layout.html").Funcs(funcs).ParseFS(templatesFS, "templates/layout.html", "templates/"+name)
	if err != nil {
		slog.Error("parse template", "name", name, "error", err)
		http.Error(w, "template error", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		slog.Warn("exec template", "name", name, "error", err)
	}
}

// --- Serve ---

func (s *Server) Serve(addr string) error {
	mux := http.NewServeMux()

	// Public
	mux.HandleFunc("GET /{$}", s.HandleLanding)
	mux.HandleFunc("GET /pricing", s.HandlePricing)
	mux.HandleFunc("GET /privacy", s.HandleStatic("privacy.html"))
	mux.HandleFunc("GET /terms", s.HandleStatic("terms.html"))
	mux.HandleFunc("GET /robots.txt", s.HandleRobots)
	mux.HandleFunc("GET /sitemap.xml", s.HandleSitemap)

	// App (auth required)
	mux.HandleFunc("GET /app", s.HandleApp)
	mux.HandleFunc("GET /app/profile", s.HandleProfile)
	mux.HandleFunc("POST /app/profile", s.HandleProfileSave)
	mux.HandleFunc("GET /app/tailor", s.HandleTailor)
	mux.HandleFunc("POST /app/tailor", s.HandleTailorSubmit)
	mux.HandleFunc("GET /app/cover", s.HandleCover)
	mux.HandleFunc("POST /app/cover", s.HandleCoverSubmit)
	mux.HandleFunc("GET /app/interview", s.HandleInterview)
	mux.HandleFunc("POST /app/interview", s.HandleInterviewSubmit)
	mux.HandleFunc("GET /app/applications", s.HandleApplications)
	mux.HandleFunc("POST /app/applications", s.HandleApplicationsCreate)
	mux.HandleFunc("POST /app/applications/{id}", s.HandleApplicationsUpdate)
	mux.HandleFunc("POST /app/applications/{id}/delete", s.HandleApplicationsDelete)
	mux.HandleFunc("GET /app/history", s.HandleHistory)
	mux.HandleFunc("GET /app/history/{id}", s.HandleHistoryItem)
	mux.HandleFunc("GET /app/upgrade", s.HandleUpgrade)
	mux.HandleFunc("POST /app/redeem", s.HandleRedeem)

	// Admin
	mux.HandleFunc("GET /admin", s.HandleAdmin)
	mux.HandleFunc("GET /admin/config", s.HandleAdminConfig)
	mux.HandleFunc("POST /admin/config", s.HandleAdminConfigSave)
	mux.HandleFunc("GET /admin/users", s.HandleAdminUsers)
	mux.HandleFunc("POST /admin/users/plan", s.HandleAdminSetPlan)
	mux.HandleFunc("GET /admin/marketing", s.HandleAdminMarketing)
	mux.HandleFunc("POST /admin/marketing/generate", s.HandleAdminMarketingGenerate)
	mux.HandleFunc("GET /admin/hosting", s.HandleAdminHosting)
	mux.HandleFunc("POST /admin/hosting/restart", s.HandleAdminRestart)
	mux.HandleFunc("GET /admin/logs", s.HandleAdminLogs)

	// Health/API
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") })

	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	slog.Info("jobpilot starting", "addr", addr, "hostname", s.Hostname)
	return http.ListenAndServe(addr, withRecover(mux))
}

func withRecover(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic", "err", rec, "path", r.URL.Path)
				http.Error(w, "internal error", 500)
			}
		}()
		h.ServeHTTP(w, r)
	})
}

func (s *Server) HandleStatic(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.render(w, name, map[string]any{"User": s.currentUser(r)})
	}
}

func (s *Server) HandleRobots(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "User-agent: *\nAllow: /\nSitemap: %s/sitemap.xml\n", s.cfg().PublicURL)
}

func (s *Server) HandleSitemap(w http.ResponseWriter, r *http.Request) {
	base := s.cfg().PublicURL
	w.Header().Set("Content-Type", "application/xml")
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
<url><loc>%s/</loc></url>
<url><loc>%s/pricing</loc></url>
<url><loc>%s/privacy</loc></url>
<url><loc>%s/terms</loc></url>
</urlset>`, base, base, base, base)
}

// envOr returns env value or default.
func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
