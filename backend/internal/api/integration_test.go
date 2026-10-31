package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"math"
	"myfit/backend/internal/domain"
	"myfit/backend/internal/store"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"
)

type client struct {
	t      *testing.T
	h      http.Handler
	cookie *http.Cookie
	csrf   string
}

func (c *client) req(method, path string, body any, status int) []byte {
	c.t.Helper()
	var b bytes.Buffer
	if body != nil {
		json.NewEncoder(&b).Encode(body)
	}
	r := httptest.NewRequest(method, path, &b)
	r.Header.Set("Origin", "http://localhost:5173")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", c.csrf)
	if c.cookie != nil {
		r.AddCookie(c.cookie)
	}
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	if w.Code != status {
		c.t.Fatalf("%s %s: status %d want %d: %s", method, path, w.Code, status, w.Body.String())
	}
	for _, v := range w.Result().Cookies() {
		c.cookie = v
	}
	return w.Body.Bytes()
}
func parse[T any](t *testing.T, b []byte) T {
	t.Helper()
	var v T
	if e := json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	return v
}
func (c *client) register(email string) {
	v := parse[struct{ CSRF string }](c.t, c.req("POST", "/api/register", map[string]string{"name": "Test", "email": email, "password": "very-secure-test-password", "timezone": "Asia/Kolkata"}, 200))
	c.csrf = v.CSRF
}
func TestIntegrationAcceptance(t *testing.T) {
	db := integrationStore(t)
	h := New(db, "http://localhost:5173", false)
	a := client{t: t, h: h}
	b := client{t: t, h: h}
	a.register("first@example.test")
	completeTestOnboarding(&a)
	b.register("second@example.test")
	completeTestOnboarding(&b)
	foods := parse[[]domain.Food](t, a.req("GET", "/api/foods?q=Banana%2C%20raw", nil, 200))
	if len(foods) != 1 {
		t.Fatal("expected banana")
	}
	f := foods[0]
	var serving int64
	for _, s := range f.Servings {
		if s.Grams == 118 {
			serving = s.ID
		}
	}
	if serving == 0 {
		t.Fatal("missing medium banana")
	}
	day := time.Now().Format("2006-01-02")
	body := map[string]any{"food_id": f.ID, "serving_id": serving, "quantity": 2, "day": day, "meal": "breakfast"}
	l := parse[domain.Log](t, a.req("POST", "/api/logs", body, 200))
	if math.Abs(l.Nutrition.Calories-210.04) > .0001 {
		t.Fatal(l)
	}
	diary := func(c *client) struct {
		Logs   []domain.Log
		Totals domain.Nutrition
	} {
		return parse[struct {
			Logs   []domain.Log
			Totals domain.Nutrition
		}](t, c.req("GET", "/api/diary?date="+day, nil, 200))
	}
	if d := diary(&a); len(d.Logs) != 1 || d.Totals != l.Nutrition {
		t.Fatal("aggregation incorrect")
	}
	if len(diary(&b).Logs) != 0 {
		t.Fatal("logs leaked")
	}
	path := fmt.Sprintf("/api/logs/%d", l.ID)
	edit := map[string]any{"quantity": 1, "day": day, "meal": "breakfast", "version": l.Version}
	b.req("PUT", path, edit, 404)
	b.req("DELETE", path, map[string]int{"version": 1}, 404)
	oldcsrf := a.csrf
	a.csrf = "wrong"
	a.req("PUT", path, edit, 403)
	a.csrf = oldcsrf
	l = parse[domain.Log](t, a.req("PUT", path, edit, 200))
	if math.Abs(diary(&a).Totals.Calories-105.02) > .0001 {
		t.Fatal("edit failed")
	}
	a.req("PUT", path, edit, 409)
	a.req("POST", "/api/weights", map[string]any{"day": day, "kg": 54.5}, 200)
	if len(parse[[]domain.Weight](t, b.req("GET", "/api/weights", nil, 200))) != 0 {
		t.Fatal("weight leaked")
	}
	private := parse[domain.Food](t, a.req("POST", "/api/foods", map[string]any{"name": "Private label food", "calories": 100, "protein": 10, "carbs": 10, "fat": 2, "fibre": 1, "source": "Test label", "servings": []map[string]any{{"name": "Portion", "grams": 50}}}, 200))
	if len(parse[[]domain.Food](t, b.req("GET", "/api/foods?q=Private", nil, 200))) != 0 {
		t.Fatal("food leaked")
	}
	body["food_id"] = private.ID
	body["serving_id"] = private.Servings[0].ID
	b.req("POST", "/api/logs", body, 404)
	b.req("PUT", fmt.Sprintf("/api/foods/%d/favourite", private.ID), map[string]bool{"enabled": true}, 404)
	pl := parse[domain.Log](t, a.req("POST", "/api/logs", body, 200))
	private.Calories = 800
	a.req("PUT", fmt.Sprintf("/api/foods/%d", private.ID), private, 200)
	pl = parse[domain.Log](t, a.req("PUT", fmt.Sprintf("/api/logs/%d", pl.ID), map[string]any{"quantity": 1, "day": day, "meal": "lunch", "version": pl.Version}, 200))
	if pl.Nutrition.Calories != 50 {
		t.Fatal("snapshot changed")
	}
	reuse := map[string]any{"reuse_id": pl.ID, "quantity": 1, "day": day, "meal": "snacks"}
	a.req("POST", "/api/logs", reuse, 200)
	b.req("POST", "/api/logs", reuse, 404)
	oldCookie := a.cookie
	a.req("POST", "/api/logout", map[string]any{}, 200)
	a.cookie = oldCookie
	a.req("GET", "/api/me", nil, 401)
	a.cookie = nil
	login := parse[struct{ CSRF string }](t, a.req("POST", "/api/login", map[string]string{"email": "first@example.test", "password": "very-secure-test-password"}, 200))
	a.csrf = login.CSRF
	if len(diary(&a).Logs) != 3 {
		t.Fatal("records did not persist")
	}
	ws := parse[[]domain.Weight](t, a.req("GET", "/api/weights", nil, 200))
	if len(ws) != 1 || ws[0].KG != 54.5 {
		t.Fatal("weight persistence failed")
	}
	me := parse[struct{ User domain.User }](t, a.req("GET", "/api/me", nil, 200))
	me.User.Timezone = "Pacific/Kiritimati"
	updated := parse[domain.User](t, a.req("PUT", "/api/profile", map[string]any{"name": me.User.Name, "timezone": me.User.Timezone, "goals": me.User.Goals, "version": me.User.OnboardingVersion}, 200))
	if updated.Profile.Timezone != me.User.Timezone {
		t.Fatal("profile timezone diverged")
	}
	if updated.GoalsSource != me.User.GoalsSource {
		t.Fatal("unchanged targets lost recommendation provenance")
	}
	actual := parse[struct{ Date string }](t, a.req("GET", "/api/diary", nil, 200))
	loc, _ := time.LoadLocation(me.User.Timezone)
	if actual.Date != time.Now().In(loc).Format("2006-01-02") {
		t.Fatal("timezone default wrong")
	}
	a.req("GET", "/api/diary?date=bad", nil, 400)
	r := httptest.NewRequest("POST", "/api/login", bytes.NewBufferString(`{}`))
	r.Header.Set("Origin", "https://evil.example")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross origin accepted")
	}
	a.req("DELETE", path, map[string]int{"version": l.Version}, 200)
}

func integrationStore(t *testing.T, legacy ...func(*pgxpool.Pool)) *store.Store {
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("set TEST_DATABASE_URL for PostgreSQL integration tests")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, raw)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(admin.Close)
	schema := "myfit_test_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if _, e = admin.Exec(ctx, `CREATE SCHEMA `+schema); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { admin.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`) })
	u, e := url.Parse(raw)
	if e != nil {
		t.Fatal(e)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	if len(legacy) > 0 {
		sqlDB, err := sql.Open("pgx", u.String())
		if err != nil {
			t.Fatal(err)
		}
		if err = goose.SetDialect("postgres"); err != nil {
			t.Fatal(err)
		}
		if err = goose.UpToContext(ctx, sqlDB, "../../../database/migrations", 4); err != nil {
			t.Fatal(err)
		}
		sqlDB.Close()
		old, err := pgxpool.New(ctx, u.String())
		if err != nil {
			t.Fatal(err)
		}
		legacy[0](old)
		old.Close()
	}
	db, e := store.Open(ctx, u.String(), "../../../database/migrations")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(db.Pool.Close)

	return db
}
