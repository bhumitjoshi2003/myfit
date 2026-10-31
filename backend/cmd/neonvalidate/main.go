// neonvalidate uses only an explicitly supplied, private Neon environment file.
// It has no local database fallback and never resets, drops, or cleans up data.
package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pressly/goose/v3"
	"myfit/backend/internal/config"
	"myfit/backend/internal/store"
)

const origin = "https://myfit.edunexify.co.in"

var tables = []string{"favourites", "food_logs", "foods", "goose_db_version", "nutrition_recommendations", "servings", "sessions", "users", "weights"}
var indexNames = []string{"users_pkey", "users_email_key", "sessions_pkey", "sessions_user", "sessions_expiry", "foods_pkey", "foods_owner", "foods_name", "servings_pkey", "servings_food", "food_logs_pkey", "logs_user_day", "logs_recent", "favourites_pkey", "weights_pkey", "weights_user_id_day_key", "nutrition_recommendations_pkey", "nutrition_recommendations_user_id_profile_version_key", "recommendations_user_latest", "goose_db_version_pkey"}

type target struct {
	Runtime, Direct, Endpoint, Database string
	Values                              map[string]string
}
type inspection struct {
	Endpoint      string    `json:"endpoint"`
	Database      string    `json:"database"`
	Role          string    `json:"migration_role"`
	ServerVersion string    `json:"server_version"`
	DirectTLS     string    `json:"direct_tls"`
	RuntimeTLS    string    `json:"runtime_tls"`
	Tables        []string  `json:"user_relations"`
	SchemaVersion int64     `json:"schema_version"`
	CanCreate     bool      `json:"migration_role_can_create"`
	InspectedAt   time.Time `json:"inspected_at"`
	MigrationHash string    `json:"migration_files_sha256"`
}

func main() {
	phase := flag.String("phase", "inspect", "inspect (read-only), migrate, verify (read-only), or smoke (new identifiable fixtures retained)")
	env := flag.String("env-file", "../deploy/neon-staging.env", "private credential file, never a connection URL")
	dir := flag.String("migrations", "../database/migrations", "existing Goose SQL directory")
	reportDir := flag.String("reports", "../.run/neon-staging", "private report directory")
	flag.Parse()
	log.SetOutput(io.Discard)
	goose.SetLogger(goose.NopLogger())
	if e := run(*phase, *env, *dir, *reportDir); e != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", safeError(e))
		os.Exit(1)
	}
}
func safeError(e error) string {
	var pg *pgconn.PgError
	if errors.As(e, &pg) {
		return "database operation failed (SQLSTATE " + pg.Code + "); raw server details suppressed"
	}
	var connect *pgconn.ConnectError
	if errors.As(e, &connect) {
		return "Neon connection failed; verify endpoint, role/password, TLS and network access (connection details suppressed)"
	}
	if errors.Is(e, context.DeadlineExceeded) {
		return "operation timed out; inspect Neon availability/cold start and pool contention"
	}
	var pe *os.PathError
	if errors.As(e, &pe) {
		return "local file operation failed; check the supplied file path and permissions"
	}
	return e.Error()
}
func loadTarget(path string) (target, error) {
	var out target
	fi, e := os.Lstat(path)
	if e != nil {
		return out, fmt.Errorf("credential file is unavailable")
	}
	if !fi.Mode().IsRegular() || fi.Mode().Perm()&0077 != 0 {
		return out, errors.New("credential file must be a regular, non-symlink file readable only by its owner (chmod 600)")
	}
	f, e := os.Open(path)
	if e != nil {
		return out, errors.New("credential file cannot be opened")
	}
	defer f.Close()
	values := map[string]string{}
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 4096), 65536)
	allowed := map[string]bool{"DATABASE_URL": true, "MIGRATION_DATABASE_URL": true, "NEON_EXPECTED_ENDPOINT": true, "NEON_EXPECTED_DATABASE": true, "CONFIRM_DEDICATED_MYFIT": true}
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || !allowed[key] {
			return out, errors.New("unsupported environment-file entry; use only the provided keys without shell commands")
		}
		if _, dup := values[key]; dup {
			return out, errors.New("duplicate environment-file entry")
		}
		value = strings.TrimSpace(value)
		if len(value) > 1 && ((value[0] == '\'' && value[len(value)-1] == '\'') || (value[0] == '"' && value[len(value)-1] == '"')) {
			value = value[1 : len(value)-1]
		}
		values[key] = value
	}
	if scan.Err() != nil {
		return out, errors.New("cannot parse environment file")
	}
	if values["CONFIRM_DEDICATED_MYFIT"] != "true" {
		return out, errors.New("set CONFIRM_DEDICATED_MYFIT=true only after verifying the dedicated MyFit project in Neon Console")
	}
	out = target{Runtime: values["DATABASE_URL"], Direct: values["MIGRATION_DATABASE_URL"], Endpoint: values["NEON_EXPECTED_ENDPOINT"], Database: values["NEON_EXPECTED_DATABASE"], Values: values}
	if !strings.HasPrefix(out.Endpoint, "ep-") || strings.ContainsAny(out.Endpoint, "./: \t\n") || strings.HasSuffix(out.Endpoint, "-pooler") || out.Database == "" {
		return out, errors.New("supply the expected direct endpoint ID (ep-...) and actual database name from Neon Console")
	}
	var runtimeHost string
	for i, raw := range []string{out.Runtime, out.Direct} {
		if raw == "" {
			return out, errors.New("both pooled DATABASE_URL and direct MIGRATION_DATABASE_URL are required")
		}
		if e := config.ValidateDatabaseURL(raw); e != nil {
			return out, errors.New("both URLs must be PostgreSQL URLs with sslmode=verify-full")
		}
		u, _ := url.Parse(raw)
		h := u.Hostname()
		label := strings.Split(h, ".")[0]
		expected := out.Endpoint
		if i == 0 {
			expected += "-pooler"
			runtimeHost = strings.Replace(h, "-pooler.", ".", 1)
		}
		if !strings.HasSuffix(h, ".neon.tech") || label != expected || strings.TrimPrefix(u.Path, "/") != out.Database || u.Fragment != "" || (u.Port() != "" && u.Port() != "5432") {
			return out, errors.New("URLs must match the confirmed Neon endpoint/database, using pooled runtime and direct migration hosts")
		}
		if i == 1 && h != runtimeHost {
			return out, errors.New("pooled and direct endpoints must identify the same Neon compute")
		}
		for k, v := range u.Query() {
			if (k != "sslmode" && k != "connect_timeout" && k != "channel_binding") || len(v) != 1 {
				return out, errors.New("only sslmode, connect_timeout and channel_binding URL options are accepted; search_path/host overrides are refused")
			}
		}
		password, ok := u.User.Password()
		if !ok || password == "" {
			return out, errors.New("both URLs require a password supplied only in the private file")
		}
	}
	return out, nil
}
func connect(ctx context.Context, raw string) (*pgx.Conn, string, error) {
	c, e := pgx.ParseConfig(raw)
	if e != nil {
		return nil, "", errors.New("invalid Neon connection settings (values suppressed)")
	}
	c.ConnectTimeout = 15 * time.Second
	c.DefaultQueryExecMode = pgx.QueryExecModeExec
	c.StatementCacheCapacity = 0
	c.DescriptionCacheCapacity = 0
	if c.TLSConfig == nil || c.TLSConfig.InsecureSkipVerify || len(c.Fallbacks) > 0 {
		return nil, "", errors.New("TLS configuration is not exclusively certificate-verified")
	}
	conn, e := pgx.ConnectConfig(ctx, c)
	if e != nil {
		return nil, "", e
	}
	tc, ok := conn.PgConn().Conn().(*tls.Conn)
	if !ok {
		conn.Close(ctx)
		return nil, "", errors.New("connection did not negotiate TLS")
	}
	state := tc.ConnectionState()
	if state.Version < tls.VersionTLS12 || len(state.VerifiedChains) == 0 {
		conn.Close(ctx)
		return nil, "", errors.New("TLS certificate chain verification failed")
	}
	if e = state.PeerCertificates[0].VerifyHostname(c.Host); e != nil {
		conn.Close(ctx)
		return nil, "", errors.New("TLS hostname verification failed")
	}
	return conn, tls.VersionName(state.Version), nil
}
func migrationHash(dir string) (string, error) {
	files, e := filepath.Glob(filepath.Join(dir, "*.sql"))
	if e != nil || len(files) != 5 {
		return "", errors.New("expected the five existing MyFit Goose migrations")
	}
	sort.Strings(files)
	h := sha256.New()
	for _, path := range files {
		b, e := os.ReadFile(path)
		if e != nil {
			return "", errors.New("cannot read migration files")
		}
		h.Write([]byte(filepath.Base(path)))
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func inspect(ctx context.Context, t target, dir string) (inspection, error) {
	var out inspection
	out.Endpoint = t.Endpoint
	out.Database = t.Database
	out.InspectedAt = time.Now().UTC()
	var e error
	out.MigrationHash, e = migrationHash(dir)
	if e != nil {
		return out, e
	}
	c, tlsVersion, e := connect(ctx, t.Direct)
	if e != nil {
		return out, e
	}
	defer c.Close(ctx)
	out.DirectTLS = tlsVersion
	tx, e := c.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)
	var db, activeSchema string
	e = tx.QueryRow(ctx, `SELECT current_database(),current_user,current_setting('server_version'),current_schema(),has_schema_privilege(current_user,'public','CREATE')`).Scan(&db, &out.Role, &out.ServerVersion, &activeSchema, &out.CanCreate)
	if e != nil {
		return out, e
	}
	if db != t.Database {
		return out, errors.New("connected database differs from the expected target")
	}
	if activeSchema != "public" {
		return out, errors.New("unexpected default schema; no migrations permitted outside public")
	}
	rows, e := tx.Query(ctx, `SELECT n.nspname||'.'||c.relname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind IN ('r','p','v','m','f') AND n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname NOT LIKE 'pg_toast%' AND n.nspname NOT LIKE 'pg_temp%' ORDER BY 1`)
	if e != nil {
		return out, e
	}
	out.Tables = []string{}
	for rows.Next() {
		var name string
		if e = rows.Scan(&name); e != nil {
			return out, e
		}
		out.Tables = append(out.Tables, name)
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return out, e
	}
	for _, name := range out.Tables {
		if name == "public.goose_db_version" {
			e = tx.QueryRow(ctx, `SELECT coalesce(max(version_id),0) FROM (SELECT DISTINCT ON(version_id) version_id,is_applied FROM public.goose_db_version ORDER BY version_id,id DESC) s WHERE is_applied`).Scan(&out.SchemaVersion)
			if e != nil {
				return out, e
			}
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return out, e
	}
	runtime, runtimeTLS, e := connect(ctx, t.Runtime)
	if e != nil {
		return out, e
	}
	defer runtime.Close(ctx)
	out.RuntimeTLS = runtimeTLS
	if e = runtime.QueryRow(ctx, `SELECT current_database()`).Scan(&db); e != nil {
		return out, e
	}
	if db != t.Database {
		return out, errors.New("pooled connection reached a different database")
	}
	return out, nil
}
func save(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return errors.New("report serialization failed")
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}
func run(phase, path, dir, reports string) error {
	if phase != "inspect" && phase != "migrate" && phase != "verify" && phase != "smoke" && phase != "security-inspect" && phase != "provision-runtime" && phase != "verify-runtime" && phase != "snapshot-existing" && phase != "verify-preserved" {
		return errors.New("unsupported phase")
	}
	t, e := loadTarget(path)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(reports, 0700); e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	state, e := inspect(ctx, t, dir)
	if e != nil {
		return e
	}
	switch phase {
	case "snapshot-existing":
		return dataPreservation(ctx, t, reports, false)
	case "verify-preserved":
		return dataPreservation(ctx, t, reports, true)
	case "provision-runtime":
		return provisionRuntime(ctx, t, path, dir, reports)
	case "verify-runtime":
		return verifyRuntime(ctx, t, dir, reports)
	case "security-inspect":
		return securityInspect(ctx, t, reports)
	case "inspect":
		if e = save(filepath.Join(reports, "neon-inspection.json"), state); e != nil {
			return e
		}
		b, _ := json.MarshalIndent(state, "", "  ")
		fmt.Println(string(b))
		fmt.Println("Inspection is read-only. No migrations or application records were written.")
		return nil
	case "migrate":
		var prior inspection
		b, e := os.ReadFile(filepath.Join(reports, "neon-inspection.json"))
		if e != nil {
			return errors.New("run the read-only inspect phase first")
		}
		if e = json.Unmarshal(b, &prior); e != nil {
			return errors.New("invalid inspection report")
		}
		if prior.Endpoint != state.Endpoint || prior.Database != state.Database || prior.MigrationHash != state.MigrationHash || time.Since(prior.InspectedAt) > time.Hour {
			return errors.New("inspection is stale or the target/migration files changed; inspect again")
		}
		if len(state.Tables) != 0 || state.SchemaVersion != 0 {
			return errors.New("target is not empty; no migration executed. Review its inspection report before any further change")
		}
		if !state.CanCreate {
			return errors.New("migration role lacks CREATE on public schema; provide the dedicated migration owner URL")
		}
		migrationCtx, cancelMigration := context.WithTimeout(ctx, 2*time.Minute)
		defer cancelMigration()
		if e = store.Migrate(migrationCtx, t.Direct, dir); e != nil {
			return e
		}
		fmt.Println("Applied existing Goose Up migrations 001–005 to the confirmed dedicated Neon target. No down/reset operation performed.")
		return verify(ctx, t, dir, reports)
	case "verify":
		return verify(ctx, t, dir, reports)
	case "smoke":
		if e = verify(ctx, t, dir, reports); e != nil {
			return e
		}
		return smoke(ctx, t, dir, reports)
	}
	return nil
}
