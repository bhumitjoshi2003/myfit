package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"myfit/backend/internal/domain"
	"time"
)

type RecommendationRecord struct {
	ID             int64                 `json:"id"`
	ProfileVersion int                   `json:"profile_version"`
	CalculatedAt   time.Time             `json:"calculated_at"`
	Result         domain.Recommendation `json:"result"`
	Decision       string                `json:"decision"`
}
type OnboardingState struct {
	User           domain.User           `json:"user"`
	Recommendation *RecommendationRecord `json:"recommendation"`
	LatestWeight   *domain.Weight        `json:"latest_weight"`
}

func (s *Store) Onboarding(ctx context.Context, uid int64) (OnboardingState, error) {
	var state OnboardingState
	var e error
	state.User, e = s.User(ctx, uid)
	if e != nil {
		return state, e
	}
	var rec RecommendationRecord
	var b []byte
	e = s.Pool.QueryRow(ctx, `SELECT id,profile_version,calculated_at,result,decision FROM nutrition_recommendations WHERE user_id=$1 ORDER BY id DESC LIMIT 1`, uid).Scan(&rec.ID, &rec.ProfileVersion, &rec.CalculatedAt, &b, &rec.Decision)
	if e == nil {
		if e = json.Unmarshal(b, &rec.Result); e != nil {
			return state, e
		}
		state.Recommendation = &rec
	} else if !errors.Is(e, pgx.ErrNoRows) {
		return state, e
	}
	var w domain.Weight
	e = s.Pool.QueryRow(ctx, `SELECT id,day::text,kg FROM weights WHERE user_id=$1 ORDER BY day DESC LIMIT 1`, uid).Scan(&w.ID, &w.Day, &w.KG)
	if e == nil {
		state.LatestWeight = &w
	} else if !errors.Is(e, pgx.ErrNoRows) {
		return state, e
	}
	return state, nil
}
func (s *Store) SaveOnboarding(ctx context.Context, uid int64, version int, p domain.OnboardingProfile, r domain.Recommendation, now time.Time) (OnboardingState, error) {
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return OnboardingState{}, e
	}
	defer tx.Rollback(ctx)
	pb, _ := json.Marshal(p)
	rb, _ := json.Marshal(r)
	tag, e := tx.Exec(ctx, `UPDATE users SET onboarding_profile=$3,timezone=$4,onboarding_version=onboarding_version+1 WHERE id=$1 AND onboarding_version=$2`, uid, version, string(pb), p.Timezone)
	if e != nil {
		return OnboardingState{}, e
	}
	if tag.RowsAffected() != 1 {
		return OnboardingState{}, ErrConflict
	}
	// A new calculation supersedes an unreviewed result but never overwrites active goals.
	if _, e = tx.Exec(ctx, `UPDATE nutrition_recommendations SET decision='rejected' WHERE user_id=$1 AND decision='pending'`, uid); e != nil {
		return OnboardingState{}, e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO nutrition_recommendations(user_id,profile_version,algorithm_version,calculated_at,profile_snapshot,result) VALUES($1,$2,$3,$4,$5,$6)`, uid, version+1, r.Version, now, string(pb), string(rb)); e != nil {
		return OnboardingState{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return OnboardingState{}, e
	}
	return s.Onboarding(ctx, uid)
}

var ErrManualRequired = errors.New("automatic targets are unavailable; enter clinician-guided manual goals")
var ErrOnboardingRequired = errors.New("complete your profile before choosing goals")

func (s *Store) ChooseGoals(ctx context.Context, uid int64, version int, recID int64, mode string, custom domain.Nutrition) (domain.User, error) {
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return domain.User{}, e
	}
	defer tx.Rollback(ctx)
	var v int
	var completed bool
	var profile []byte
	e = tx.QueryRow(ctx, `SELECT onboarding_version,onboarding_completed,onboarding_profile FROM users WHERE id=$1 FOR UPDATE`, uid).Scan(&v, &completed, &profile)
	if e != nil {
		return domain.User{}, e
	}
	if v != version {
		return domain.User{}, ErrConflict
	}
	if len(profile) == 0 {
		return domain.User{}, ErrOnboardingRequired
	}
	var id int64
	var b []byte
	e = tx.QueryRow(ctx, `SELECT id,result FROM nutrition_recommendations WHERE user_id=$1 ORDER BY id DESC LIMIT 1`, uid).Scan(&id, &b)
	if e != nil {
		return domain.User{}, e
	}
	if id != recID {
		return domain.User{}, ErrConflict
	}
	var rec domain.Recommendation
	if e = json.Unmarshal(b, &rec); e != nil {
		return domain.User{}, e
	}
	decision := "custom"
	source := "custom"
	goals := custom
	if mode == "recommendation" {
		if !rec.Supported {
			return domain.User{}, ErrManualRequired
		}
		goals = rec.Goals
		source = "recommendation"
		decision = "accepted"
	}
	if mode == "keep" {
		if !completed {
			return domain.User{}, ErrOnboardingRequired
		}
		decision = "rejected"
		_, e = tx.Exec(ctx, `UPDATE users SET onboarding_version=onboarding_version+1 WHERE id=$1`, uid)
	} else {
		g, _ := json.Marshal(goals)
		_, e = tx.Exec(ctx, `UPDATE users SET goals=$2,goals_source=$3,onboarding_completed=true,onboarding_version=onboarding_version+1 WHERE id=$1`, uid, string(g), source)
	}
	if e != nil {
		return domain.User{}, e
	}
	if _, e = tx.Exec(ctx, `UPDATE nutrition_recommendations SET decision=$3 WHERE user_id=$1 AND id=$2`, uid, id, decision); e != nil {
		return domain.User{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return domain.User{}, e
	}
	return s.User(ctx, uid)
}
