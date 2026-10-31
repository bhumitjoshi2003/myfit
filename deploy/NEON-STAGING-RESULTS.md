# Neon staging validation results — October 31, 2026

**Security update:** the separate runtime role is now created and validated. The earlier owner-capable-runtime warning below is historical and resolved; see [NEON-ROLE-SECURITY.md](NEON-ROLE-SECURITY.md).

**Result: passed.** Existing MyFit Goose migrations were applied to the confirmed dedicated Neon target after read-only inspection. No database reset, down migration or test-data cleanup was performed. No Edunexify database, VPS, DNS, host Nginx or existing certificate was modified.

## Target and TLS

- Dedicated MyFit endpoint ID: `ep-square-mode-azj6avgc`; database: `neondb`.
- Server: PostgreSQL **18.6 (4e5fd08)**. Application/migration compatibility passed on this Neon version, although local development uses PostgreSQL 17.
- Direct migration connection and pooled runtime connection: **TLS 1.3**, with certificate-chain and hostname validation (`sslmode=verify-full`). No TLS verification bypass was used.
- Pre-migration inspection: **zero user relations**, Goose version **0**, expected `public` schema and migration-owner CREATE permission. The target was re-inspected immediately before DDL, with matching endpoint/database/migration-file digest.
- Connection strings and passwords were read only from the ignored local mode-0600 file `deploy/neon-staging.env`; they were not printed or copied into reports.

## Migrations, schema and seeds

- Applied existing **001–005 Up migrations** successfully; final Goose version **5**.
- Verified **9 tables**, **20 valid indexes** and **9 validated foreign keys**, including expected ownership relationships.
- Column types/defaults, constraints, index definitions and FK metadata are captured in the private schema-verification report.
- Verified **22 shared foods** and **68 shared serving definitions**, approximate/source flags, the 118 g medium banana serving, and the corrected paneer/roti references.
- **No migration errors observed.** No existing user/history data was overwritten; the target contained none before migrations.

## Real application checks against Neon

The actual Go HTTP handlers and production-configured pgx store were invoked locally with Neon persistence, without opening a web listener or deploying anything. Passed:

- Registration and isolated Secure/HttpOnly host-only session cookies.
- Required onboarding, backend recommendations and explicit acceptance of goals.
- Banana search and two medium servings: **210.04 kcal, 2.5724 g protein, 53.9024 g carbohydrate, 0.7788 g fat, 6.136 g fibre**.
- Correction to one medium banana: **105.02 kcal**, with server recalculation and stale-version rejection.
- Weight recording at **54.5 kg**, plus food/onboarding/weight persistence through a fresh login.
- Cross-user isolation for profile/recommendations, diary, weights and private food; cross-user modification attempts rejected.
- Invalid-CSRF mutations rejected.

This is a database/API integration validation, not a new browser/DigitalOcean/Nginx deployment test. Earlier local production-image/browser validation remains documented separately.

## Pool results

- Runtime max **4**, min **0**; the probe ended with **4** connections, never exceeding four.
- **8 concurrent read probes** completed in **370 ms**, with no connection/query failures or timeouts.
- Empty-acquire count **8** reflects initial connection acquisition/queuing under this bounded probe; it is not a failed query count.
- No query-mode/JSON binding compatibility problems observed with Neon's pooled endpoint.
- This short probe does not establish production capacity, long-idle/cold-start behaviour or memory on the VPS.

## Configuration changes and remaining production gate

Two safe local-file corrections were made after checking the supplied non-secret target metadata:

1. `NEON_EXPECTED_ENDPOINT` was normalized from the full pooled hostname to the direct endpoint ID.
2. `MIGRATION_DATABASE_URL` was switched from the pooler to the matching direct Neon hostname, retaining its supplied credentials and database.

**Production hardening required:** the supplied runtime URL currently uses an owner-capable role (`neondb_owner`), with CREATE privilege and table ownership. Staging passed with that role, but production should use a separate least-privilege runtime role. Keep the owner/migration credential separate; use the prepared `deploy/neon-runtime-grants.sql` after reviewing actual role names and effective privileges. No roles, grants or passwords were automatically changed during validation.

## Retained, identifiable fixtures

- Run ID: `20261031T152308Z-cd4e2fa7`.
- Validation accounts: IDs **1** and **2**, with names beginning `MYFIT NEON STAGING VALIDATION` and emails under the `myfit-neon-validation-...@example.test` prefix.
- Retained scope: two test users, one private validation food (ID **23**), one food log (ID **1**), one weight entry, one recommendation and three sessions.
- Normal expired-session housekeeping was checked in advance and would not remove any existing sessions. No logout/delete endpoint or fixture cleanup was requested.
- Exact fixture emails, record IDs and ownership are recorded in `.run/neon-staging/neon-smoke-20261031T152308Z-cd4e2fa7.json`. Passwords/session tokens are absent from this manifest.
- **Cleanup requires explicit confirmation.** No cleanup SQL, DROP or reset was executed.

## Local PostgreSQL preservation

Before/after read-only fingerprints match exactly for the existing local `myfit-postgres` database: **schema, table contents and sequence values are unchanged**. The ordinary destructive-cleanup integration suite was not run against either database; offline tests explicitly unset `TEST_DATABASE_URL`. Local baseline/comparison files are retained privately under `.run/neon-staging/`.

## Wildcard certificate template

The local MyFit Nginx template now references the existing operator-verified files:

- `/etc/letsencrypt/live/edunexify-wildcard/fullchain.pem`
- `/etc/letsencrypt/live/edunexify-wildcard/privkey.pem`

Operator-reported coverage: `*.edunexify.co.in`; expiry: **January 14, 2027**. The updated template passed standalone local `nginx -t` using a test certificate mounted at the expected container path. The real certificate/private key, renewal configuration and VPS Nginx were not accessed or modified.

## Evidence

Private reports (ignored by Git, owner-readable only):

- `.run/neon-staging/neon-inspection.json`
- `.run/neon-staging/neon-schema-verification.json`
- `.run/neon-staging/neon-smoke-20261031T152308Z-cd4e2fa7.json`
- `.run/neon-staging/local-before.json`
- `.run/neon-staging/local-after.json`

Offline validator tests, full offline Go tests and Go vet passed. Certificate-template syntax validation passed. No deployment or destructive operation follows automatically.
