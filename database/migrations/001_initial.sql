-- +goose Up
CREATE TABLE users (
 id bigserial PRIMARY KEY, email text NOT NULL UNIQUE, password_hash text NOT NULL,
 name text NOT NULL, timezone text NOT NULL DEFAULT 'Asia/Kolkata',
 goals jsonb NOT NULL DEFAULT '{"calories":2000,"protein":100,"carbs":250,"fat":65,"fibre":30}', created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE sessions (token_hash text PRIMARY KEY, user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE, csrf text NOT NULL, expires_at timestamptz NOT NULL);
CREATE INDEX sessions_user ON sessions(user_id);
CREATE INDEX sessions_expiry ON sessions(expires_at);
CREATE TABLE foods (
 id bigserial PRIMARY KEY, owner_id bigint REFERENCES users(id) ON DELETE CASCADE,
 name text NOT NULL CHECK(length(name) BETWEEN 1 AND 160), brand text NOT NULL DEFAULT '',
 calories double precision NOT NULL CHECK(calories BETWEEN 0 AND 1000),
 protein double precision NOT NULL CHECK(protein BETWEEN 0 AND 100),
 carbs double precision NOT NULL CHECK(carbs BETWEEN 0 AND 100),
 fat double precision NOT NULL CHECK(fat BETWEEN 0 AND 100),
 fibre double precision NOT NULL CHECK(fibre BETWEEN 0 AND 100),
 source text NOT NULL, approximate boolean NOT NULL DEFAULT true, version integer NOT NULL DEFAULT 1
);
CREATE INDEX foods_owner ON foods(owner_id);
CREATE INDEX foods_name ON foods(lower(name));
CREATE TABLE servings (id bigserial PRIMARY KEY, food_id bigint NOT NULL REFERENCES foods(id) ON DELETE CASCADE, name text NOT NULL, grams double precision NOT NULL CHECK(grams > 0 AND grams <= 10000));
CREATE INDEX servings_food ON servings(food_id);
CREATE TABLE food_logs (
 id bigserial PRIMARY KEY, user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 food_id bigint NOT NULL REFERENCES foods(id), day date NOT NULL,
 meal text NOT NULL CHECK(meal IN ('breakfast','lunch','dinner','snacks')),
 quantity double precision NOT NULL CHECK(quantity > 0 AND quantity <= 10000),
 serving_name text NOT NULL, serving_grams double precision NOT NULL CHECK(serving_grams > 0),
 food_name text NOT NULL, brand text NOT NULL, reference jsonb NOT NULL, nutrition jsonb NOT NULL,
 version integer NOT NULL DEFAULT 1, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX logs_user_day ON food_logs(user_id,day);
CREATE INDEX logs_recent ON food_logs(user_id,created_at DESC);
CREATE TABLE favourites (user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE, food_id bigint NOT NULL REFERENCES foods(id) ON DELETE CASCADE, PRIMARY KEY(user_id,food_id));
CREATE TABLE weights (id bigserial PRIMARY KEY, user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE, day date NOT NULL, kg double precision NOT NULL CHECK(kg BETWEEN 15 AND 500), UNIQUE(user_id,day));
-- +goose Down
DROP TABLE weights, favourites, food_logs, servings, foods, sessions, users;
