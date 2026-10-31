-- +goose Up
ALTER TABLE users ADD COLUMN onboarding_profile jsonb;
ALTER TABLE users ADD COLUMN onboarding_completed boolean NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN onboarding_version integer NOT NULL DEFAULT 0 CHECK(onboarding_version>=0);
ALTER TABLE users ADD COLUMN goals_source text NOT NULL DEFAULT 'legacy' CHECK(goals_source IN ('legacy','unset','custom','recommendation'));
-- Existing chosen goals stay intact. New accounts have no universal calorie target.
ALTER TABLE users ALTER COLUMN goals SET DEFAULT '{"calories":0,"protein":0,"carbs":0,"fat":0,"fibre":0}';
ALTER TABLE users ALTER COLUMN goals_source SET DEFAULT 'unset';
CREATE TABLE nutrition_recommendations (
 id bigserial PRIMARY KEY, user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 profile_version integer NOT NULL, algorithm_version text NOT NULL,
 calculated_at timestamptz NOT NULL, profile_snapshot jsonb NOT NULL, result jsonb NOT NULL,
 decision text NOT NULL DEFAULT 'pending' CHECK(decision IN ('pending','accepted','rejected','custom')),
 UNIQUE(user_id,profile_version)
);
CREATE INDEX recommendations_user_latest ON nutrition_recommendations(user_id,id DESC);
-- +goose Down
DROP TABLE nutrition_recommendations;
ALTER TABLE users DROP COLUMN onboarding_profile, DROP COLUMN onboarding_completed, DROP COLUMN onboarding_version, DROP COLUMN goals_source;
-- No goal values or food/weight history are rewritten during rollback.
