package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"myfit/backend/internal/api"
	"myfit/backend/internal/domain"
	"myfit/backend/internal/store"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"time"
)

type client struct {
	handler http.Handler
	cookie  *http.Cookie
	csrf    string
}

func (c *client) request(method, path string, body any, status int, out any) error {
	var b bytes.Buffer
	if body != nil {
		if e := json.NewEncoder(&b).Encode(body); e != nil {
			return errors.New("cannot encode validation request")
		}
	}
	r := httptest.NewRequest(method, path, &b)
	r.Header.Set("Origin", origin)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", c.csrf)
	if c.cookie != nil {
		r.AddCookie(c.cookie)
	}
	w := httptest.NewRecorder()
	c.handler.ServeHTTP(w, r)
	if w.Code != status {
		return fmt.Errorf("%s %s returned %d, expected %d (body and credentials suppressed)", method, path, w.Code, status)
	}
	for _, cookie := range w.Result().Cookies() {
		c.cookie = cookie
	}
	if out != nil {
		if e := json.Unmarshal(w.Body.Bytes(), out); e != nil {
			return errors.New("cannot decode validation response (body suppressed)")
		}
	}
	return nil
}
func random() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic("cannot generate validation identifier")
	}
	return hex.EncodeToString(b)
}
func closeEnough(a, b float64) bool { return math.Abs(a-b) < .000001 }
func smoke(ctx context.Context, t target, dir, reports string) error {
	cfg, e := runtimeConfig(t, dir)
	if e != nil {
		return e
	}
	db, e := store.OpenConfigured(ctx, cfg)
	if e != nil {
		return e
	}
	defer db.Pool.Close()
	// Normal session creation sweeps expired sessions. Refuse to run if that would
	// remove any existing records; no test cleanup/down/delete endpoints are used.
	var expired int
	if e = db.Pool.QueryRow(ctx, `SELECT count(*) FROM public.sessions WHERE expires_at<now()+interval '15 minutes'`).Scan(&expired); e != nil {
		return e
	}
	if expired > 0 {
		return errors.New("expired or soon-expiring sessions exist; normal registration could remove them. No smoke records created; request confirmation before that cleanup")
	}
	runID := time.Now().UTC().Format("20060102T150405Z") + "-" + random()[:8]
	handler := api.New(db, origin, true)
	a := client{handler: handler}
	b := client{handler: handler}
	emailA := "myfit-neon-validation-" + runID + "-a@example.test"
	emailB := "myfit-neon-validation-" + runID + "-b@example.test"
	password := random() + "-test"
	ids := []int64{}
	// Persist an audit manifest before the first API write. Passwords/session tokens
	// never enter reports. Fixtures are retained until explicit cleanup approval.
	manifest := map[string]any{"run_id": runID, "endpoint": t.Endpoint, "database": t.Database, "test_emails": []string{emailA, emailB}, "user_ids": ids, "started_at": time.Now().UTC(), "cleanup": "not performed; explicit confirmation required", "status": "started"}
	reportPath := filepath.Join(reports, "neon-smoke-"+runID+".json")
	if e = save(reportPath, manifest); e != nil {
		return e
	}
	register := func(c *client, email string) error {
		var v struct {
			User domain.User `json:"user"`
			CSRF string      `json:"csrf"`
		}
		if e := c.request("POST", "/api/register", map[string]string{"name": "MYFIT NEON STAGING VALIDATION " + runID, "email": email, "password": password, "timezone": "Asia/Kolkata"}, 200, &v); e != nil {
			return e
		}
		c.csrf = v.CSRF
		ids = append(ids, v.User.ID)
		manifest["user_ids"] = ids
		if e = save(reportPath, manifest); e != nil {
			return e
		}
		if v.User.OnboardingCompleted || v.User.Goals.Calories != 0 {
			return errors.New("new account incorrectly has generic active goals")
		}
		if c.cookie == nil || c.cookie.Name != "__Host-myfit_session" || !c.cookie.Secure || !c.cookie.HttpOnly || c.cookie.Domain != "" {
			return errors.New("secure session cookie validation failed")
		}
		return nil
	}
	if e = register(&a, emailA); e != nil {
		return e
	}
	if e = register(&b, emailB); e != nil {
		return e
	}
	profile := domain.OnboardingProfile{Age: 24, Sex: "male", Height: 167.64, HeightUnit: "cm", Weight: 54.5, WeightUnit: "kg", Units: "metric", Timezone: "Asia/Kolkata", Lifestyle: "sedentary", Sessions: 3.5, Minutes: 60, Intensity: "moderate", Training: "resistance", Objective: "muscle", RatePercent: .25, Diet: "vegetarian", HealthContext: "healthy"}
	var state store.OnboardingState
	if e = a.request("PUT", "/api/onboarding/profile", map[string]any{"version": 0, "profile": profile}, 200, &state); e != nil {
		return e
	}
	if state.Recommendation == nil || !state.Recommendation.Result.Supported || !closeEnough(state.Recommendation.Result.REE, 1477.75) {
		return errors.New("backend onboarding calculation mismatch")
	}
	if e = a.request("POST", "/api/onboarding/goals", map[string]any{"version": state.User.OnboardingVersion, "recommendation_id": state.Recommendation.ID, "mode": "recommendation"}, 200, nil); e != nil {
		return e
	}
	var other store.OnboardingState
	if e = b.request("GET", "/api/onboarding", nil, 200, &other); e != nil {
		return e
	}
	if other.User.Profile != nil || other.Recommendation != nil {
		return errors.New("onboarding data leaked across users")
	}
	var foods []domain.Food
	if e = a.request("GET", "/api/foods?q=banana", nil, 200, &foods); e != nil {
		return e
	}
	if len(foods) != 1 {
		return errors.New("banana catalogue search mismatch")
	}
	f := foods[0]
	var serving int64
	for _, s := range f.Servings {
		if s.Grams == 118 {
			serving = s.ID
		}
	}
	if serving == 0 {
		return errors.New("medium banana serving missing")
	}
	day := time.Now().UTC().Format("2006-01-02")
	var entry domain.Log
	if e = a.request("POST", "/api/logs", map[string]any{"food_id": f.ID, "serving_id": serving, "quantity": 2, "day": day, "meal": "breakfast"}, 200, &entry); e != nil {
		return e
	}
	want := domain.Nutrition{Calories: 210.04, Protein: 2.5724, Carbs: 53.9024, Fat: .7788, Fibre: 6.136}
	actual := entry.Nutrition
	if !closeEnough(actual.Calories, want.Calories) || !closeEnough(actual.Protein, want.Protein) || !closeEnough(actual.Carbs, want.Carbs) || !closeEnough(actual.Fat, want.Fat) || !closeEnough(actual.Fibre, want.Fibre) {
		return errors.New("server nutrition calculation mismatch")
	}
	edit := map[string]any{"quantity": 1, "day": day, "meal": "breakfast", "version": entry.Version}
	logPath := fmt.Sprintf("/api/logs/%d", entry.ID)
	if e = b.request("PUT", logPath, edit, 404, nil); e != nil {
		return e
	}
	if e = a.request("PUT", logPath, edit, 200, &entry); e != nil {
		return e
	}
	if !closeEnough(entry.Nutrition.Calories, 105.02) {
		return errors.New("quantity correction did not recalculate")
	}
	if e = a.request("PUT", logPath, edit, 409, nil); e != nil {
		return e
	}
	var diary struct {
		Logs   []domain.Log     `json:"logs"`
		Totals domain.Nutrition `json:"totals"`
	}
	if e = b.request("GET", "/api/diary?date="+day, nil, 200, &diary); e != nil {
		return e
	}
	if len(diary.Logs) != 0 || diary.Totals.Calories != 0 {
		return errors.New("food diary data leaked across users")
	}
	var private domain.Food
	if e = a.request("POST", "/api/foods", map[string]any{"name": "MYFIT NEON STAGING PRIVATE FOOD " + runID, "calories": 100, "protein": 10, "carbs": 10, "fat": 2, "fibre": 1, "source": "Staging validation fixture, not a real product", "servings": []map[string]any{{"name": "Validation portion", "grams": 50}}}, 200, &private); e != nil {
		return e
	}
	if e = b.request("GET", "/api/foods?filter=private", nil, 200, &foods); e != nil {
		return e
	}
	if len(foods) != 0 {
		return errors.New("private food catalogue data leaked")
	}
	if e = b.request("POST", "/api/logs", map[string]any{"food_id": private.ID, "serving_id": private.Servings[0].ID, "quantity": 1, "day": day, "meal": "lunch"}, 404, nil); e != nil {
		return e
	}
	if e = a.request("POST", "/api/weights", map[string]any{"day": day, "kg": 54.5}, 200, nil); e != nil {
		return e
	}
	var weights []domain.Weight
	if e = b.request("GET", "/api/weights", nil, 200, &weights); e != nil {
		return e
	}
	if len(weights) != 0 {
		return errors.New("weight data leaked across users")
	}
	// A fresh client signs in without revoking any prior session; no logout/delete
	// operation or fixture cleanup is invoked. Verify persisted records via that session.
	again := client{handler: handler}
	var login struct {
		CSRF string      `json:"csrf"`
		User domain.User `json:"user"`
	}
	if e = again.request("POST", "/api/login", map[string]string{"email": emailA, "password": password}, 200, &login); e != nil {
		return e
	}
	again.csrf = login.CSRF
	if !login.User.OnboardingCompleted {
		return errors.New("onboarding did not persist")
	}
	if e = again.request("GET", "/api/diary?date="+day, nil, 200, &diary); e != nil {
		return e
	}
	if len(diary.Logs) != 1 || !closeEnough(diary.Totals.Calories, 105.02) {
		return errors.New("diary persistence mismatch after login")
	}
	if e = again.request("GET", "/api/weights", nil, 200, &weights); e != nil {
		return e
	}
	if len(weights) != 1 || weights[0].KG != 54.5 {
		return errors.New("weight persistence mismatch")
	}
	wrong := a.csrf
	a.csrf = "invalid"
	if e = a.request("POST", "/api/weights", map[string]any{"day": day, "kg": 55}, 403, nil); e != nil {
		return e
	}
	a.csrf = wrong
	manifest["status"] = "passed"
	manifest["completed_at"] = time.Now().UTC()
	manifest["checks"] = []string{"registration", "secure session", "onboarding recommendation and explicit acceptance", "all five nutrition calculations", "quantity correction", "stale update conflict", "private food ownership", "diary/weight/profile isolation", "login persistence", "CSRF"}
	manifest["retained_records"] = map[string]any{"test_users": 2, "private_food_id": private.ID, "food_log_id": entry.ID, "weight_entries": 1, "sessions": 3}
	if e = save(reportPath, manifest); e != nil {
		return e
	}
	fmt.Println("PASS: Neon API validation completed. Two clearly labelled validation accounts and their scoped records were retained; no cleanup/reset was performed. Manifest:", filepath.Base(reportPath))
	return nil
}
