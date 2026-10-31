package main

import (
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture() string {
	return "DATABASE_URL=postgresql://myfit_runtime:NOT_A_REAL_PASSWORD@ep-myfit-fixture-pooler.us-east-2.aws.neon.tech/neondb?sslmode=verify-full&channel_binding=require\nMIGRATION_DATABASE_URL=postgresql://myfit_owner:NOT_A_REAL_PASSWORD@ep-myfit-fixture.us-east-2.aws.neon.tech/neondb?sslmode=verify-full\nNEON_EXPECTED_ENDPOINT=ep-myfit-fixture\nNEON_EXPECTED_DATABASE=neondb\nCONFIRM_DEDICATED_MYFIT=true\n"
}
func envFile(t *testing.T, s string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "private.env")
	if e := os.WriteFile(p, []byte(s), 0600); e != nil {
		t.Fatal(e)
	}
	return p
}
func TestLoadsPrivateTargetWithoutFallback(t *testing.T) {
	v, e := loadTarget(envFile(t, fixture()))
	if e != nil {
		t.Fatal(e)
	}
	if v.Endpoint != "ep-myfit-fixture" || v.Database != "neondb" {
		t.Fatal("target mismatch")
	}
	cfg, e := runtimeConfig(v, "migrations")
	if e != nil || !cfg.Production || cfg.AutoMigrate || cfg.PoolMax != 4 {
		t.Fatal("runtime production settings mismatch")
	}
}
func TestRefusesUnsafeTargets(t *testing.T) {
	cases := []string{
		strings.ReplaceAll(fixture(), "CONFIRM_DEDICATED_MYFIT=true", "CONFIRM_DEDICATED_MYFIT=false"),
		strings.ReplaceAll(fixture(), "sslmode=verify-full", "sslmode=disable"),
		strings.ReplaceAll(fixture(), "ep-myfit-fixture-pooler.us-east-2.aws.neon.tech", "localhost"),
		strings.ReplaceAll(fixture(), "ep-myfit-fixture-pooler", "ep-another-project-pooler"),
		strings.ReplaceAll(fixture(), "neondb?", "edunexify?"),
		strings.ReplaceAll(fixture(), "sslmode=verify-full", "sslmode=verify-full&search_path=somewhere"),
		strings.ReplaceAll(fixture(), ".neon.tech", ".neon.tech.evil.example"),
		strings.ReplaceAll(fixture(), "DATABASE_URL=postgresql://myfit_runtime:NOT_A_REAL_PASSWORD@ep-myfit-fixture-pooler.us-east-2.aws.neon.tech/neondb?sslmode=verify-full&channel_binding=require", "DATABASE_URL="),
		fixture() + "export SHELL_COMMAND=anything\n",
		fixture() + "DATABASE_URL=duplicate\n",
	}
	for i, s := range cases {
		if _, e := loadTarget(envFile(t, s)); e == nil {
			t.Errorf("unsafe fixture %d accepted", i)
		}
	}
}
func TestPrivatePermissionsAndSymlinks(t *testing.T) {
	p := envFile(t, fixture())
	os.Chmod(p, 0644)
	if _, e := loadTarget(p); e == nil {
		t.Fatal("public credential file accepted")
	}
	os.Chmod(p, 0600)
	link := filepath.Join(t.TempDir(), "link.env")
	if e := os.Symlink(p, link); e != nil {
		t.Fatal(e)
	}
	if _, e := loadTarget(link); e == nil {
		t.Fatal("credential symlink accepted")
	}
}
func TestErrorsSuppressDatabaseDetails(t *testing.T) {
	secret := "postgresql://user:NOT_A_REAL_PASSWORD@host/db"
	e := fmt.Errorf("wrapped: %w", &pgconn.PgError{Code: "42501", Message: secret})
	s := safeError(e)
	if strings.Contains(s, "PASSWORD") || strings.Contains(s, "postgresql") || !strings.Contains(s, "42501") {
		t.Fatal("SQL error was not safely formatted")
	}
	s = safeError(&pgconn.ConnectError{})
	if strings.Contains(s, "postgresql") {
		t.Fatal("connection error leaked")
	}
	if !strings.Contains(safeError(errors.New("known safe validation failure")), "known safe") {
		t.Fatal("validation error hidden")
	}
}
func TestReviewedMigrationsPresent(t *testing.T) {
	h, e := migrationHash("../../../database/migrations")
	if e != nil || len(h) != 64 {
		t.Fatal("migration manifest could not be read")
	}
}
