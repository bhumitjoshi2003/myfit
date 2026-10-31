package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"math"
	"myfit/backend/internal/config"
	"myfit/backend/internal/domain"
	"time"
)

type Store struct{ Pool *pgxpool.Pool }

var ErrConflict = errors.New("record changed; reload and try again")

// Open preserves the local development and integration-test migration behaviour.
func Open(ctx context.Context, url, migrations string) (*Store, error) {
	return OpenConfigured(ctx, config.Config{DatabaseURL: url, MigrationsDir: migrations, AutoMigrate: true, PoolMax: 4, PoolIdle: 5 * time.Minute, PoolLifetime: 30 * time.Minute, ConnectTimeout: 10 * time.Second})
}
func Migrate(ctx context.Context, url, dir string) error {
	db, e := sql.Open("pgx", url)
	if e != nil {
		return errors.New("invalid migration database configuration")
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(0)
	if e = goose.SetDialect("postgres"); e != nil {
		return e
	}
	return goose.UpContext(ctx, db, dir)
}
func OpenConfigured(ctx context.Context, c config.Config) (*Store, error) {
	if c.AutoMigrate {
		if e := Migrate(ctx, c.DatabaseURL, c.MigrationsDir); e != nil {
			return nil, e
		}
	}
	pc, e := pgxpool.ParseConfig(c.DatabaseURL)
	if e != nil {
		return nil, errors.New("invalid database configuration")
	}
	pc.MaxConns = c.PoolMax
	pc.MinConns = 0
	pc.MaxConnIdleTime = c.PoolIdle
	pc.MaxConnLifetime = c.PoolLifetime
	pc.MaxConnLifetimeJitter = time.Minute
	pc.ConnConfig.ConnectTimeout = c.ConnectTimeout
	// Avoid depending on session-persistent prepared statements through Neon PgBouncer.
	pc.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	pc.ConnConfig.StatementCacheCapacity = 0
	pc.ConnConfig.DescriptionCacheCapacity = 0
	p, e := pgxpool.NewWithConfig(ctx, pc)
	if e != nil {
		return nil, errors.New("could not create database pool")
	}
	startup, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if e = p.Ping(startup); e != nil {
		p.Close()
		return nil, e
	}
	if !c.AutoMigrate {
		migrations, e := goose.CollectMigrations(c.MigrationsDir, 0, math.MaxInt64)
		if e != nil {
			p.Close()
			return nil, e
		}
		var version int64
		e = p.QueryRow(startup, `SELECT COALESCE(max(version_id),0) FROM (SELECT DISTINCT ON(version_id) version_id,is_applied FROM goose_db_version ORDER BY version_id,id DESC) latest WHERE is_applied=true`).Scan(&version)
		if e != nil || len(migrations) == 0 || version != migrations[len(migrations)-1].Version {
			p.Close()
			return nil, errors.New("database schema is not at this release's migration version; run an approved migration or use a compatible image")
		}
	}
	return &Store{p}, nil
}

const userCols = `id,email,name,timezone,goals,onboarding_profile,onboarding_completed,onboarding_version,goals_source`

func scanUser(row pgx.Row) (u domain.User, err error) {
	var b, p []byte
	err = row.Scan(&u.ID, &u.Email, &u.Name, &u.Timezone, &b, &p, &u.OnboardingCompleted, &u.OnboardingVersion, &u.GoalsSource)
	if err == nil {
		err = json.Unmarshal(b, &u.Goals)
	}
	if err == nil && len(p) > 0 {
		err = json.Unmarshal(p, &u.Profile)
	}
	return
}
func (s *Store) CreateUser(ctx context.Context, email, hash, name, tz string) (domain.User, error) {
	return scanUser(s.Pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,name,timezone) VALUES($1,$2,$3,$4) RETURNING `+userCols, email, hash, name, tz))
}
func (s *Store) Credentials(ctx context.Context, email string) (int64, string, error) {
	var id int64
	var hash string
	err := s.Pool.QueryRow(ctx, `SELECT id,password_hash FROM users WHERE email=$1`, email).Scan(&id, &hash)
	return id, hash, err
}
func (s *Store) User(ctx context.Context, id int64) (domain.User, error) {
	return scanUser(s.Pool.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE id=$1`, id))
}
func (s *Store) Profile(ctx context.Context, id int64, name, tz string, g domain.Nutrition, version int) (domain.User, error) {
	b, _ := json.Marshal(g)
	u, e := scanUser(s.Pool.QueryRow(ctx, `UPDATE users SET name=$2,timezone=$3,onboarding_profile=CASE WHEN onboarding_profile IS NULL THEN NULL ELSE jsonb_set(onboarding_profile,'{timezone}',to_jsonb($3::text)) END,goals_source=CASE WHEN goals<>$4::jsonb THEN 'custom' ELSE goals_source END,goals=$4,onboarding_version=onboarding_version+1 WHERE id=$1 AND onboarding_version=$5 RETURNING `+userCols, id, name, tz, string(b), version))
	if errors.Is(e, pgx.ErrNoRows) {
		return u, ErrConflict
	}
	return u, e
}
func (s *Store) Session(ctx context.Context, hash string) (int64, string, error) {
	var id int64
	var csrf string
	err := s.Pool.QueryRow(ctx, `SELECT user_id,csrf FROM sessions WHERE token_hash=$1 AND expires_at>now()`, hash).Scan(&id, &csrf)
	return id, csrf, err
}
func (s *Store) NewSession(ctx context.Context, hash string, id int64, csrf string) error {
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `DELETE FROM sessions WHERE expires_at<now()`); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO sessions VALUES($1,$2,$3,$4)`, hash, id, csrf, time.Now().Add(7*24*time.Hour)); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (s *Store) DeleteSession(ctx context.Context, hash string) error {
	_, e := s.Pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash=$1`, hash)
	return e
}

const foodCols = `f.id,f.owner_id,f.name,f.brand,f.calories,f.protein,f.carbs,f.fat,f.fibre,f.source,f.approximate,f.version, EXISTS(SELECT 1 FROM favourites x WHERE x.user_id=$1 AND x.food_id=f.id), COALESCE((SELECT jsonb_agg(jsonb_build_object('id',v.id,'name',v.name,'grams',v.grams) ORDER BY v.id) FROM servings v WHERE v.food_id=f.id),'[]')`

func scanFood(row pgx.Row) (f domain.Food, e error) {
	var b []byte
	e = row.Scan(&f.ID, &f.OwnerID, &f.Name, &f.Brand, &f.Calories, &f.Protein, &f.Carbs, &f.Fat, &f.Fibre, &f.Source, &f.Approximate, &f.Version, &f.Favourite, &b)
	if e == nil {
		e = json.Unmarshal(b, &f.Servings)
	}
	return
}
func (s *Store) Food(ctx context.Context, uid, id int64) (domain.Food, error) {
	return scanFood(s.Pool.QueryRow(ctx, `SELECT `+foodCols+` FROM foods f WHERE f.id=$2 AND (f.owner_id IS NULL OR f.owner_id=$1)`, uid, id))
}
func (s *Store) Foods(ctx context.Context, uid int64, q, filter string) ([]domain.Food, error) {
	rows, e := s.Pool.Query(ctx, `SELECT `+foodCols+` FROM foods f WHERE (f.owner_id IS NULL OR f.owner_id=$1) AND (f.name ILIKE '%'||$2||'%' OR f.brand ILIKE '%'||$2||'%') AND ($3<>'favourites' OR EXISTS(SELECT 1 FROM favourites x WHERE x.user_id=$1 AND x.food_id=f.id)) AND ($3<>'recent' OR EXISTS(SELECT 1 FROM food_logs l WHERE l.user_id=$1 AND l.food_id=f.id)) AND ($3<>'private' OR f.owner_id=$1) ORDER BY (SELECT max(l.created_at) FROM food_logs l WHERE l.user_id=$1 AND l.food_id=f.id) DESC NULLS LAST, f.name LIMIT 100`, uid, q, filter)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.Food{}
	for rows.Next() {
		f, e := scanFood(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
func (s *Store) SaveFood(ctx context.Context, uid int64, f domain.Food) (domain.Food, error) {
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return f, e
	}
	defer tx.Rollback(ctx)
	if f.ID == 0 {
		e = tx.QueryRow(ctx, `INSERT INTO foods(owner_id,name,brand,calories,protein,carbs,fat,fibre,source,approximate) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`, uid, f.Name, f.Brand, f.Calories, f.Protein, f.Carbs, f.Fat, f.Fibre, f.Source, f.Approximate).Scan(&f.ID)
	} else {
		var id int64
		e = tx.QueryRow(ctx, `UPDATE foods SET name=$3,brand=$4,calories=$5,protein=$6,carbs=$7,fat=$8,fibre=$9,source=$10,approximate=$11,version=version+1 WHERE id=$2 AND owner_id=$1 AND version=$12 RETURNING id`, uid, f.ID, f.Name, f.Brand, f.Calories, f.Protein, f.Carbs, f.Fat, f.Fibre, f.Source, f.Approximate, f.Version).Scan(&id)
		if errors.Is(e, pgx.ErrNoRows) {
			return f, ErrConflict
		}
	}
	if e != nil {
		return f, e
	}
	if _, e = tx.Exec(ctx, `DELETE FROM servings WHERE food_id=$1`, f.ID); e != nil {
		return f, e
	}
	for _, v := range f.Servings {
		if _, e = tx.Exec(ctx, `INSERT INTO servings(food_id,name,grams) VALUES($1,$2,$3)`, f.ID, v.Name, v.Grams); e != nil {
			return f, e
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return f, e
	}
	return s.Food(ctx, uid, f.ID)
}
func (s *Store) Favourite(ctx context.Context, uid, id int64, enabled bool) error {
	if _, e := s.Food(ctx, uid, id); e != nil {
		return e
	}
	if enabled {
		_, e := s.Pool.Exec(ctx, `INSERT INTO favourites VALUES($1,$2) ON CONFLICT DO NOTHING`, uid, id)
		return e
	}
	_, e := s.Pool.Exec(ctx, `DELETE FROM favourites WHERE user_id=$1 AND food_id=$2`, uid, id)
	return e
}

const logCols = `id,food_id,day::text,meal,quantity,serving_name,serving_grams,food_name,brand,reference,nutrition,version`

func scanLog(row pgx.Row) (l domain.Log, e error) {
	var a, b []byte
	e = row.Scan(&l.ID, &l.FoodID, &l.Day, &l.Meal, &l.Quantity, &l.ServingName, &l.ServingGrams, &l.FoodName, &l.Brand, &a, &b, &l.Version)
	if e == nil {
		e = json.Unmarshal(a, &l.Reference)
	}
	if e == nil {
		e = json.Unmarshal(b, &l.Nutrition)
	}
	return
}
func (s *Store) Logs(ctx context.Context, uid int64, day string) ([]domain.Log, domain.Nutrition, error) {
	rows, e := s.Pool.Query(ctx, `SELECT `+logCols+` FROM food_logs WHERE user_id=$1 AND day=$2 ORDER BY id`, uid, day)
	out := []domain.Log{}
	var total domain.Nutrition
	if e != nil {
		return out, total, e
	}
	defer rows.Close()
	for rows.Next() {
		l, e := scanLog(rows)
		if e != nil {
			return out, total, e
		}
		out = append(out, l)
		total.Add(l.Nutrition)
	}
	return out, total, rows.Err()
}
func (s *Store) Log(ctx context.Context, uid, id int64) (domain.Log, error) {
	return scanLog(s.Pool.QueryRow(ctx, `SELECT `+logCols+` FROM food_logs WHERE id=$2 AND user_id=$1`, uid, id))
}
func (s *Store) SaveLog(ctx context.Context, uid int64, l domain.Log) (domain.Log, error) {
	a, _ := json.Marshal(l.Reference)
	b, _ := json.Marshal(l.Nutrition)
	if l.ID == 0 {
		return scanLog(s.Pool.QueryRow(ctx, `INSERT INTO food_logs(user_id,food_id,day,meal,quantity,serving_name,serving_grams,food_name,brand,reference,nutrition) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING `+logCols, uid, l.FoodID, l.Day, l.Meal, l.Quantity, l.ServingName, l.ServingGrams, l.FoodName, l.Brand, string(a), string(b)))
	}
	v, e := scanLog(s.Pool.QueryRow(ctx, `UPDATE food_logs SET day=$3,meal=$4,quantity=$5,nutrition=$6,version=version+1 WHERE user_id=$1 AND id=$2 AND version=$7 RETURNING `+logCols, uid, l.ID, l.Day, l.Meal, l.Quantity, string(b), l.Version))
	if errors.Is(e, pgx.ErrNoRows) {
		return l, ErrConflict
	}
	return v, e
}
func (s *Store) DeleteLog(ctx context.Context, uid, id int64, version int) error {
	tag, e := s.Pool.Exec(ctx, `DELETE FROM food_logs WHERE user_id=$1 AND id=$2 AND version=$3`, uid, id, version)
	if e == nil && tag.RowsAffected() == 0 {
		return ErrConflict
	}
	return e
}
func (s *Store) Weights(ctx context.Context, uid int64) ([]domain.Weight, error) {
	rows, e := s.Pool.Query(ctx, `SELECT id,day::text,kg FROM weights WHERE user_id=$1 ORDER BY day`, uid)
	out := []domain.Weight{}
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var w domain.Weight
		if e = rows.Scan(&w.ID, &w.Day, &w.KG); e != nil {
			return out, e
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
func (s *Store) SaveWeight(ctx context.Context, uid int64, w domain.Weight) (domain.Weight, error) {
	e := s.Pool.QueryRow(ctx, `INSERT INTO weights(user_id,day,kg) VALUES($1,$2,$3) ON CONFLICT(user_id,day) DO UPDATE SET kg=excluded.kg RETURNING id`, uid, w.Day, w.KG).Scan(&w.ID)
	return w, e
}
func (s *Store) DeleteWeight(ctx context.Context, uid, id int64) error {
	tag, e := s.Pool.Exec(ctx, `DELETE FROM weights WHERE user_id=$1 AND id=$2`, uid, id)
	if e == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return e
}
