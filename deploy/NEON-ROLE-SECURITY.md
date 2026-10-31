# MyFit Neon least-privilege security validation — October 31, 2026

**Completed and passed.** Runtime role `myfit_runtime` is active on the verified dedicated MyFit Neon endpoint `ep-square-mode-azj6avgc`, database `neondb`. `neondb_owner` remains the separate migration owner. No production deployment was started.

## Inspection and user confirmation

Inspected roles, memberships, schemas, database ACLs, object ownership, table/sequence grants and default privileges before changes. The additional NOLOGIN `token_roles` role was reported; the user explicitly approved leaving it unchanged and continuing. Its role attributes and memberships match the pre-change inventory. No role-specific changes or memberships were assigned to it. Provider-owned `neon_auth` objects were not modified.

The approved plan narrowed the old SQL template: no broad grants on all sequences and no default grants on unreviewed future objects. Only the dedicated MyFit database's PUBLIC CREATE/TEMPORARY and public-schema CREATE paths were removed. This prevents the runtime inheriting TEMP creation through PUBLIC; the owner retains its implicit migration rights. Edunexify is not in this Neon project and was not accessed.

## Runtime permissions verified

| Capability | Effective result |
| --- | --- |
| Database CONNECT | Allowed for `neondb` |
| Schema USAGE | Allowed on `public` |
| App tables | SELECT, INSERT, UPDATE, DELETE on the eight explicit MyFit application tables |
| Application ID sequences | USAGE on exactly six; no SELECT or UPDATE/setval privileges |
| Goose version table | SELECT only |
| Goose ID sequence | No USAGE, SELECT or UPDATE |
| SUPERUSER / CREATEDB / CREATEROLE | All false |
| INHERIT / REPLICATION / BYPASSRLS | All false |
| Membership in other roles | **0** |
| Owned objects | **0** |
| Schema/database CREATE / TEMP | Denied |
| TRUNCATE / REFERENCES / TRIGGER / MAINTAIN | Not granted on app tables |
| Grant options | None |
| SQL role connection ceiling | 8; application pgx pool remains max 4, min 0 |

The database login has app-level access; end-user record isolation remains enforced by MyFit's authenticated authorization queries, tested below. No per-user PostgreSQL roles or claim of database RLS was introduced.

## Explicit security probes

Actual runtime authentication rejected CREATE TABLE, CREATE TEMP TABLE, CREATE SCHEMA, migration-tracker updates and SET ROLE `neondb_owner`, with **SQLSTATE 42501**.

Runtime ALTER TABLE and DROP TABLE were rejected with **42501** against an **empty, owner-owned probe created solely inside an uncommitted transaction**. No app/history table was the DROP target. The owner context and temporary SET permission were used only within this rollback-protected transaction. No probe objects or temporary permission changes remained.

The owner successfully executed CREATE/ALTER/DROP on the same uncommitted probe. Actual `store.Migrate` / Goose Up executed successfully with the owner's direct URL; there were no pending migrations and schema version stayed **5**. No new migration was fabricated, and no reset/down migration ran.

## Credentials and configuration

A password was generated from **32 cryptographically random bytes (256 bits)**. It was never printed or placed in a report or process argument. An initial attempt using a pre-hashed verifier was rejected by Neon and rolled back without creating the role or grants. Neon requires plaintext password input over TLS; the successful operation used a **bound parameter over certificate- and hostname-verified TLS**, a transaction-local setting and a server-side CREATE statement. Client SQL text did not embed the password. Server statement/duration/transaction/error-parameter logging settings and SCRAM storage were checked first; an audit-extension presence would have stopped creation for review. No raw error messages or credential-bearing query text were emitted.

The following ignored files are mode **0600**, with matching runtime/migration connection values:

- `deploy/neon-staging.env`
- `deploy/production.env`
- `deploy/migration.env`

`DATABASE_URL` uses `myfit_runtime` at the pooled endpoint. `MIGRATION_DATABASE_URL` remains `neondb_owner` at the matching direct endpoint; its existing credential was not rotated. The production Compose file forwards **only the runtime URL**, not the migration credential, into the API container. No credential files are copied by the Docker build context. No Git commit was performed; ignore rules exclude these env files. The temporary credential-recovery env was removed after confirming all active private files matched.

`deploy/neon-runtime-grants.sql` is the reviewed non-secret grant plan. Future migrations adding objects require explicit grant review; runtime receives no automatic blanket access to future objects. Creating roles through Neon Console/API can add privileged memberships, so runtime role provisioning uses SQL instead.

## Application integration results using myfit_runtime

All of these passed against the real Neon-backed store and actual HTTP handlers:

- Registration and secure HttpOnly host-only sessions.
- Login and persisted records through a fresh authenticated session.
- Personalized onboarding, backend goal calculation and explicit acceptance.
- Food search, serving selection, logging and all five nutrient calculations.
- Two medium bananas: **210.04 kcal**; editing to one: **105.02 kcal**.
- Stale food-log update rejected with 409.
- Weight recording and persistence at **54.5 kg**.
- User isolation for profile/recommendations, diary, weight, private foods and cross-user modification attempts.
- Missing/invalid CSRF protection.

Existing staging accounts/records were **not deleted or edited**. Private before/after row fingerprints prove every pre-test row is still present and unchanged. New clearly labelled validation users **3 and 4** and their scoped records were retained. Exact fixture ownership is recorded in `.run/neon-staging/neon-smoke-20261031T160400Z-9585ac9a.json`; cleanup still requires explicit approval.

Offline Go tests and Go vet passed, as did production Compose validation with `config --quiet`. No services were started by that validation.

## Local database observation

This role-hardening workflow issued **no local PostgreSQL writes**. A read-only comparison against the earlier staging baseline found unchanged local schema but two additional food-log rows (17 → 19) and corresponding sequence advancement (19 → 21). That baseline predates this task; the cause/timing of those local additions is not established by this workflow. No local records were removed or restored to conceal the difference. All role, grant, migration-owner and application validation writes in this step used the guarded Neon target only.

## Remaining deployment gates

The database least-privilege blocker is resolved. Remaining gates are the previously documented VPS architecture/headroom and 96 MiB admission checks, Neon/container workload and reconnect/soak validation, a real immutable image/release reference in place of the image placeholder, and approved host Nginx/routing/DNS activation with rollback readiness. The existing wildcard certificate paths remain prepared, but the host has not been contacted or reconfigured.

No DigitalOcean VPS, DNS, Nginx, production containers, Edunexify code/configuration/database, or existing certificate was modified during this step.

## Private evidence

- `.run/neon-staging/neon-role-inspection-before-runtime.json`
- `.run/neon-staging/runtime-provisioning.json`
- `.run/neon-staging/neon-runtime-security.json`
- `.run/neon-staging/neon-role-inspection.json` (final inventory)
- `.run/neon-staging/runtime-existing-data-preserved.json`
- `.run/neon-staging/neon-smoke-20261031T160400Z-9585ac9a.json`

No evidence report contains a password, full connection string or session/CSRF credential.

References: [Neon SQL-role and password requirements](https://neon.com/docs/manage/roles), [PostgreSQL GRANT](https://www.postgresql.org/docs/18/sql-grant.html). These are source references for the policy; the permission and application results above are observed tests on the confirmed MyFit database.
