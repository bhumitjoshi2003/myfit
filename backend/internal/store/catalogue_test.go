package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"myfit/backend/internal/config"
	"myfit/backend/internal/domain"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
)

// Dedicated opt-in local database only. Never reads DATABASE_URL or Neon env files.
func TestCatalogueMigration(t *testing.T) {
	raw := os.Getenv("TEST_CATALOGUE_DATABASE_URL")
	if raw == "" {
		t.Skip("use database/catalogue/test-local.py for an isolated local PostgreSQL database")
	}
	u, e := url.Parse(raw)
	if e != nil || u.Hostname() != "127.0.0.1" || !strings.HasPrefix(u.Path, "/myfit_catalogue_test_") || u.Query().Get("sslmode") != "disable" {
		t.Fatal("refusing non-local or non-disposable database")
	}
	ctx := context.Background()
	db, e := sql.Open("pgx", raw)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = goose.SetDialect("postgres"); e != nil {
		t.Fatal(e)
	}
	const migrations = "../../../database/migrations"
	if e = goose.UpToContext(ctx, db, migrations, 5); e != nil {
		t.Fatal(e)
	}
	pool, e := pgxpool.New(ctx, raw)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	store := &Store{pool}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := pool.Exec(ctx, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec(`INSERT INTO users(id,email,password_hash,name) VALUES(1,'catalogue-a@example.test','unused','Local catalogue test'),(2,'catalogue-b@example.test','unused','Local catalogue test')`)
	// Same private name must not suppress a shared insertion, nor be rewritten.
	exec(`INSERT INTO foods(owner_id,name,calories,protein,carbs,fat,fibre,source) VALUES(1,'Idli, steamed',99,1,1,1,1,'private fixture')`)
	// Case/outer-whitespace-equivalent shared name must be skipped, not overwritten.
	exec(`INSERT INTO foods(name,calories,protein,carbs,fat,fibre,source) VALUES('  mAnGo, raw, edible flesh  ',77,1,1,1,1,'pre-existing shared fixture')`)
	var bananaID int64
	if e = pool.QueryRow(ctx, `SELECT id FROM foods WHERE name='Banana, raw' AND owner_id IS NULL`).Scan(&bananaID); e != nil {
		t.Fatal(e)
	}
	exec(`INSERT INTO favourites(user_id,food_id) VALUES(1,$1)`, bananaID)
	banana, e := store.Food(ctx, 1, bananaID)
	if e != nil {
		t.Fatal(e)
	}
	original, e := store.SaveLog(ctx, 1, domain.Log{FoodID: bananaID, Day: "2026-11-01", Meal: "breakfast", Quantity: 2, ServingName: "Medium banana", ServingGrams: 118, FoodName: banana.Name, Reference: banana.Nutrition, Nutrition: banana.Nutrition.Scale(118, 2)})
	if e != nil {
		t.Fatal(e)
	}
	// Full row content and identities of all existing catalogue/user data must survive.
	tables := []string{"foods", "servings", "users", "favourites", "food_logs"}
	before := map[string][]string{}
	for _, table := range tables {
		rows, e := pool.Query(ctx, `SELECT row_to_json(t)::text FROM `+table+` t ORDER BY row_to_json(t)::text`)
		if e != nil {
			t.Fatal(e)
		}
		for rows.Next() {
			var v string
			if e = rows.Scan(&v); e != nil {
				t.Fatal(e)
			}
			before[table] = append(before[table], v)
		}
		if e = rows.Err(); e != nil {
			t.Fatal(e)
		}
		rows.Close()
	}
	if e = goose.UpContext(ctx, db, migrations); e != nil {
		t.Fatal(e)
	}
	var count int
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM foods WHERE owner_id IS NULL`).Scan(&count); e != nil || count != 219 {
		t.Fatalf("shared count %d (expected 22 + collision + 196 new): %v", count, e)
	}
	for table, rows := range before {
		for _, row := range rows {
			var exists bool
			if e = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM `+table+` t WHERE row_to_json(t)::text=$1)`, row).Scan(&exists); e != nil || !exists {
				t.Fatalf("existing %s row changed: %v", table, e)
			}
		}
	}
	var duplicateNames, duplicateServings int
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM (SELECT lower(btrim(name)) FROM foods WHERE owner_id IS NULL GROUP BY 1 HAVING count(*)>1) d`).Scan(&duplicateNames); e != nil || duplicateNames != 0 {
		t.Fatal("shared duplicates", duplicateNames, e)
	}
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM (SELECT food_id,name FROM servings GROUP BY 1,2 HAVING count(*)>1) d`).Scan(&duplicateServings); e != nil || duplicateServings != 0 {
		t.Fatal("serving duplicates", duplicateServings, e)
	}
	var foodsBefore, servingsBefore int
	pool.QueryRow(ctx, `SELECT count(*) FROM foods`).Scan(&foodsBefore)
	pool.QueryRow(ctx, `SELECT count(*) FROM servings`).Scan(&servingsBefore)
	// Replay the actual Goose Up SQL, not merely the already-recorded version.
	migration, e := os.ReadFile("../../../database/migrations/006_expanded_food_catalogue.sql")
	if e != nil {
		t.Fatal(e)
	}
	up := strings.Split(strings.Split(string(migration), "-- +goose Up")[1], "-- +goose Down")[0]
	tx, e := pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, up); e != nil {
		tx.Rollback(ctx)
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	pool.QueryRow(ctx, `SELECT count(*) FROM foods`).Scan(&count)
	if count != foodsBefore {
		t.Fatal("Up replay duplicated foods")
	}
	pool.QueryRow(ctx, `SELECT count(*) FROM servings`).Scan(&count)
	if count != servingsBefore {
		t.Fatal("Up replay duplicated servings")
	}
	for _, term := range []string{"idli", "dosa", "paratha", "rajma", "poha", "paneer", "soya", "mango", "lassi", "protein shake"} {
		a, e := store.Foods(ctx, 1, term, "")
		if e != nil {
			t.Fatal(e)
		}
		b, e := store.Foods(ctx, 2, term, "")
		if e != nil || len(b) == 0 {
			t.Fatalf("search %q invisible to second user: %v", term, e)
		}
		for _, f := range b {
			if f.OwnerID != nil {
				t.Fatal("private food leaked")
			}
		}
		if len(a) == 0 {
			t.Fatal("search returned no foods", term)
		}
	}
	idlis, e := store.Foods(ctx, 2, "Idli, steamed", "")
	if e != nil {
		t.Fatal(e)
	}
	var idli domain.Food
	for _, candidate := range idlis {
		if candidate.Name == "Idli, steamed" {
			idli = candidate
		}
	}
	if idli.ID == 0 {
		t.Fatal("shared idli missing")
	}
	var grams float64
	for _, s := range idli.Servings {
		if strings.HasPrefix(s.Name, "1 idli ") {
			grams = s.Grams
		}
	}
	if grams != 40 {
		t.Fatal("idli serving weight", grams)
	}
	scaled := idli.Nutrition.Scale(grams, 2)
	want := domain.Nutrition{Calories: idli.Nutrition.Calories * .8, Protein: idli.Nutrition.Protein * .8, Carbs: idli.Nutrition.Carbs * .8, Fat: idli.Nutrition.Fat * .8, Fibre: idli.Nutrition.Fibre * .8}
	aa, _ := json.Marshal(scaled)
	bb, _ := json.Marshal(want)
	if string(aa) != string(bb) {
		t.Fatalf("serving calculation mismatch: %s %s", aa, bb)
	}
	logged, e := store.SaveLog(ctx, 2, domain.Log{FoodID: idli.ID, Day: "2026-11-01", Meal: "breakfast", Quantity: 2, ServingName: "1 idli (estimated)", ServingGrams: grams, FoodName: idli.Name, Reference: idli.Nutrition, Nutrition: scaled})
	if e != nil {
		t.Fatal(e)
	}
	// Later catalogue edits must not change existing reference/nutrition snapshots.
	exec(`UPDATE foods SET calories=calories+10 WHERE id=$1`, idli.ID)
	persisted, e := store.Log(ctx, 2, logged.ID)
	if e != nil || persisted.Nutrition != logged.Nutrition || persisted.Reference != logged.Reference {
		t.Fatal("snapshot changed with catalogue", e)
	}
	old, e := store.Log(ctx, 1, original.ID)
	if e != nil || math.Abs(old.Nutrition.Calories-210.04) > .000001 {
		t.Fatal("old log changed", e)
	}
	// Restore only our disposable test row before validating read-only application startup.
	exec(`UPDATE foods SET calories=calories-10 WHERE id=$1`, idli.ID)
	opened, e := OpenConfigured(ctx, config.Config{DatabaseURL: raw, MigrationsDir: migrations, AutoMigrate: false, PoolMax: 4, PoolIdle: time.Minute, PoolLifetime: time.Hour, ConnectTimeout: time.Second})
	if e != nil {
		t.Fatal(e)
	}
	opened.Pool.Close()
	// Down intentionally keeps logged/favourited records; Up after Down stays idempotent.
	if e = goose.DownContext(ctx, db, migrations); e != nil {
		t.Fatal(e)
	}
	if e = goose.UpContext(ctx, db, migrations); e != nil {
		t.Fatal(e)
	}
	pool.QueryRow(ctx, `SELECT count(*) FROM foods`).Scan(&count)
	if count != foodsBefore {
		t.Fatal("Down/Up lost or duplicated foods")
	}
	started := time.Now()
	for i := 0; i < 20; i++ {
		if _, e = store.Foods(ctx, 2, "dosa", ""); e != nil {
			t.Fatal(e)
		}
	}
	t.Logf("20 real store searches: %s total; 219 shared foods, preservation, replay, snapshots, visibility and schema-v6 startup verified", time.Since(started))
}
