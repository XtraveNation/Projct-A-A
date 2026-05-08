package srv

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"srv.exe.dev/db/dbgen"
)

// --- Landing & pricing ---

func (s *Server) HandleLanding(w http.ResponseWriter, r *http.Request) {
	u := s.currentUser(r)
	s.render(w, "landing.html", map[string]any{"User": u})
}

func (s *Server) HandlePricing(w http.ResponseWriter, r *http.Request) {
	s.render(w, "pricing.html", map[string]any{
		"User":           s.currentUser(r),
		"PriceUSD":       s.cfg().ProPriceUSD,
		"ProCheckoutURL": s.cfg().ProCheckoutURL,
		"LifetimeURL":    s.cfg().LifetimeURL,
		"Quota":          s.cfg().FreeMonthlyQuota,
	})
}

// --- App home ---

func (s *Server) HandleApp(w http.ResponseWriter, r *http.Request) {
	u := s.requireUser(w, r)
	if u == nil {
		return
	}
	apps, _ := s.q().ListApplications(r.Context(), u.ID)
	gens, _ := s.q().ListGenerations(r.Context(), u.ID)
	s.render(w, "app_home.html", map[string]any{
		"User":         u,
		"Applications": apps,
		"Generations":  gens,
		"Quota":        s.cfg().FreeMonthlyQuota,
	})
}

// --- Profile (base resume) ---

func (s *Server) HandleProfile(w http.ResponseWriter, r *http.Request) {
	u := s.requireUser(w, r)
	if u == nil {
		return
	}
	s.render(w, "profile.html", map[string]any{"User": u})
}

func (s *Server) HandleProfileSave(w http.ResponseWriter, r *http.Request) {
	u := s.requireUser(w, r)
	if u == nil {
		return
	}
	_ = r.ParseForm()
	_ = s.q().SetBaseResume(r.Context(), dbgen.SetBaseResumeParams{
		BaseResume:   r.FormValue("base_resume"),
		ProfileNotes: r.FormValue("profile_notes"),
		ID:           u.ID,
	})
	http.Redirect(w, r, "/app/profile?saved=1", http.StatusSeeOther)
}

// --- Quota check ---

func (s *Server) checkQuota(u *userCtx) (allowed bool, used, quota int) {
	quota = s.cfg().FreeMonthlyQuota
	if u.Row == nil {
		return false, 0, quota
	}
	if u.Row.Plan == "pro" || u.Row.Plan == "lifetime" {
		return true, int(u.Row.MonthlyCount), 99999
	}
	period := time.Now().Format("2006-01")
	used = int(u.Row.MonthlyCount)
	if u.Row.MonthlyPeriod != period {
		used = 0
	}
	return used < quota, used, quota
}

func (s *Server) bumpQuota(u *userCtx) {
	if u.Row == nil {
		return
	}
	period := time.Now().Format("2006-01")
	count := int64(1)
	if u.Row.MonthlyPeriod == period {
		count = u.Row.MonthlyCount + 1
	}
	_ = s.q().BumpMonthlyCount(context.Background(), dbgen.BumpMonthlyCountParams{
		MonthlyCount:  count,
		MonthlyPeriod: period,
		ID:            u.ID,
	})
}

// --- Generators ---

func (s *Server) HandleTailor(w http.ResponseWriter, r *http.Request) {
	u := s.requireUser(w, r)
	if u == nil {
		return
	}
	s.render(w, "tailor.html", map[string]any{"User": u})
}

func (s *Server) HandleTailorSubmit(w http.ResponseWriter, r *http.Request) {
	s.handleGenerate(w, r, "tailor",
		"You are a senior career coach and resume writer. Rewrite the candidate's resume to closely match the target job, in clean ATS-friendly Markdown. Keep facts truthful. Highlight measurable impact. Reorder bullets so most-relevant ones appear first. Add a short Summary section tailored to this role.",
	)
}

func (s *Server) HandleCover(w http.ResponseWriter, r *http.Request) {
	u := s.requireUser(w, r)
	if u == nil {
		return
	}
	s.render(w, "cover.html", map[string]any{"User": u})
}

func (s *Server) HandleCoverSubmit(w http.ResponseWriter, r *http.Request) {
	s.handleGenerate(w, r, "cover",
		"You write concise, warm, hiring-manager-friendly cover letters. 3 short paragraphs, 250 words max. No clichés. Open with a specific hook tied to the company. Close with a clear ask.",
	)
}

func (s *Server) HandleInterview(w http.ResponseWriter, r *http.Request) {
	u := s.requireUser(w, r)
	if u == nil {
		return
	}
	s.render(w, "interview.html", map[string]any{"User": u})
}

func (s *Server) HandleInterviewSubmit(w http.ResponseWriter, r *http.Request) {
	s.handleGenerate(w, r, "interview",
		"You are an interview coach. Produce: (1) the 10 most likely interview questions for this role at this company; (2) a STAR-format draft answer for each, tailored to the candidate's resume; (3) 5 great questions the candidate should ask the interviewer. Use Markdown.",
	)
}

func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request, kind, system string) {
	u := s.requireUser(w, r)
	if u == nil {
		return
	}
	ok, used, quota := s.checkQuota(u)
	if !ok {
		http.Redirect(w, r, fmt.Sprintf("/app/upgrade?used=%d&quota=%d", used, quota), http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	jobTitle := r.FormValue("job_title")
	company := r.FormValue("company")
	jd := r.FormValue("job_description")
	extra := r.FormValue("extra")
	if jd == "" {
		http.Error(w, "job description required", 400)
		return
	}
	resume := ""
	notes := ""
	if u.Row != nil {
		resume = u.Row.BaseResume
		notes = u.Row.ProfileNotes
	}
	prompt := fmt.Sprintf("# Candidate base resume\n%s\n\n# Notes about candidate\n%s\n\n# Target role\n%s @ %s\n\n# Job description\n%s\n\n# Extra instructions\n%s",
		defaultStr(resume, "(none provided yet — ask user to fill profile)"),
		notes, jobTitle, company, jd, extra)
	out, err := s.AI.Complete(r.Context(), system, prompt)
	if err != nil {
		http.Error(w, "AI error: "+err.Error(), 502)
		return
	}
	gen, err := s.q().InsertGeneration(r.Context(), dbgen.InsertGenerationParams{
		UserID:         u.ID,
		Kind:           kind,
		JobTitle:       jobTitle,
		Company:        company,
		JobDescription: jd,
		Output:         out,
		CreatedAt:      time.Now(),
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	s.bumpQuota(u)
	http.Redirect(w, r, fmt.Sprintf("/app/history/%d", gen.ID), http.StatusSeeOther)
}

func defaultStr(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

// --- Applications ---

func (s *Server) HandleApplications(w http.ResponseWriter, r *http.Request) {
	u := s.requireUser(w, r)
	if u == nil {
		return
	}
	apps, _ := s.q().ListApplications(r.Context(), u.ID)
	s.render(w, "applications.html", map[string]any{"User": u, "Applications": apps})
}

func (s *Server) HandleApplicationsCreate(w http.ResponseWriter, r *http.Request) {
	u := s.requireUser(w, r)
	if u == nil {
		return
	}
	_ = r.ParseForm()
	now := time.Now()
	_, _ = s.q().InsertApplication(r.Context(), dbgen.InsertApplicationParams{
		UserID:    u.ID,
		Company:   r.FormValue("company"),
		Role:      r.FormValue("role"),
		Status:    defaultStr(r.FormValue("status"), "saved"),
		Url:       r.FormValue("url"),
		Notes:     r.FormValue("notes"),
		CreatedAt: now,
		UpdatedAt: now,
	})
	http.Redirect(w, r, "/app/applications", http.StatusSeeOther)
}

func (s *Server) HandleApplicationsUpdate(w http.ResponseWriter, r *http.Request) {
	u := s.requireUser(w, r)
	if u == nil {
		return
	}
	_ = r.ParseForm()
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	_ = s.q().UpdateApplicationStatus(r.Context(), dbgen.UpdateApplicationStatusParams{
		Status:    r.FormValue("status"),
		Notes:     r.FormValue("notes"),
		UpdatedAt: time.Now(),
		ID:        id,
		UserID:    u.ID,
	})
	http.Redirect(w, r, "/app/applications", http.StatusSeeOther)
}

func (s *Server) HandleApplicationsDelete(w http.ResponseWriter, r *http.Request) {
	u := s.requireUser(w, r)
	if u == nil {
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	_ = s.q().DeleteApplication(r.Context(), dbgen.DeleteApplicationParams{ID: id, UserID: u.ID})
	http.Redirect(w, r, "/app/applications", http.StatusSeeOther)
}

// --- History ---

func (s *Server) HandleHistory(w http.ResponseWriter, r *http.Request) {
	u := s.requireUser(w, r)
	if u == nil {
		return
	}
	gens, _ := s.q().ListGenerations(r.Context(), u.ID)
	s.render(w, "history.html", map[string]any{"User": u, "Generations": gens})
}

func (s *Server) HandleHistoryItem(w http.ResponseWriter, r *http.Request) {
	u := s.requireUser(w, r)
	if u == nil {
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	gen, err := s.q().GetGeneration(r.Context(), dbgen.GetGenerationParams{ID: id, UserID: u.ID})
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	s.render(w, "history_item.html", map[string]any{"User": u, "Gen": gen})
}

// --- Upgrade ---

func (s *Server) HandleUpgrade(w http.ResponseWriter, r *http.Request) {
	u := s.requireUser(w, r)
	if u == nil {
		return
	}
	s.render(w, "upgrade.html", map[string]any{
		"User":           u,
		"PriceUSD":       s.cfg().ProPriceUSD,
		"ProCheckoutURL": s.cfg().ProCheckoutURL,
		"LifetimeURL":    s.cfg().LifetimeURL,
	})
}

func (s *Server) HandleRedeem(w http.ResponseWriter, r *http.Request) {
	u := s.requireUser(w, r)
	if u == nil {
		return
	}
	_ = r.ParseForm()
	code := strings.TrimSpace(r.FormValue("code"))
	secret := s.cfg().RedeemSecret
	if code == "" || secret == "" || !strings.HasPrefix(code, secret+"-") {
		http.Redirect(w, r, "/app/upgrade?err=invalid", http.StatusSeeOther)
		return
	}
	plan := "pro"
	var until sql.NullTime
	if strings.HasSuffix(code, "-LIFETIME") {
		plan = "lifetime"
	} else {
		until = sql.NullTime{Time: time.Now().AddDate(0, 1, 0), Valid: true}
	}
	_ = s.q().SetPlan(r.Context(), dbgen.SetPlanParams{
		Plan:      plan,
		PlanUntil: nullTimePtr(until),
		ID:        u.ID,
	})
	http.Redirect(w, r, "/app?upgraded=1", http.StatusSeeOther)
}

func nullTimePtr(n sql.NullTime) *time.Time {
	if !n.Valid {
		return nil
	}
	t := n.Time
	return &t
}

