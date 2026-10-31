package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"os"
	"path/filepath"
)

func dataPreservation(ctx context.Context, t target, reports string, compare bool) error {
	c, _, e := connect(ctx, t.Runtime)
	if e != nil {
		return e
	}
	defer c.Close(ctx)
	tx, e := c.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	fingerprints := map[string][]string{}
	for _, table := range tables {
		rows, e := tx.Query(ctx, "SELECT md5(row_to_json(t)::text) FROM "+pgx.Identifier{"public", table}.Sanitize()+" t ORDER BY 1")
		if e != nil {
			return e
		}
		values := []string{}
		for rows.Next() {
			var hash string
			if e = rows.Scan(&hash); e != nil {
				return e
			}
			values = append(values, hash)
		}
		rows.Close()
		if e = rows.Err(); e != nil {
			return e
		}
		fingerprints[table] = values
	}
	if e = tx.Commit(ctx); e != nil {
		return e
	}
	path := filepath.Join(reports, "runtime-existing-data-before.json")
	if !compare {
		if _, e = os.Lstat(path); e == nil {
			return errors.New("existing-record baseline already exists; do not silently overwrite it")
		}
		if e = save(path, fingerprints); e != nil {
			return e
		}
		fmt.Println("PASS: recorded private row fingerprints for existing staging records; no row values or credentials printed.")
		return nil
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return errors.New("existing-record baseline is missing")
	}
	old := map[string][]string{}
	if e = json.Unmarshal(b, &old); e != nil {
		return errors.New("cannot decode existing-record baseline")
	}
	counts := map[string]int{}
	for table, hashes := range old {
		available := map[string]int{}
		for _, h := range fingerprints[table] {
			available[h]++
		}
		for _, h := range hashes {
			if available[h] == 0 {
				return errors.New("an existing staging record changed or disappeared; stop and investigate without deleting data")
			}
			available[h]--
		}
		counts[table] = len(hashes)
	}
	if e = save(filepath.Join(reports, "runtime-existing-data-preserved.json"), map[string]any{"endpoint": t.Endpoint, "database": t.Database, "all_existing_rows_preserved": true, "existing_rows_by_table": counts}); e != nil {
		return e
	}
	fmt.Println("PASS: every pre-test staging row is still present and unchanged; only scoped validation records were added.")
	return nil
}
