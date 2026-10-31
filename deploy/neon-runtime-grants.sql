-- Execute ONLY against the verified dedicated MyFit database, as neondb_owner.
-- Create myfit_runtime separately via SQL with a generated credential and no
-- elevated attributes or role memberships. Never put passwords in this script.
-- Example (credentials supplied privately, never in process arguments):
-- psql ... -v database_name=neondb -v runtime_role=myfit_runtime \
--   -f deploy/neon-runtime-grants.sql
\set ON_ERROR_STOP on
SELECT current_database() = :'database_name' AND current_user = 'neondb_owner' AS correct_target \gset
\if :correct_target
BEGIN;
-- PUBLIC normally grants TEMP on a new database. Remove that path to CREATE
-- TEMP TABLE for the runtime role, scoped ONLY to this dedicated MyFit DB.
-- The database/schema owner retains its implicit ownership privileges.
REVOKE CREATE, TEMPORARY ON DATABASE :"database_name" FROM PUBLIC;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT CONNECT ON DATABASE :"database_name" TO :"runtime_role";
GRANT USAGE ON SCHEMA public TO :"runtime_role";
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE
 public.users, public.sessions, public.foods, public.servings, public.food_logs,
 public.favourites, public.weights, public.nutrition_recommendations TO :"runtime_role";
-- Startup must read the schema version, but runtime must never change it.
GRANT SELECT ON TABLE public.goose_db_version TO :"runtime_role";
-- Only application-generated IDs; exclude goose_db_version_id_seq.
-- USAGE supports nextval/currval; SELECT and UPDATE/setval are not needed.
GRANT USAGE ON SEQUENCE
 public.users_id_seq, public.foods_id_seq, public.servings_id_seq,
 public.food_logs_id_seq, public.weights_id_seq,
 public.nutrition_recommendations_id_seq TO :"runtime_role";
-- No ownership, role membership, grant option, CREATE, TRUNCATE, REFERENCES,
-- TRIGGER, MAINTAIN, Goose writes or migration sequence access are granted.
-- Future objects require explicit reviewed grants after their migrations;
-- no blanket ALTER DEFAULT PRIVILEGES grants to unreviewed future objects.
COMMIT;
\else
\echo 'Unexpected database or migration owner: refusing grants.'
\quit 1
\endif
