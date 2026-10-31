package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Production                                                     bool
	DatabaseURL, MigrationDatabaseURL, Origin, Addr, MigrationsDir string
	CookieSecure, AutoMigrate                                      bool
	PoolMax                                                        int32
	PoolIdle, PoolLifetime, ConnectTimeout                         time.Duration
}

func Load(get func(string) string) (Config, error) {
	value := func(k, d string) string {
		if s := get(k); s != "" {
			return s
		}
		return d
	}
	prod := value("APP_ENV", "development") == "production"
	c := Config{Production: prod, DatabaseURL: get("DATABASE_URL"), MigrationDatabaseURL: get("MIGRATION_DATABASE_URL"), Origin: value("APP_ORIGIN", "http://localhost:5173"), Addr: value("API_ADDR", "127.0.0.1:8080"), MigrationsDir: value("MIGRATIONS_DIR", "../database/migrations")}
	var e error
	c.CookieSecure, e = strconv.ParseBool(value("COOKIE_SECURE", "false"))
	if e != nil {
		return c, fmt.Errorf("COOKIE_SECURE must be a boolean")
	}
	autoDefault := "true"
	if prod {
		autoDefault = "false"
	}
	c.AutoMigrate, e = strconv.ParseBool(value("AUTO_MIGRATE", autoDefault))
	if e != nil {
		return c, fmt.Errorf("AUTO_MIGRATE must be a boolean")
	}
	max, e := strconv.Atoi(value("DB_MAX_CONNS", "4"))
	if e != nil || max < 1 || max > 20 {
		return c, fmt.Errorf("DB_MAX_CONNS must be 1–20")
	}
	c.PoolMax = int32(max)
	for _, v := range []struct {
		key, def string
		dest     *time.Duration
	}{{"DB_MAX_IDLE_TIME", "5m", &c.PoolIdle}, {"DB_MAX_LIFETIME", "30m", &c.PoolLifetime}, {"DB_CONNECT_TIMEOUT", "10s", &c.ConnectTimeout}} {
		*v.dest, e = time.ParseDuration(value(v.key, v.def))
		if e != nil || *v.dest <= 0 {
			return c, fmt.Errorf("invalid %s", v.key)
		}
	}
	if c.DatabaseURL == "" {
		if prod {
			return c, fmt.Errorf("DATABASE_URL is required in production")
		}
		c.DatabaseURL = "postgres://myfit:myfit_local_dev@localhost:5433/myfit?sslmode=disable"
	}
	if prod {
		if c.AutoMigrate {
			return c, fmt.Errorf("production requires AUTO_MIGRATE=false; run the explicit migrate command")
		}
		if !c.CookieSecure {
			return c, fmt.Errorf("production requires COOKIE_SECURE=true")
		}
		u, e := url.Parse(c.Origin)
		if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
			return c, fmt.Errorf("APP_ORIGIN must be an exact HTTPS origin without a trailing slash")
		}
		if e = ValidateDatabaseURL(c.DatabaseURL); e != nil {
			return c, e
		}
	}
	return c, nil
}
func ValidateDatabaseURL(raw string) error {
	u, e := url.Parse(raw)
	if e != nil || !strings.HasPrefix(u.Scheme, "postgres") || u.Hostname() == "" || u.User == nil || u.Path == "" || u.Path == "/" {
		return fmt.Errorf("a PostgreSQL connection URL is required")
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return fmt.Errorf("invalid PostgreSQL URL scheme")
	}
	if u.Query().Get("sslmode") != "verify-full" {
		return fmt.Errorf("production PostgreSQL requires sslmode=verify-full")
	}
	return nil
}
