-- READ-ONLY inspection of one user's food-log days around 2026-10-31 / 2026-11-01.
-- Usage (use a read-only DB role if you have one):
--   psql "$DATABASE_URL" -v email='you@example.com' -f scripts/audit-food-log-days.sql
-- The whole script runs in a READ ONLY transaction that is always rolled back,
-- so the database rejects any write. It contains no UPDATE/DELETE.
BEGIN READ ONLY;
SET LOCAL statement_timeout = '15s';
\pset pager off

\echo '== 1. Stored account timezone =='
SELECT id AS user_id, timezone, onboarding_profile->>'timezone' AS profile_timezone
FROM users WHERE lower(email) = lower(:'email');

\echo '== 2. Every entry filed under, or created on, Oct 30 - Nov 2 (IST) =='
SELECT l.id, l.day AS diary_day,
       l.created_at AT TIME ZONE 'Asia/Kolkata' AS created_ist,
       l.created_at AS created_utc,
       (l.created_at AT TIME ZONE u.timezone)::date AS created_day_in_stored_tz,
       l.version, l.meal, l.food_name, round((l.nutrition->>'calories')::numeric, 2) AS kcal
FROM food_logs l JOIN users u ON u.id = l.user_id
WHERE lower(u.email) = lower(:'email')
  AND (l.day BETWEEN '2026-10-30' AND '2026-11-02'
       OR (l.created_at AT TIME ZONE 'Asia/Kolkata')::date BETWEEN '2026-10-30' AND '2026-11-02')
ORDER BY l.created_at;

\echo '== 3. Per-day totals: by diary day vs by IST creation day (is Oct 31 missing?) =='
SELECT 'diary_day' AS grouped_by, l.day::text AS day, count(*) AS entries,
       round(sum((l.nutrition->>'calories')::numeric), 2) AS kcal
FROM food_logs l JOIN users u ON u.id = l.user_id
WHERE lower(u.email) = lower(:'email') AND l.day BETWEEN '2026-10-28' AND '2026-11-02'
GROUP BY 2
UNION ALL
SELECT 'created_day_ist', (l.created_at AT TIME ZONE 'Asia/Kolkata')::date::text, count(*),
       round(sum((l.nutrition->>'calories')::numeric), 2)
FROM food_logs l JOIN users u ON u.id = l.user_id
WHERE lower(u.email) = lower(:'email')
  AND (l.created_at AT TIME ZONE 'Asia/Kolkata')::date BETWEEN '2026-10-28' AND '2026-11-02'
GROUP BY 2
ORDER BY 1, 2;

\echo '== 4. Candidates to move Nov 1 -> Oct 31 =='
-- MOVE: filed under Nov 1 but created before midnight IST, and never edited (version 1),
--       so the Nov 1 date was assigned at creation, not chosen by hand.
-- REVIEW: created before midnight IST but edited (version > 1); the date may be deliberate.
SELECT l.id, l.day AS diary_day, l.created_at AT TIME ZONE 'Asia/Kolkata' AS created_ist,
       l.version, l.meal, l.food_name, round((l.nutrition->>'calories')::numeric, 2) AS kcal,
       CASE WHEN l.version = 1 THEN 'MOVE to 2026-10-31' ELSE 'REVIEW (edited)' END AS verdict
FROM food_logs l JOIN users u ON u.id = l.user_id
WHERE lower(u.email) = lower(:'email') AND l.day = '2026-11-01'
  AND l.created_at AT TIME ZONE 'Asia/Kolkata' < '2026-11-01'
ORDER BY l.created_at;

ROLLBACK;
