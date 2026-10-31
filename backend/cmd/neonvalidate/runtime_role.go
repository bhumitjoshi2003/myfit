package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const runtimeRole = "myfit_runtime"
const migrationRole = "neondb_owner"

var appSequences = []string{"users_id_seq", "foods_id_seq", "servings_id_seq", "food_logs_id_seq", "weights_id_seq", "nutrition_recommendations_id_seq"}

func writePrivate(path string, b []byte) error {
	if fi, e := os.Lstat(path); e == nil && (!fi.Mode().IsRegular() || fi.Mode().Perm()&0077 != 0) {
		return errors.New("refusing to overwrite an unprotected or symlinked private configuration file")
	}
	tmp, e := os.OpenFile(path+".tmp", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return errors.New("cannot create private temporary credential file")
	}
	if _, e = tmp.Write(b); e != nil {
		tmp.Close()
		return errors.New("cannot write private credential file")
	}
	if e = tmp.Sync(); e != nil {
		tmp.Close()
		return errors.New("cannot sync private credential file")
	}
	if e = tmp.Close(); e != nil {
		return errors.New("cannot close private credential file")
	}
	if e = os.Rename(path+".tmp", path); e != nil {
		return errors.New("cannot atomically install private credential file")
	}
	return nil
}
func updatePrivateEnv(path, template string, values map[string]string, compose bool) error {
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) && template != "" {
		b, e = os.ReadFile(template)
	}
	if e != nil {
		return errors.New("cannot read private configuration or its example")
	}
	lines := strings.Split(string(b), "\n")
	seen := map[string]bool{}
	value := func(k string) string {
		v := values[k]
		if compose {
			return "'" + strings.ReplaceAll(v, "'", "%27") + "'"
		}
		return v
	}
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		k, _, ok := strings.Cut(line, "=")
		k = strings.TrimSpace(k)
		if ok {
			if _, replace := values[k]; replace {
				lines[i] = k + "=" + value(k)
				seen[k] = true
			}
		}
	}
	keys := []string{}
	for k := range values {
		if !seen[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		lines = append(lines, k+"="+value(k))
	}
	return writePrivate(path, []byte(strings.TrimRight(strings.Join(lines, "\n"), "\n")+"\n"))
}
func reviewedGrantStatements(path, dbName string) ([]string, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, errors.New("cannot read reviewed grant plan")
	}
	clean := []string{}
	for _, line := range strings.Split(string(b), "\n") {
		if at := strings.Index(line, "--"); at >= 0 {
			line = line[:at]
		}
		clean = append(clean, line)
	}
	sql := strings.Join(clean, "\n")
	start := strings.Index(sql, "\nBEGIN;")
	end := strings.Index(sql, "\nCOMMIT;")
	if start < 0 || end <= start {
		return nil, errors.New("grant plan transaction markers are missing")
	}
	sql = sql[start+len("\nBEGIN;") : end]
	sql = strings.ReplaceAll(sql, `:"database_name"`, pgx.Identifier{dbName}.Sanitize())
	sql = strings.ReplaceAll(sql, `:"runtime_role"`, pgx.Identifier{runtimeRole}.Sanitize())
	if strings.Contains(sql, ":\"") || strings.Contains(sql, "ALTER DEFAULT PRIVILEGES") {
		return nil, errors.New("unreviewed grant substitution or blanket future grant")
	}
	out := []string{}
	for _, statement := range strings.Split(sql, ";") {
		if statement = strings.TrimSpace(statement); statement != "" {
			out = append(out, statement)
		}
	}
	if len(out) != 7 {
		return nil, errors.New("expected exactly seven reviewed scope-limited grant/revoke statements")
	}
	return out, nil
}
func assertRoleTarget(ctx context.Context, c *pgx.Conn, t target) error {
	if t.Endpoint != "ep-square-mode-azj6avgc" || t.Database != "neondb" {
		return errors.New("role operation target differs from the explicitly verified MyFit database")
	}
	var db, user string
	if e := c.QueryRow(ctx, `SELECT current_database(),current_user`).Scan(&db, &user); e != nil {
		return e
	}
	if db != t.Database || user != migrationRole {
		return errors.New("role operation requires the verified database and existing migration owner")
	}
	var roles, schemas []string
	if e := c.QueryRow(ctx, `SELECT array_agg(rolname::text ORDER BY rolname) FROM pg_roles WHERE rolname NOT LIKE 'pg_%'`).Scan(&roles); e != nil {
		return e
	}
	allowed := map[string]bool{"cloud_admin": true, "neon_auth": true, "neon_service": true, "neon_superuser": true, "neondb_owner": true, "token_roles": true, runtimeRole: true}
	for _, name := range roles {
		if !allowed[name] {
			return errors.New("unexpected role found; stop for user confirmation")
		}
	}
	if e := c.QueryRow(ctx, `SELECT array_agg(nspname::text ORDER BY nspname) FROM pg_namespace WHERE nspname NOT LIKE 'pg_%' AND nspname<>'information_schema'`).Scan(&schemas); e != nil {
		return e
	}
	if strings.Join(schemas, ",") != "neon_auth,public" {
		return errors.New("unexpected schema found; stop for user confirmation")
	}
	var bad int
	if e := c.QueryRow(ctx, `SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind IN ('r','p','S','v','m') AND pg_get_userbyid(c.relowner)<>$1`, migrationRole).Scan(&bad); e != nil {
		return e
	}
	if bad != 0 {
		return errors.New("unexpected public object owner; stop for user confirmation")
	}
	return nil
}
func provisionRuntime(ctx context.Context, t target, envPath, dir, reports string) error {
	// User explicitly approved leaving token_roles unchanged. Its role, memberships,
	// provider schemas and permissions are never modified by this operation.
	if e := verify(ctx, t, dir, reports); e != nil {
		return e
	}
	c, _, e := connect(ctx, t.Direct)
	if e != nil {
		return e
	}
	defer c.Close(ctx)
	if e = assertRoleTarget(ctx, c, t); e != nil {
		return e
	}
	var exists bool
	if e = c.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)`, runtimeRole).Scan(&exists); e != nil {
		return e
	}
	if exists {
		return errors.New("myfit_runtime already exists; no password/grant changes made. Inspect and confirm reuse rather than silently rotating it")
	}
	statements, e := reviewedGrantStatements(filepath.Join(filepath.Dir(envPath), "neon-runtime-grants.sql"), t.Database)
	if e != nil {
		return e
	}
	pending := filepath.Join(reports, "runtime-credential.pending.env")
	newRuntime, password, e := loadOrGenerateRuntimeCredential(pending, t)
	if e != nil {
		return e
	}
	var logStatement, logDuration, logSample, logTransaction, logErrorParams, encryption string
	if e = c.QueryRow(ctx, `SELECT current_setting('log_statement'),current_setting('log_min_duration_statement'),current_setting('log_min_duration_sample'),current_setting('log_transaction_sample_rate'),current_setting('log_parameter_max_length_on_error'),current_setting('password_encryption')`).Scan(&logStatement, &logDuration, &logSample, &logTransaction, &logErrorParams, &encryption); e != nil {
		return e
	}
	if logStatement != "none" || logDuration != "-1" || logSample != "-1" || logTransaction != "0" || logErrorParams != "0" || encryption != "scram-sha-256" {
		return errors.New("server credential-logging/password-encryption settings require review before role creation")
	}
	var audit bool
	if e = c.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname='pgaudit')`).Scan(&audit); e != nil {
		return e
	}
	if audit {
		return errors.New("database audit extension found; review password logging before role creation")
	}

	tx, e := c.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	// Neon does not accept pre-hashed passwords. Bind the password to a local
	// transaction setting, keeping it out of client statement text and error output.
	if _, e = tx.Exec(ctx, `SELECT set_config('myfit.provision_password',$1,true)`, password); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `DO $myfit_role$ BEGIN EXECUTE format('CREATE ROLE myfit_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS CONNECTION LIMIT 8 PASSWORD %L',current_setting('myfit.provision_password')); END $myfit_role$`); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `SELECT set_config('myfit.provision_password','',true)`); e != nil {
		return e
	}

	for _, sql := range statements {
		if _, e = tx.Exec(ctx, sql); e != nil {
			return e
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return e
	}
	var last error
	for attempt := 0; attempt < 4; attempt++ {
		runtime, _, err := connect(ctx, newRuntime)
		if err == nil {
			var name string
			err = runtime.QueryRow(ctx, `SELECT current_user`).Scan(&name)
			runtime.Close(ctx)
			if err == nil && name == runtimeRole {
				last = nil
				break
			}
			if err == nil {
				err = errors.New("runtime authentication returned an unexpected identity")
			}
		}
		last = err
		if attempt < 3 {
			time.Sleep(2 * time.Second)
		}
	}
	if last != nil {
		return last
	}
	deploy := filepath.Dir(envPath)
	values := map[string]string{"DATABASE_URL": newRuntime, "MIGRATION_DATABASE_URL": t.Direct}
	if e = updatePrivateEnv(envPath, "", map[string]string{"DATABASE_URL": newRuntime}, false); e != nil {
		return e
	}
	if e = updatePrivateEnv(filepath.Join(deploy, "production.env"), filepath.Join(deploy, "production.env.example"), values, true); e != nil {
		return e
	}
	if e = updatePrivateEnv(filepath.Join(deploy, "migration.env"), filepath.Join(deploy, "migration.env.example"), values, false); e != nil {
		return e
	}
	if e = save(filepath.Join(reports, "runtime-provisioning.json"), map[string]any{"role": runtimeRole, "migration_role": migrationRole, "endpoint": t.Endpoint, "database": t.Database, "created_at": time.Now().UTC(), "password_entropy_bits": 256, "sql_password_format": "Neon-required plaintext bound over verified TLS; server-side SCRAM storage; logging settings checked", "configured_files": []string{"deploy/neon-staging.env", "deploy/production.env", "deploy/migration.env"}, "grant_statements": len(statements), "connection_limit": 8, "token_roles": "left unchanged per user confirmation", "credential_recovery_file": "private pending env retained; contains secrets, never print"}); e != nil {
		return e
	}
	fmt.Println("PASS: created myfit_runtime with a cryptographically generated password, applied reviewed grants and authenticated through the pooled endpoint. Updated ignored mode-0600 environment files; migration-owner credential unchanged. No credentials printed.")
	return nil
}

func loadOrGenerateRuntimeCredential(path string, t target) (string, string, error) {
	source, _ := url.Parse(t.Runtime)
	if fi, e := os.Lstat(path); e == nil {
		if !fi.Mode().IsRegular() || fi.Mode().Perm()&0077 != 0 {
			return "", "", errors.New("pending credential file is not private or regular")
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return "", "", errors.New("cannot read pending private credential")
		}
		line := strings.TrimSpace(string(b))
		value, ok := strings.CutPrefix(line, "DATABASE_URL=")
		if !ok || strings.Contains(value, "\n") {
			return "", "", errors.New("invalid pending credential format")
		}
		u, e := url.Parse(value)
		if e != nil || u.User == nil || u.User.Username() != runtimeRole || u.Host != source.Host || u.Path != source.Path || u.RawQuery != source.RawQuery || u.Scheme != source.Scheme {
			return "", "", errors.New("pending credential does not match this runtime role and verified target")
		}
		password, ok := u.User.Password()
		bytes, e := hex.DecodeString(password)
		if !ok || e != nil || len(bytes) != 32 {
			return "", "", errors.New("pending credential entropy/format mismatch")
		}
		return value, password, nil
	} else if !os.IsNotExist(e) {
		return "", "", errors.New("cannot inspect pending private credential")
	}
	secret := make([]byte, 32)
	if _, e := rand.Read(secret); e != nil {
		return "", "", errors.New("secure password generation failed")
	}
	password := hex.EncodeToString(secret)
	source.User = url.UserPassword(runtimeRole, password)
	value := source.String()
	if e := writePrivate(path, []byte("DATABASE_URL="+value+"\n")); e != nil {
		return "", "", e
	}
	return value, password, nil
}
