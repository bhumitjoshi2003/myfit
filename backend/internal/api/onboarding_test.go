package api

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"myfit/backend/internal/domain"
	"myfit/backend/internal/store"
	"testing"
)

func onboardingProfileFixture() domain.OnboardingProfile {
	return domain.OnboardingProfile{Age: 24, Sex: "male", Height: 167.64, HeightUnit: "cm", Weight: 54.5, WeightUnit: "kg", Units: "metric", Timezone: "Asia/Kolkata", Lifestyle: "sedentary", Sessions: 3.5, Minutes: 60, Intensity: "moderate", Training: "resistance", Objective: "muscle", Diet: "vegetarian", HealthContext: "healthy"}
}
func stateOf(c *client) store.OnboardingState {
	return parse[store.OnboardingState](c.t, c.req("GET", "/api/onboarding", nil, 200))
}
func saveCalculation(c *client, p domain.OnboardingProfile, version int) store.OnboardingState {
	return parse[store.OnboardingState](c.t, c.req("PUT", "/api/onboarding/profile", map[string]any{"version": version, "profile": p}, 200))
}
func completeTestOnboarding(c *client) {
	s := stateOf(c)
	s = saveCalculation(c, onboardingProfileFixture(), s.User.OnboardingVersion)
	c.req("POST", "/api/onboarding/goals", map[string]any{"version": s.User.OnboardingVersion, "recommendation_id": s.Recommendation.ID, "mode": "recommendation"}, 200)
}
func TestOnboardingLifecycle(t *testing.T) {
	db := integrationStore(t)
	h := New(db, "http://localhost:5173", false)
	a := client{t: t, h: h}
	b := client{t: t, h: h}
	a.register("onboarding@example.test")
	b.register("other@example.test")
	initial := stateOf(&a)
	if initial.User.OnboardingCompleted || initial.User.Goals.Calories != 0 || initial.User.GoalsSource != "unset" {
		t.Fatal("new account has universal goals")
	}
	a.req("POST", "/api/onboarding/goals", map[string]any{"version": 0, "recommendation_id": 1, "mode": "recommendation"}, 422)
	a.req("PUT", "/api/onboarding/profile", map[string]any{"version": 0, "profile": map[string]any{}}, 400)
	s := saveCalculation(&a, onboardingProfileFixture(), 0)
	if s.User.OnboardingCompleted || s.User.Goals.Calories != 0 || !s.Recommendation.Result.Supported {
		t.Fatal("calculation silently selected goals")
	}
	a.req("PUT", "/api/onboarding/profile", map[string]any{"version": 0, "profile": onboardingProfileFixture()}, 409)
	bState := saveCalculation(&b, onboardingProfileFixture(), 0)
	b.req("POST", "/api/onboarding/goals", map[string]any{"version": bState.User.OnboardingVersion, "recommendation_id": s.Recommendation.ID, "mode": "recommendation"}, 409)
	// Submitted nutrient values are never authoritative when accepting a recommendation.
	u := parse[domain.User](t, a.req("POST", "/api/onboarding/goals", map[string]any{"version": s.User.OnboardingVersion, "recommendation_id": s.Recommendation.ID, "mode": "recommendation", "goals": domain.Nutrition{Calories: 9999}}, 200))
	if u.Goals != s.Recommendation.Result.Goals || !u.OnboardingCompleted || u.GoalsSource != "recommendation" {
		t.Fatal(u)
	}
	custom := domain.Nutrition{Calories: 2200, Protein: 110, Carbs: 270, Fat: 70, Fibre: 30}
	u = parse[domain.User](t, a.req("PUT", "/api/profile", map[string]any{"name": u.Name, "timezone": u.Timezone, "goals": custom, "version": u.OnboardingVersion}, 200))
	if u.GoalsSource != "custom" {
		t.Fatal(u)
	}
	p := onboardingProfileFixture()
	p.Weight = 57
	p.Lifestyle = "light"
	s = saveCalculation(&a, p, u.OnboardingVersion)
	if s.User.Goals != custom || s.User.GoalsSource != "custom" {
		t.Fatal("profile edit overwrote goals")
	}
	u = parse[domain.User](t, a.req("POST", "/api/onboarding/goals", map[string]any{"version": s.User.OnboardingVersion, "recommendation_id": s.Recommendation.ID, "mode": "keep"}, 200))
	if u.Goals != custom {
		t.Fatal("rejected recommendation changed goals")
	}
	a.req("POST", "/api/weights", map[string]any{"day": "2026-10-31", "kg": 60}, 200)
	after := stateOf(&a)
	if after.User.Goals != custom || after.User.Profile.Weight != 57 || after.User.OnboardingVersion != u.OnboardingVersion {
		t.Fatal("weight log changed profile/goals")
	}
	// Unsupported population can finish with explicit manual targets, never automated ones.
	p.HealthContext = "clinician"
	s = saveCalculation(&a, p, u.OnboardingVersion)
	a.req("POST", "/api/onboarding/goals", map[string]any{"version": s.User.OnboardingVersion, "recommendation_id": s.Recommendation.ID, "mode": "recommendation"}, 422)
	a.req("POST", "/api/onboarding/goals", map[string]any{"version": s.User.OnboardingVersion, "recommendation_id": s.Recommendation.ID, "mode": "custom", "goals": custom}, 200)
	// CSRF and user isolation apply equally to the new profile API.
	old := a.csrf
	a.csrf = "invalid"
	a.req("PUT", "/api/onboarding/profile", map[string]any{"version": 0, "profile": p}, 403)
	a.csrf = old
	if stateOf(&b).User.Profile.Weight != 54.5 {
		t.Fatal("another user's profile was modified")
	}
	a.req("PUT", "/api/profile", map[string]any{"name": "stale", "timezone": "UTC", "goals": custom, "version": 0}, 409)
}
func TestLegacyAccountPreservesHistory(t *testing.T) {
	ctx := context.Background()
	hash := string(dummyHash)
	var id int64
	var l domain.Log
	legacy := domain.Nutrition{Calories: 2000, Protein: 100, Carbs: 250, Fat: 65, Fibre: 30}
	db := integrationStore(t, func(pool *pgxpool.Pool) {
		db := &store.Store{Pool: pool}
		// Simulate an account made before migration 005: no onboarding data and existing goals/history.
		g, _ := json.Marshal(legacy)
		if e := db.Pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,name,timezone,goals) VALUES('legacy@example.test',$1,'Legacy','Asia/Kolkata',$2) RETURNING id`, hash, string(g)).Scan(&id); e != nil {
			t.Fatal(e)
		}
		if _, e := db.SaveWeight(ctx, id, domain.Weight{Day: "2026-10-30", KG: 54.5}); e != nil {
			t.Fatal(e)
		}
		foods, e := db.Foods(ctx, id, "banana", "")
		if e != nil {
			t.Fatal(e)
		}
		f := foods[0]
		l, e = db.SaveLog(ctx, id, domain.Log{FoodID: f.ID, Day: "2026-10-30", Meal: "breakfast", Quantity: 1, ServingName: "Medium", ServingGrams: 118, FoodName: f.Name, Reference: f.Nutrition, Nutrition: f.Nutrition.Scale(118, 1)})
		if e != nil {
			t.Fatal(e)
		}
	})
	h := New(db, "http://localhost:5173", false)
	a := client{t: t, h: h}
	login := parse[struct {
		CSRF string
		User domain.User
	}](t, a.req("POST", "/api/login", map[string]string{"email": "legacy@example.test", "password": "not-a-real-password"}, 200))
	a.csrf = login.CSRF
	s := stateOf(&a)
	if s.User.OnboardingCompleted || s.User.Profile != nil || s.User.Goals != legacy || s.LatestWeight.KG != 54.5 {
		t.Fatal("legacy prefill unsafe")
	}
	s = saveCalculation(&a, onboardingProfileFixture(), 0)
	if s.User.Goals != legacy {
		t.Fatal("existing goals overwritten")
	}
	a.req("POST", "/api/onboarding/goals", map[string]any{"version": s.User.OnboardingVersion, "recommendation_id": s.Recommendation.ID, "mode": "custom", "goals": legacy}, 200)
	got, e := db.Log(ctx, id, l.ID)
	if e != nil || got.Nutrition != l.Nutrition {
		t.Fatal("history changed")
	}
	w, e := db.Weights(ctx, id)
	if e != nil || len(w) != 1 || w[0].KG != 54.5 {
		t.Fatal("weight history changed")
	}
}
