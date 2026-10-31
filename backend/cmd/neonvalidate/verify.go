package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"myfit/backend/internal/config"
	"myfit/backend/internal/store"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

func runtimeConfig(t target, dir string) (config.Config, error) {
	v := map[string]string{"APP_ENV": "production", "APP_ORIGIN": origin, "COOKIE_SECURE": "true", "AUTO_MIGRATE": "false", "DATABASE_URL": t.Runtime, "MIGRATION_DATABASE_URL": t.Direct, "MIGRATIONS_DIR": dir, "DB_MAX_CONNS": "4"}
	return config.Load(func(k string) string { return v[k] })
}
func verify(ctx context.Context, t target, dir, reports string) error {
	c, tlsVersion, e := connect(ctx, t.Direct)
	if e != nil {
		return e
	}
	defer c.Close(ctx)
	var version int64
	e = c.QueryRow(ctx, `SELECT coalesce(max(version_id),0) FROM (SELECT DISTINCT ON(version_id) version_id,is_applied FROM public.goose_db_version ORDER BY version_id,id DESC) s WHERE is_applied`).Scan(&version)
	if e != nil {
		return e
	}
	if version != 5 {
		return errors.New("expected Goose schema version 5; no verification writes performed")
	}
	var actual []string
	e = c.QueryRow(ctx, `SELECT array_agg(tablename::text ORDER BY tablename) FROM pg_tables WHERE schemaname='public'`).Scan(&actual)
	if e != nil {
		return e
	}
	if strings.Join(actual, ",") != strings.Join(tables, ",") {
		return errors.New("public table list differs from the nine expected MyFit tables")
	}
	var cols, cons, indexes []byte
	if e = c.QueryRow(ctx, `SELECT coalesce(jsonb_agg(jsonb_build_object('table',table_name,'column',column_name,'type',data_type,'nullable',is_nullable,'default',column_default) ORDER BY table_name,ordinal_position),'[]') FROM information_schema.columns WHERE table_schema='public'`).Scan(&cols); e != nil {
		return e
	}
	if e = c.QueryRow(ctx, `SELECT jsonb_agg(jsonb_build_object('table',t.relname,'name',c.conname,'type',c.contype,'definition',pg_get_constraintdef(c.oid),'validated',c.convalidated) ORDER BY t.relname,c.conname) FROM pg_constraint c JOIN pg_class t ON t.oid=c.conrelid JOIN pg_namespace n ON n.oid=t.relnamespace WHERE n.nspname='public'`).Scan(&cons); e != nil {
		return e
	}
	var foreignKeys, unvalidated int
	e = c.QueryRow(ctx, `SELECT count(*) FILTER(WHERE c.contype='f'),count(*) FILTER(WHERE NOT c.convalidated) FROM pg_constraint c JOIN pg_class t ON t.oid=c.conrelid JOIN pg_namespace n ON n.oid=t.relnamespace WHERE n.nspname='public'`).Scan(&foreignKeys, &unvalidated)
	if e != nil {
		return e
	}
	if foreignKeys != 9 || unvalidated != 0 {
		return errors.New("expected nine validated foreign keys and no unvalidated constraints")
	}
	expectedFK := []string{"sessions:user_id:users:id", "foods:owner_id:users:id", "servings:food_id:foods:id", "food_logs:user_id:users:id", "food_logs:food_id:foods:id", "favourites:user_id:users:id", "favourites:food_id:foods:id", "weights:user_id:users:id", "nutrition_recommendations:user_id:users:id"}
	var actualFK []string
	e = c.QueryRow(ctx, `SELECT array_agg(t.relname||':'||a.attname||':'||f.relname||':'||b.attname ORDER BY t.relname,a.attname) FROM pg_constraint c JOIN pg_class t ON t.oid=c.conrelid JOIN pg_namespace n ON n.oid=t.relnamespace JOIN pg_class f ON f.oid=c.confrelid JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=c.conkey[1] JOIN pg_attribute b ON b.attrelid=c.confrelid AND b.attnum=c.confkey[1] WHERE c.contype='f' AND n.nspname='public'`).Scan(&actualFK)
	if e != nil {
		return e
	}
	sort.Strings(expectedFK)
	sort.Strings(actualFK)
	if strings.Join(expectedFK, ",") != strings.Join(actualFK, ",") {
		return errors.New("foreign-key relationships differ from the expected schema")
	}
	var names []string
	var invalid int
	if e = c.QueryRow(ctx, `SELECT array_agg(i.relname::text ORDER BY i.relname),count(*) FILTER(WHERE NOT x.indisvalid) FROM pg_index x JOIN pg_class i ON i.oid=x.indexrelid JOIN pg_class t ON t.oid=x.indrelid JOIN pg_namespace n ON n.oid=t.relnamespace WHERE n.nspname='public'`).Scan(&names, &invalid); e != nil {
		return e
	}
	expectedIndexes := append([]string(nil), indexNames...)
	sort.Strings(expectedIndexes)
	if invalid != 0 || strings.Join(names, ",") != strings.Join(expectedIndexes, ",") {
		return errors.New("indexes differ from the 20 expected valid MyFit indexes")
	}
	if e = c.QueryRow(ctx, `SELECT jsonb_agg(jsonb_build_object('table',tablename,'name',indexname,'definition',indexdef) ORDER BY tablename,indexname) FROM pg_indexes WHERE schemaname='public'`).Scan(&indexes); e != nil {
		return e
	}
	var sharedFoods, sharedServings, invalidSeeds int
	e = c.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.foods WHERE owner_id IS NULL),(SELECT count(*) FROM public.servings s JOIN public.foods f ON f.id=s.food_id WHERE f.owner_id IS NULL),(SELECT count(*) FROM public.foods WHERE owner_id IS NULL AND (NOT approximate OR source=''))`).Scan(&sharedFoods, &sharedServings, &invalidSeeds)
	if e != nil {
		return e
	}
	if sharedFoods != 22 || sharedServings != 68 || invalidSeeds != 0 {
		return errors.New("expected 22 sourced approximate shared foods and 68 shared servings")
	}
	var calories, grams, paneer, roti float64
	e = c.QueryRow(ctx, `SELECT f.calories,s.grams,(SELECT calories FROM public.foods WHERE owner_id IS NULL AND name='Paneer, plain'),(SELECT calories FROM public.foods WHERE owner_id IS NULL AND name='Roti, whole wheat, plain') FROM public.foods f JOIN public.servings s ON s.food_id=f.id WHERE f.owner_id IS NULL AND f.name='Banana, raw' AND s.name='Medium banana (edible portion)'`).Scan(&calories, &grams, &paneer, &roti)
	if e != nil {
		return e
	}
	if calories != 89 || grams != 118 || math.Abs(paneer-1278.0/4.184) > .00001 || roti != 238 {
		return errors.New("seed reference correction or banana serving validation failed")
	}
	cfg, e := runtimeConfig(t, dir)
	if e != nil {
		return e
	}
	db, e := store.OpenConfigured(ctx, cfg)
	if e != nil {
		return e
	}
	defer db.Pool.Close()
	var canCreate, ownsTables bool
	e = db.Pool.QueryRow(ctx, `SELECT has_schema_privilege(current_user,'public','CREATE'),EXISTS(SELECT 1 FROM pg_tables WHERE schemaname='public' AND tableowner=current_user)`).Scan(&canCreate, &ownsTables)
	if e != nil {
		return e
	}
	var wg sync.WaitGroup
	failures := make(chan error, 8)
	start := time.Now()
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			qctx, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			var dbName string
			if e := db.Pool.QueryRow(qctx, `SELECT current_database()`).Scan(&dbName); e != nil {
				failures <- e
			} else if dbName != t.Database {
				failures <- errors.New("pool returned a different database")
			}
		}()
	}
	wg.Wait()
	close(failures)
	for e := range failures {
		return e
	}
	stats := db.Pool.Stat()
	if stats.MaxConns() != 4 || stats.TotalConns() > 4 {
		return errors.New("pool exceeded the configured four-connection limit")
	}
	warnings := []string{}
	if canCreate || ownsTables {
		warnings = append(warnings, "Runtime role can create schema objects or owns tables. Staging can run, but configure the separate least-privilege runtime role before production; grants template is prepared, not automatically applied.")
	}
	report := map[string]any{"verified_at": time.Now().UTC(), "endpoint": t.Endpoint, "database": t.Database, "schema_version": version, "direct_tls": tlsVersion, "tables": actual, "columns": json.RawMessage(cols), "constraints": json.RawMessage(cons), "indexes": json.RawMessage(indexes), "foreign_keys": foreignKeys, "shared_foods": sharedFoods, "shared_servings": sharedServings, "runtime_can_create": canCreate, "runtime_owns_tables": ownsTables, "pool_max": stats.MaxConns(), "pool_total_after_probe": stats.TotalConns(), "pool_probe_requests": 8, "pool_probe_duration_ms": time.Since(start).Milliseconds(), "pool_empty_acquire_count": stats.EmptyAcquireCount(), "warnings": warnings}
	if e = save(filepath.Join(reports, "neon-schema-verification.json"), report); e != nil {
		return e
	}
	fmt.Println("PASS: verified TLS, Goose version 5, nine tables, 20 indexes, nine foreign keys, 22 shared foods, 68 servings, corrected seed nutrition and pooled connectivity (max four).")
	for _, warning := range warnings {
		fmt.Println("CONFIGURATION REQUIRED:", warning)
	}
	return nil
}
