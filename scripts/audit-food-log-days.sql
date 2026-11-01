-- READ-ONLY audit: which food_logs.day values disagree with the day the entry
-- was actually created, in each user's own time zone. Run against production
-- only with your approval:  psql "$DATABASE_URL" -f scripts/audit-food-log-days.sql
-- Nothing here modifies data.

-- 1) Users whose stored time zone differs from what you expect.
SELECT id, email, timezone FROM users ORDER BY id;

-- 2) Entries whose diary day != creation day in the user's stored zone.
SELECT l.id, l.user_id, u.timezone, l.day AS diary_day,
       (l.created_at AT TIME ZONE u.timezone)::date AS created_day_in_user_tz,
       (l.created_at AT TIME ZONE 'Asia/Kolkata') AS created_ist,
       l.meal, l.food_name, l.nutrition->>'calories' AS kcal
FROM food_logs l JOIN users u ON u.id = l.user_id
WHERE l.day <> (l.created_at AT TIME ZONE u.timezone)::date
ORDER BY l.created_at;

-- 3) The case reported: entries filed under 2026-11-01 with their IST creation time.
--    created_ist before 2026-11-01 00:00 means the entry was logged on Oct 31 (IST)
--    and is a candidate to move; at/after midnight it genuinely belongs to Nov 1.
SELECT l.id, l.user_id, u.timezone, l.day, l.created_at AT TIME ZONE 'Asia/Kolkata' AS created_ist,
       l.meal, l.food_name, l.nutrition->>'calories' AS kcal,
       CASE WHEN l.created_at AT TIME ZONE 'Asia/Kolkata' < '2026-11-01' THEN 'MOVE to 2026-10-31' ELSE 'keep on 2026-11-01' END AS verdict
FROM food_logs l JOIN users u ON u.id = l.user_id
WHERE l.day = '2026-11-01'
ORDER BY l.created_at;

-- 4) Correction template (DO NOT RUN without approval and a backup/snapshot).
--    Only changes `day`; nutrition snapshots, quantity, version etc. are untouched.
--    Review the ids from query 3 first, then:
--
-- BEGIN;
-- UPDATE food_logs SET day = '2026-10-31'
--  WHERE id IN (/* ids marked MOVE above */) AND day = '2026-11-01';
-- -- expect the row count you reviewed, then COMMIT; otherwise ROLLBACK;
