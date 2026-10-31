package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewedLeastPrivilegeGrantPlan(t *testing.T) {
	s, e := reviewedGrantStatements("../../../deploy/neon-runtime-grants.sql", "neondb")
	if e != nil {
		t.Fatal(e)
	}
	all := strings.Join(s, ";")
	for _, required := range []string{"REVOKE CREATE, TEMPORARY", "GRANT SELECT ON TABLE public.goose_db_version", "GRANT USAGE ON SEQUENCE", "nutrition_recommendations_id_seq"} {
		if !strings.Contains(all, required) {
			t.Fatal("missing reviewed scope")
		}
	}
	for _, forbidden := range []string{"ALL SEQUENCES", "ALTER DEFAULT PRIVILEGES", "WITH GRANT OPTION", "GRANT CREATE", "GRANT ALL"} {
		if strings.Contains(all, forbidden) {
			t.Fatal("overbroad grant")
		}
	}
}
func TestPrivateEnvUpdateDoesNotPrintOrBroadenFiles(t *testing.T) {
	p := filepath.Join(t.TempDir(), "production.env")
	os.WriteFile(p, []byte("MYFIT_IMAGE=preserved\nDATABASE_URL=old\n"), 0600)
	if e := updatePrivateEnv(p, "", map[string]string{"DATABASE_URL": "private-test-value", "MIGRATION_DATABASE_URL": "private-owner-value"}, true); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(p)
	fi, _ := os.Stat(p)
	if fi.Mode().Perm() != 0600 || !strings.Contains(string(b), "MYFIT_IMAGE=preserved") || !strings.Contains(string(b), "DATABASE_URL='private-test-value'") {
		t.Fatal("private env update failed")
	}
	os.Chmod(p, 0644)
	if e := updatePrivateEnv(p, "", map[string]string{"DATABASE_URL": "replacement"}, true); e == nil {
		t.Fatal("unprotected file overwritten")
	}
}
func TestPendingCredentialIsPrivateAndTargetBound(t *testing.T) {
	t1, e := loadTarget(envFile(t, fixture()))
	if e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(t.TempDir(), "pending.env")
	a, password, e := loadOrGenerateRuntimeCredential(p, t1)
	if e != nil {
		t.Fatal(e)
	}
	if len(password) != 64 {
		t.Fatal("wrong entropy")
	}
	b, pw, e := loadOrGenerateRuntimeCredential(p, t1)
	if e != nil || a != b || pw != password {
		t.Fatal("private recovery changed credential")
	}
	t1.Runtime = strings.ReplaceAll(t1.Runtime, "ep-myfit-fixture-pooler", "ep-other-pooler")
	if _, _, e = loadOrGenerateRuntimeCredential(p, t1); e == nil {
		t.Fatal("cross-target recovery accepted")
	}
}
