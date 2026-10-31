package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"myfit/backend/internal/store"
	"net/url"
	"path/filepath"
	"time"
)

func expectDenied(ctx context.Context, tx pgx.Tx, statement string) error {
	if _, e := tx.Exec(ctx, "SAVEPOINT myfit_permission_check"); e != nil {
		return e
	}
	_, attempt := tx.Exec(ctx, statement)
	_, rollback := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT myfit_permission_check")
	if rollback != nil {
		return rollback
	}
	var pg *pgconn.PgError
	if !errors.As(attempt, &pg) || pg.Code != "42501" {
		return errors.New("permission probe did not return expected insufficient_privilege; transaction remains uncommitted")
	}
	return nil
}
func verifyRuntime(ctx context.Context, t target, dir, reports string) error {
	u, _ := url.Parse(t.Runtime)
	ownerURL, _ := url.Parse(t.Direct)
	if u.User.Username() != runtimeRole || ownerURL.User.Username() != migrationRole {
		return errors.New("runtime and migration credentials are not separated as required")
	}
	c, tlsVersion, e := connect(ctx, t.Runtime)
	if e != nil {
		return e
	}
	defer c.Close(ctx)
	var name string
	var super, createdb, createrole, inherit, repl, bypass, login, dbcreate, temp, schemacreate, schemausage bool
	var limit int
	e = c.QueryRow(ctx, `SELECT current_user,r.rolsuper,r.rolcreatedb,r.rolcreaterole,r.rolinherit,r.rolreplication,r.rolbypassrls,r.rolcanlogin,r.rolconnlimit,has_database_privilege(current_user,current_database(),'CREATE'),has_database_privilege(current_user,current_database(),'TEMP'),has_schema_privilege(current_user,'public','CREATE'),has_schema_privilege(current_user,'public','USAGE') FROM pg_roles r WHERE r.rolname=current_user`).Scan(&name, &super, &createdb, &createrole, &inherit, &repl, &bypass, &login, &limit, &dbcreate, &temp, &schemacreate, &schemausage)
	if e != nil {
		return e
	}
	if name != runtimeRole || super || createdb || createrole || inherit || repl || bypass || !login || dbcreate || temp || schemacreate || !schemausage || limit != 8 {
		return errors.New("runtime role flags or effective schema/database privileges are not least-privilege")
	}
	var memberships, owned int
	if e = c.QueryRow(ctx, `SELECT count(*) FROM pg_auth_members a JOIN pg_roles r ON r.oid=a.member WHERE r.rolname=current_user`).Scan(&memberships); e != nil {
		return e
	}
	if e = c.QueryRow(ctx, `SELECT count(*) FROM pg_shdepend d JOIN pg_roles r ON r.oid=d.refobjid WHERE d.refclassid='pg_authid'::regclass AND r.rolname=current_user AND d.deptype='o'`).Scan(&owned); e != nil {
		return e
	}
	if memberships != 0 || owned != 0 {
		return errors.New("runtime unexpectedly has inherited/settable roles or object ownership")
	}
	matrix := map[string]map[string]bool{}
	for _, table := range tables {
		rights := map[string]bool{}
		for _, priv := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER", "MAINTAIN", "SELECT WITH GRANT OPTION", "INSERT WITH GRANT OPTION", "UPDATE WITH GRANT OPTION", "DELETE WITH GRANT OPTION"} {
			var yes bool
			if e = c.QueryRow(ctx, `SELECT has_table_privilege(current_user,$1,$2)`, "public."+table, priv).Scan(&yes); e != nil {
				return e
			}
			want := priv == "SELECT" || (table != "goose_db_version" && (priv == "INSERT" || priv == "UPDATE" || priv == "DELETE"))
			if yes != want {
				return errors.New("unexpected effective table permission or grant option")
			}
			rights[priv] = yes
		}
		matrix[table] = rights
	}
	sequences := map[string]map[string]bool{}
	for _, seq := range append(append([]string(nil), appSequences...), "goose_db_version_id_seq") {
		rights := map[string]bool{}
		for _, priv := range []string{"USAGE", "SELECT", "UPDATE", "USAGE WITH GRANT OPTION"} {
			var yes bool
			if e = c.QueryRow(ctx, `SELECT has_sequence_privilege(current_user,$1,$2)`, "public."+seq, priv).Scan(&yes); e != nil {
				return e
			}
			want := seq != "goose_db_version_id_seq" && priv == "USAGE"
			if yes != want {
				return errors.New("unexpected effective sequence permission")
			}
			rights[priv] = yes
		}
		sequences[seq] = rights
	}
	// Real runtime authentication tests CREATE/TEMP, migration-table mutation and
	// owner impersonation. Each attempt is protected by a savepoint and rollback.
	tx, e := c.Begin(ctx)
	if e != nil {
		return e
	}
	for _, sql := range []string{"CREATE TABLE public.__myfit_denied_create_probe(id integer)", "CREATE TEMP TABLE __myfit_denied_temp_probe(id integer)", "CREATE SCHEMA __myfit_denied_schema_probe", "UPDATE public.goose_db_version SET version_id=version_id WHERE false", "SET LOCAL ROLE neondb_owner"} {
		if e = expectDenied(ctx, tx, sql); e != nil {
			tx.Rollback(ctx)
			return e
		}
	}
	if e = tx.Rollback(ctx); e != nil {
		return e
	}
	if e = c.Close(ctx); e != nil {
		return e
	}
	owner, _, e := connect(ctx, t.Direct)
	if e != nil {
		return e
	}
	defer owner.Close(ctx)
	if e = assertRoleTarget(ctx, owner, t); e != nil {
		return e
	}
	// Create an EMPTY, owner-owned probe only inside a transaction. Runtime ALTER
	// and DROP tests target this disposable object, never application/history tables.
	// The entire transaction (including temporary SET permission) is rolled back.
	ownerTx, e := owner.Begin(ctx)
	if e != nil {
		return e
	}
	defer ownerTx.Rollback(ctx)
	probe := "__myfit_acl_probe_" + random()[:12]
	ident := pgx.Identifier{"public", probe}.Sanitize()
	if _, e = ownerTx.Exec(ctx, "CREATE TABLE "+ident+" (id integer)"); e != nil {
		return e
	}
	if _, e = ownerTx.Exec(ctx, "ALTER TABLE "+ident+" ADD COLUMN owner_probe integer"); e != nil {
		return e
	}
	var maySet bool
	if e = ownerTx.QueryRow(ctx, `SELECT pg_has_role(current_user,'myfit_runtime','SET')`).Scan(&maySet); e != nil {
		return e
	}
	if !maySet {
		if _, e = ownerTx.Exec(ctx, `GRANT myfit_runtime TO neondb_owner WITH INHERIT FALSE, SET TRUE`); e != nil {
			return e
		}
	}
	if _, e = ownerTx.Exec(ctx, "SET LOCAL ROLE myfit_runtime"); e != nil {
		return e
	}
	var actor string
	if e = ownerTx.QueryRow(ctx, `SELECT current_user`).Scan(&actor); e != nil || actor != runtimeRole {
		return errors.New("DDL probe did not run as runtime role")
	}
	for _, sql := range []string{"ALTER TABLE " + ident + " ADD COLUMN runtime_probe integer", "DROP TABLE " + ident} {
		if e = expectDenied(ctx, ownerTx, sql); e != nil {
			return e
		}
	}
	if _, e = ownerTx.Exec(ctx, "RESET ROLE"); e != nil {
		return e
	}
	if _, e = ownerTx.Exec(ctx, "DROP TABLE "+ident); e != nil {
		return e
	}
	if e = ownerTx.Rollback(ctx); e != nil {
		return e
	}
	// Execute actual Goose Up against the existing version. At version 5 this is a
	// no-op; owner DDL was separately proven without committing probe objects.
	if e = store.Migrate(ctx, t.Direct, dir); e != nil {
		return e
	}
	var version int64
	if e = owner.QueryRow(ctx, `SELECT max(version_id) FROM public.goose_db_version WHERE is_applied`).Scan(&version); e != nil || version != 5 {
		return errors.New("migration-owner Goose check changed or failed schema version")
	}
	var leftovers int
	if e = owner.QueryRow(ctx, `SELECT count(*) FROM pg_class WHERE relname LIKE '__myfit_acl_probe_%' OR relname LIKE '__myfit_denied_%'`).Scan(&leftovers); e != nil {
		return e
	}
	if leftovers != 0 {
		return errors.New("permission probe objects were unexpectedly retained")
	}
	report := map[string]any{"verified_at": time.Now().UTC(), "endpoint": t.Endpoint, "database": t.Database, "runtime_role": name, "migration_role": migrationRole, "tls": tlsVersion, "login": login, "superuser": super, "createdb": createdb, "createrole": createrole, "inherit": inherit, "replication": repl, "bypassrls": bypass, "connection_limit": limit, "database_create": dbcreate, "database_temp": temp, "schema_create": schemacreate, "schema_usage": schemausage, "role_memberships": memberships, "owned_objects": owned, "table_permissions": matrix, "sequence_permissions": sequences, "denied_with_sqlstate_42501": []string{"CREATE TABLE", "CREATE TEMP TABLE", "CREATE SCHEMA", "ALTER TABLE (owner-owned uncommitted empty probe)", "DROP TABLE (owner-owned uncommitted empty probe)", "UPDATE migration tracker", "SET ROLE neondb_owner"}, "owner_ddl": "CREATE/ALTER/DROP succeeded on an uncommitted empty probe; transaction rolled back", "probe_objects_retained": leftovers, "goose_owner_up": "passed, no pending migrations, schema version 5", "token_roles": "not directly altered; no membership assigned to runtime"}
	if e = save(filepath.Join(reports, "neon-runtime-security.json"), report); e != nil {
		return e
	}
	b, _ := json.MarshalIndent(map[string]any{"runtime_role": name, "privileged_flags": false, "memberships": memberships, "owned_objects": owned, "ddl_denials": "CREATE/ALTER/DROP and TEMP/SCHEMA: SQLSTATE 42501", "migration_writes_denied": true, "owner_goose": "passed, version 5"}, "", "  ")
	fmt.Println(string(b))
	return nil
}
