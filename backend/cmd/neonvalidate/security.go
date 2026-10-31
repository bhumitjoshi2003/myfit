package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"
)

func securityInspect(ctx context.Context, t target, reports string) error {
	c, tlsVersion, e := connect(ctx, t.Direct)
	if e != nil {
		return e
	}
	defer c.Close(ctx)
	queries := map[string]string{
		"roles":              `SELECT jsonb_agg(jsonb_build_object('name',rolname,'login',rolcanlogin,'superuser',rolsuper,'createdb',rolcreatedb,'createrole',rolcreaterole,'inherit',rolinherit,'replication',rolreplication,'bypassrls',rolbypassrls,'connection_limit',rolconnlimit) ORDER BY rolname) FROM pg_roles WHERE rolname NOT LIKE 'pg_%'`,
		"memberships":        `SELECT coalesce(jsonb_agg(jsonb_build_object('role',r.rolname,'member',m.rolname,'admin',a.admin_option,'inherit',a.inherit_option,'set',a.set_option) ORDER BY r.rolname,m.rolname),'[]') FROM pg_auth_members a JOIN pg_roles r ON r.oid=a.roleid JOIN pg_roles m ON m.oid=a.member WHERE r.rolname NOT LIKE 'pg_%' OR m.rolname NOT LIKE 'pg_%'`,
		"schemas":            `SELECT jsonb_agg(jsonb_build_object('name',nspname,'owner',pg_get_userbyid(nspowner),'acl',nspacl::text) ORDER BY nspname) FROM pg_namespace WHERE nspname NOT LIKE 'pg_%' AND nspname<>'information_schema'`,
		"database":           `SELECT jsonb_build_object('name',datname,'owner',pg_get_userbyid(datdba),'acl',datacl::text) FROM pg_database WHERE datname=current_database()`,
		"relations":          `SELECT jsonb_agg(jsonb_build_object('name',c.relname,'kind',c.relkind,'owner',pg_get_userbyid(c.relowner),'acl',c.relacl::text) ORDER BY c.relname) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind IN ('r','p','S','v','m')`,
		"default_privileges": `SELECT coalesce(jsonb_agg(jsonb_build_object('owner',pg_get_userbyid(d.defaclrole),'schema',n.nspname,'type',d.defaclobjtype,'acl',d.defaclacl::text)),'[]') FROM pg_default_acl d LEFT JOIN pg_namespace n ON n.oid=d.defaclnamespace`,
	}
	out := map[string]any{"endpoint": t.Endpoint, "database_name": t.Database, "tls": tlsVersion, "inspected_at": time.Now().UTC()}
	for name, query := range queries {
		var value []byte
		if e = c.QueryRow(ctx, query).Scan(&value); e != nil {
			return e
		}
		out[name] = json.RawMessage(value)
	}
	if e = save(filepath.Join(reports, "neon-role-inspection.json"), out); e != nil {
		return e
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(b))
	return nil
}
