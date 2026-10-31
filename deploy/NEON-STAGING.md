# Neon staging validation

**Security update:** the separate runtime role is now created and validated. The earlier owner-capable-runtime warning below is historical and resolved; see [NEON-ROLE-SECURITY.md](NEON-ROLE-SECURITY.md).

**Latest execution:** passed on October 31, 2026. See [NEON-STAGING-RESULTS.md](NEON-STAGING-RESULTS.md) for observed results and the remaining least-privilege runtime-role requirement. Credentials and fixture ownership manifests remain in ignored private local files.

The current request authorizes **Neon staging inspection, MyFit Up migrations and scoped application tests** once the dedicated MyFit target is confirmed. It does not authorize a VPS deployment, DNS/Nginx activation, Edunexify access, a database reset, down migrations or cleanup/deletion of test data.

## Supply credentials privately

A local file has been created at `deploy/neon-staging.env`, with mode **0600**, ignored by Git and the Docker build context. Edit it in a private editor. Do not paste its contents into chat, a terminal command, logs or a ticket. The validator reads the file directly; it does not shell-source it, use connection URLs as process arguments, or fall back to local PostgreSQL.

In the Neon Console, choose the **dedicated MyFit project** and intended staging branch/database. In its connection dialog:

1. Enable pooling and copy the URL into `DATABASE_URL`.
2. Disable pooling and copy the direct URL into `MIGRATION_DATABASE_URL`. A migration-owner role may differ from the runtime role, but endpoint/branch/database must match.
3. Use `sslmode=verify-full` in **both** URLs. Keep `channel_binding=require` if the Neon URL includes it; this pgx version supports it. URL-encode password characters. Paste only the URL, not a `psql` command. Raw values or balanced surrounding quotes are accepted; no shell expansion occurs.
4. Independently check `NEON_EXPECTED_ENDPOINT`: the direct hostname's first component, such as `ep-example-123456`, **without** `-pooler` or the regional hostname suffix.
5. Set `NEON_EXPECTED_DATABASE` to the actual database name, often `neondb`. It does not need to be literally named `myfit` if it belongs to the dedicated MyFit project.
6. Set `CONFIRM_DEDICATED_MYFIT=true` only after confirming the selected project/branch is MyFit, not Edunexify.
7. Save and reply **“file ready”**. Do not send the values.

The runtime URL must use `ep-…-pooler.…neon.tech`; the direct URL must use the same `ep-….…neon.tech` compute and database. The validator rejects localhost, non-Neon hosts, unrelated endpoints/databases, insecure TLS, host/search-path overrides, symlinked files and group/world-readable credential files. Additional connection-string options beyond `sslmode`, `connect_timeout` and `channel_binding` are refused for validation. Both client certificate-chain validation and hostname validation must succeed; TLS is never bypassed.

## Validation sequence

The validator is run from the existing Go module, with only the local env-file path appearing in its arguments:

```sh
cd backend
go run ./cmd/neonvalidate --phase inspect
go run ./cmd/neonvalidate --phase migrate
go run ./cmd/neonvalidate --phase verify
go run ./cmd/neonvalidate --phase smoke
```

These commands are staged deliberately: inspect output is reviewed before invoking migration. They are not an automatic all-in-one deployment script. Read-only inspection reports endpoint ID, database, role, server version, negotiated direct/pooled TLS versions, relations and existing migration version, with no credentials. SQL inspection uses a read-only transaction.

Migration requires an inspection less than one hour old with matching endpoint, database and migration-file digest. It re-inspects the target immediately before the first DDL. Initial migrations are permitted only when no user relations or prior Goose version exist and the migration role can create objects in `public`. Unexpected existing objects cause a stop; they are never dropped or reset. A partially applied database is reported for review rather than automatically rewritten. Existing migrations 003/004 correct only their shared catalogue seed entries; no historical user logs/weights are rewritten. Only Goose **Up** migrations run, with a two-minute migration timeout. The original migration files are not changed.

After migration, verification checks nine tables, 20 valid indexes, nine validated foreign-key relationships, column/default/constraint metadata, Goose version 5, 22 shared seed foods, 68 shared servings, the medium banana serving and paneer/roti reference corrections. The runtime path uses the application's production configuration and query mode, with max four pooled connections. A bounded eight-query probe checks pool contention and errors. If the supplied runtime role owns tables or has schema creation rights, that is reported as a production hardening requirement; the utility does not silently change roles or grants.

Application tests invoke the actual HTTP handlers against the Neon-backed store without opening a network listener or touching Nginx/DNS/VPS. They exercise registration, secure sessions, onboarding and explicit goal acceptance, five-nutrient food calculations, quantity corrections, optimistic locking, private food creation, weight persistence, fresh login, CSRF and cross-user diary/weight/profile/private-food isolation.

All accounts use names beginning **MYFIT NEON STAGING VALIDATION** and unique emails `myfit-neon-validation-<run-id>-a/b@example.test`. The private food is also labelled as a staging fixture. A private audit manifest records emails and record IDs before/after writes, never passwords or session tokens. **Fixtures are retained**. No test cleanup, DELETE endpoint, reset, down migration or table/schema drop is invoked. Normal registration/login performs the app's expired-session sweep; the smoke phase refuses to start if any session is expired or could expire during the bounded validation window, so it will not remove existing sessions. No logout or prior-session revocation is requested by this utility.

Do not run the ordinary PostgreSQL integration suite against Neon with `TEST_DATABASE_URL`: that suite creates and drops temporary schemas. The dedicated validator above deliberately avoids those operations because cleanup requires separate confirmation.

## Reports and local-database comparison

Reports go to ignored mode-0700 `.run/neon-staging/`, with mode-0600 files:

- `neon-inspection.json`: target identity, TLS and pre-migration inventory.
- `neon-schema-verification.json`: schema/index/FK/seed verification and pool results.
- `neon-smoke-<run-id>.json`: scoped fixture manifest, completion status and tested behaviours.
- `local-before.json` and `local-after.json`: read-only local database schema, table-content and sequence fingerprints. No row values or passwords are printed.

Capture/compare with:

```sh
python3 deploy/scripts/local-db-fingerprint.py --output .run/neon-staging/local-after.json --compare .run/neon-staging/local-before.json
```

The local comparison uses the existing **local** `myfit-postgres` container and read-only psql/pg_dump commands. No local migrations, tests, writes, resets or service restarts are needed. Matching fingerprints demonstrate that local MyFit data/schema/sequences were unchanged during validation. If another local app session writes concurrently, report the discrepancy and investigate rather than claiming the validator caused it or modifying local data to hide it.

Connection and SQL errors are printed only as safe stage descriptions/SQLSTATEs. Raw driver messages, connection strings, passwords, CSRF/session credentials and API response bodies are suppressed. No production deployment follows automatically.

## Existing wildcard certificate

The operator verified `*.edunexify.co.in` coverage and an expiry of **January 14, 2027**. The local MyFit Nginx template now references:

```text
/etc/letsencrypt/live/edunexify-wildcard/fullchain.pem
/etc/letsencrypt/live/edunexify-wildcard/privkey.pem
```

Only the template and runbook are updated. The existing certificate/private key, renewal process and VPS Nginx configuration are not accessed or changed. Recheck certificate validity at the eventual deployment date; the old HTTP certificate bootstrap example is unnecessary for this verified wildcard path.
