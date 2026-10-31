package config

import (
	"testing"
	"time"
)

func productionEnv() map[string]string {
	return map[string]string{"APP_ENV": "production", "APP_ORIGIN": "https://myfit.edunexify.co.in", "COOKIE_SECURE": "true", "DATABASE_URL": "postgresql://runtime:example@ep-test-pooler.eu-central-1.aws.neon.tech/myfit?sslmode=verify-full"}
}
func load(values map[string]string) (Config, error) {
	return Load(func(k string) string { return values[k] })
}
func TestProductionConfig(t *testing.T) {
	c, e := load(productionEnv())
	if e != nil {
		t.Fatal(e)
	}
	if c.AutoMigrate || !c.CookieSecure || c.PoolMax != 4 || c.PoolIdle != 5*time.Minute {
		t.Fatal(c)
	}
}
func TestProductionFailsClosed(t *testing.T) {
	for key, value := range map[string]string{"DATABASE_URL": "", "COOKIE_SECURE": "false", "AUTO_MIGRATE": "true", "APP_ORIGIN": "http://myfit.edunexify.co.in", "DB_MAX_CONNS": "100", "DB_MAX_IDLE_TIME": "invalid"} {
		env := productionEnv()
		env[key] = value
		if _, e := load(env); e == nil {
			t.Errorf("accepted %s=%s", key, value)
		}
	}
	for _, origin := range []string{"https://myfit.edunexify.co.in/", "https://name:password@myfit.edunexify.co.in", "https://myfit.edunexify.co.in?x=1"} {
		env := productionEnv()
		env["APP_ORIGIN"] = origin
		if _, e := load(env); e == nil {
			t.Errorf("accepted origin %s", origin)
		}
	}
	env := productionEnv()
	env["DATABASE_URL"] = "postgresql://runtime:secret@ep-test.neon.tech/myfit?sslmode=require"
	if _, e := load(env); e == nil {
		t.Fatal("unverified TLS accepted")
	}
}
func TestDevelopmentDefaults(t *testing.T) {
	c, e := load(nil)
	if e != nil || c.Production || !c.AutoMigrate || c.CookieSecure {
		t.Fatal(c, e)
	}
}
