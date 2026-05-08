package srv

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"srv.exe.dev/db/dbgen"
)

// --- Admin home ---

type adminStats struct {
	Users            int64
	ProUsers         int64
	GenerationsTotal int64
	Generations24h   int64
	Applications     int64
	DBSizeMB         float64
	Uptime           string
	Goroutines       int
	Hostname         string
	PublicURL        string
}

func (s *Server) HandleAdmin(w http.ResponseWriter, r *http.Request) {
	u := s.requireAdmin(w, r)
	if u == nil {
		return
	}
	st := adminStats{
		Hostname:   s.Hostname,
		PublicURL:  s.cfg().PublicURL,
		Uptime:     time.Since(s.startedAt).Round(time.Second).String(),
		Goroutines: runtime.NumGoroutine(),
	}
	_ = s.DB.QueryRow("SELECT count(*) FROM users").Scan(&st.Users)
	_ = s.DB.QueryRow("SELECT count(*) FROM users WHERE plan IN ('pro','lifetime')").Scan(&st.ProUsers)
	_ = s.DB.QueryRow("SELECT count(*) FROM generations").Scan(&st.GenerationsTotal)
	_ = s.DB.QueryRow("SELECT count(*) FROM generations WHERE created_at > ?", time.Now().Add(-24*time.Hour)).Scan(&st.Generations24h)
	_ = s.DB.QueryRow("SELECT count(*) FROM applications").Scan(&st.Applications)
	if fi, err := os.Stat("db.sqlite3"); err == nil {
		st.DBSizeMB = float64(fi.Size()) / 1024 / 1024
	}
	s.render(w, "admin_home.html", map[string]any{"User": u, "Stats": st})
}

// --- Config editor ---

func (s *Server) HandleAdminConfig(w http.ResponseWriter, r *http.Request) {
	u := s.requireAdmin(w, r)
	if u == nil {
		return
	}
	s.render(w, "admin_config.html", map[string]any{"User": u, "Cfg": s.cfg()})
}

func (s *Server) HandleAdminConfigSave(w http.ResponseWriter, r *http.Request) {
	u := s.requireAdmin(w, r)
	if u == nil {
		return
	}
	_ = r.ParseForm()
	quota, _ := strconv.Atoi(r.FormValue("free_monthly_quota"))
	price, _ := strconv.Atoi(r.FormValue("pro_price_usd"))
	if quota <= 0 {
		quota = 3
	}
	if price <= 0 {
		price = 19
	}
	newCfg := &Config{
		BrandName:        strings.TrimSpace(r.FormValue("brand_name")),
		Tagline:          strings.TrimSpace(r.FormValue("tagline")),
		PublicURL:        strings.TrimSpace(r.FormValue("public_url")),
		AdminEmails:      splitCSV(r.FormValue("admin_emails")),
		OpenAIKey:        strings.TrimSpace(r.FormValue("openai_key")),
		OpenAIModel:      strings.TrimSpace(r.FormValue("openai_model")),
		OpenAIBaseURL:    strings.TrimSpace(r.FormValue("openai_base_url")),
		FreeMonthlyQuota: quota,
		ProPriceUSD:      price,
		ProCheckoutURL:   strings.TrimSpace(r.FormValue("pro_checkout_url")),
		LifetimeURL:      strings.TrimSpace(r.FormValue("lifetime_url")),
		RedeemSecret:     strings.TrimSpace(r.FormValue("redeem_secret")),
		AnalyticsTag:     r.FormValue("analytics_tag"),
		MarketingPrompt:  r.FormValue("marketing_prompt"),
		HostingNotes:     r.FormValue("hosting_notes"),
	}
	if newCfg.BrandName == "" {
		newCfg.BrandName = "JobPilot AI"
	}
	if newCfg.OpenAIModel == "" {
		newCfg.OpenAIModel = "gpt-4o-mini"
	}
	if newCfg.OpenAIBaseURL == "" {
		newCfg.OpenAIBaseURL = "https://api.openai.com/v1"
	}
	if err := SaveConfig(newCfg); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	s.setCfg(newCfg)
	http.Redirect(w, r, "/admin/config?saved=1", http.StatusSeeOther)
}

// --- Users ---

type adminUser struct {
	ID       string
	Email    string
	Plan     string
	Created  time.Time
	LastSeen time.Time
	Gens     int64
}

func (s *Server) HandleAdminUsers(w http.ResponseWriter, r *http.Request) {
	u := s.requireAdmin(w, r)
	if u == nil {
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `
		SELECT u.id, u.email, u.plan, u.created_at, u.last_seen,
		       (SELECT count(*) FROM generations g WHERE g.user_id = u.id) AS gens
		FROM users u ORDER BY u.last_seen DESC LIMIT 500`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	var users []adminUser
	for rows.Next() {
		var a adminUser
		if err := rows.Scan(&a.ID, &a.Email, &a.Plan, &a.Created, &a.LastSeen, &a.Gens); err == nil {
			users = append(users, a)
		}
	}
	s.render(w, "admin_users.html", map[string]any{"User": u, "Users": users})
}

func (s *Server) HandleAdminSetPlan(w http.ResponseWriter, r *http.Request) {
	u := s.requireAdmin(w, r)
	if u == nil {
		return
	}
	_ = r.ParseForm()
	id := r.FormValue("user_id")
	plan := r.FormValue("plan")
	var until *time.Time
	if plan == "pro" {
		t := time.Now().AddDate(0, 1, 0)
		until = &t
	}
	_ = s.q().SetPlan(r.Context(), dbgen.SetPlanParams{Plan: plan, PlanUntil: until, ID: id})
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

// --- Marketing studio ---

func (s *Server) HandleAdminMarketing(w http.ResponseWriter, r *http.Request) {
	u := s.requireAdmin(w, r)
	if u == nil {
		return
	}
	s.render(w, "admin_marketing.html", map[string]any{"User": u, "Cfg": s.cfg()})
}

func (s *Server) HandleAdminMarketingGenerate(w http.ResponseWriter, r *http.Request) {
	u := s.requireAdmin(w, r)
	if u == nil {
		return
	}
	_ = r.ParseForm()
	channel := r.FormValue("channel")
	topic := r.FormValue("topic")
	cta := defaultStr(r.FormValue("cta"), s.cfg().PublicURL)
	system := s.cfg().MarketingPrompt + "\nProduct: " + s.cfg().BrandName + " — " + s.cfg().Tagline

	var instr string
	switch channel {
	case "x":
		instr = "Write 5 distinct X/Twitter posts (<=270 chars each). Numbered. Each ends with the CTA URL on its own line."
	case "linkedin":
		instr = "Write 1 LinkedIn post (~150 words), conversational, line-broken, ending with the CTA URL."
	case "reddit":
		instr = "Write 1 Reddit post for r/jobs that gives genuine value first, soft mention of the product at the end with the CTA URL. Title + body."
	case "email":
		instr = "Write a cold outreach email to a job-seeker community manager. Subject line + body. End with CTA URL."
	case "tiktok":
		instr = "Write 3 TikTok/Shorts scripts (15–30s each). Hook first. Bullet beats. End with verbal CTA."
	case "seo":
		instr = "Write a 600-word SEO blog post outline with H2/H3 headers and meta description, targeting the topic."
	default:
		instr = "Generate 3 short marketing snippets."
	}
	prompt := fmt.Sprintf("%s\n\nTopic / angle: %s\nCTA URL: %s", instr, topic, cta)
	out, err := s.AI.Complete(r.Context(), system, prompt)
	if err != nil {
		http.Error(w, "AI error: "+err.Error(), 502)
		return
	}
	s.render(w, "admin_marketing.html", map[string]any{
		"User": u, "Cfg": s.cfg(),
		"Output": out, "Channel": channel, "Topic": topic, "CTA": cta,
	})
}

// --- Hosting ---

type hostingInfo struct {
	Hostname     string
	GoVersion    string
	NumCPU       int
	DiskFree     string
	MemInfo      string
	SystemdState string
	Uptime       string
	ListenAddr   string
	ConfigPath   string
	DBPath       string
	Notes        string
}

func (s *Server) HandleAdminHosting(w http.ResponseWriter, r *http.Request) {
	u := s.requireAdmin(w, r)
	if u == nil {
		return
	}
	info := hostingInfo{
		Hostname:     s.Hostname,
		GoVersion:    runtime.Version(),
		NumCPU:       runtime.NumCPU(),
		DiskFree:     runCmd("df", "-h", "."),
		MemInfo:      runCmd("sh", "-c", "free -h 2>/dev/null || vm_stat"),
		SystemdState: runCmd("systemctl", "is-active", "srv"),
		Uptime:       time.Since(s.startedAt).Round(time.Second).String(),
		ConfigPath:   configPath(),
		DBPath:       "db.sqlite3",
		Notes:        s.cfg().HostingNotes,
	}
	s.render(w, "admin_hosting.html", map[string]any{"User": u, "Info": info})
}

func (s *Server) HandleAdminRestart(w http.ResponseWriter, r *http.Request) {
	u := s.requireAdmin(w, r)
	if u == nil {
		return
	}
	go func() {
		time.Sleep(500 * time.Millisecond)
		_ = exec.Command("sudo", "-n", "systemctl", "restart", "srv").Run()
	}()
	http.Redirect(w, r, "/admin/hosting?restart=requested", http.StatusSeeOther)
}

func (s *Server) HandleAdminLogs(w http.ResponseWriter, r *http.Request) {
	u := s.requireAdmin(w, r)
	if u == nil {
		return
	}
	logs := runCmd("sh", "-c", "journalctl -u srv -n 200 --no-pager 2>&1 || tail -n 200 /var/log/syslog 2>&1 || echo no logs available")
	s.render(w, "admin_logs.html", map[string]any{"User": u, "Logs": logs})
}

func runCmd(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("%s\n[err: %v]", string(out), err)
	}
	return string(out)
}

// silence unused import if sql not needed elsewhere in this file
var _ = sql.ErrNoRows
