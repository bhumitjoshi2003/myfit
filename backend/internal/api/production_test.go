package api

import (
	"context"
	"myfit/backend/internal/config"
	"myfit/backend/internal/store"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionCookieName(t *testing.T) {
	if SessionCookieName(true) != "__Host-myfit_session" || SessionCookieName(false) != "myfit_session" {
		t.Fatal("cookie namespace incorrect")
	}
}
func TestProductionCookiesAndReadOnlyStartup(t *testing.T) {
	db := integrationStore(t)
	ctx := context.Background()
	url := db.Pool.Config().ConnConfig.ConnString()
	c := config.Config{DatabaseURL: url, MigrationsDir: "../../../database/migrations", PoolMax: 2, PoolIdle: time.Minute, PoolLifetime: time.Hour, ConnectTimeout: time.Second, AutoMigrate: false}
	opened, e := store.OpenConfigured(ctx, c)
	if e != nil {
		t.Fatal(e)
	}
	opened.Pool.Close()
	if _, e = db.Pool.Exec(ctx, `DELETE FROM goose_db_version WHERE version_id=6`); e != nil {
		t.Fatal(e)
	}
	if opened, e = store.OpenConfigured(ctx, c); e == nil {
		opened.Pool.Close()
		t.Fatal("outdated schema was accepted")
	}
	var v int64
	if e = db.Pool.QueryRow(ctx, `SELECT max(version_id) FROM goose_db_version`).Scan(&v); e != nil || v != 5 {
		t.Fatal("startup unexpectedly migrated database", e)
	}
	// The fixture is discarded after this test; the startup check left its version at 5.

}
func TestSecureCookieAttributes(t *testing.T) {
	db := integrationStore(t)
	origin := "https://myfit.edunexify.co.in"
	apiHandler := New(db, origin, true)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Origin", origin)
		apiHandler.ServeHTTP(w, r)
	})
	c := client{t: t, h: h}
	c.register("production-cookie@example.test")
	cookie := c.cookie
	if cookie.Name != "__Host-myfit_session" || !cookie.Secure || !cookie.HttpOnly || cookie.Path != "/" || cookie.Domain != "" || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("insecure or shared cookie", cookie.Name)
	}
	c.req("POST", "/api/logout", map[string]any{}, 200)
	if c.cookie.Name != cookie.Name || c.cookie.Domain != "" || c.cookie.MaxAge != -1 {
		t.Fatal("logout cleared wrong cookie")
	}
	r := httptest.NewRequest("GET", "/livez", nil)
	w := httptest.NewRecorder()
	apiHandler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal("liveness failed")
	}
}
