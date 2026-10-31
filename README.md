# MyFit

A local, multi-user nutrition and weight tracker built with React, TypeScript, Vite, Tailwind CSS, Go and PostgreSQL 17.

## Start and stop

From this directory, with Docker running:

```sh
./scripts/start.sh
```

Open **http://localhost:5173**, create your own account and complete the personalized onboarding wizard. Existing accounts with incomplete profiles also complete onboarding; food and weight history is preserved. Use this exact hostname: cookie and origin protection are configured for localhost. There is no default account or shared weight value. The 54.5 kg value appears only as an input example until you record it.

```sh
./scripts/stop.sh          # Stops the API and frontend started by start.sh
docker compose stop db   # Optional: stops PostgreSQL and preserves the volume
```

The existing `docker-compose.yml` and database volume are preserved. Startup runs additive Goose migrations automatically. Logs and process IDs live in the ignored `.run/` directory. Do not run `docker compose down -v`; it deletes your database volume.

Prerequisites: Node 22+, npm, Docker Compose, and Go with automatic toolchain downloads enabled. `backend/go.mod` requests Go 1.26.10; the installed Go 1.23.3 automatically downloads a separate toolchain, without changing system installations. First startup requires internet access for dependencies. `npm ci --prefix frontend` installs the locked frontend dependencies.

Optional configuration: copy `.env.example` to `.env` and edit it. `start.sh` loads this file. The helper scripts assume the default ports 8080 and 5173. For custom ports, use the manual commands below and update the Vite proxy/origin accordingly. Never commit `.env`.

Manual foreground development, in separate terminals:

```sh
docker compose up -d db
cd backend
go run .
```

```sh
cd frontend
npm ci
npm run dev
```

Use Ctrl-C to stop each foreground server. Manually export environment variables when using these commands; Go does not automatically load `.env`.

## Daily use

- **Dashboard:** selected day's calories, remaining calories, macros, fibre, meals and latest weight. Use date arrows or choose any date.
- **Food Diary:** search, choose a serving and quantity, log into a meal, edit quantity/date/meal, delete or repeat a previous entry. All nutrition values and daily/meal totals are calculated on the API.
- **Food Library:** shared reference foods and your private foods; search, favourites, recent foods, private copies and editable packaging values. The plus button opens the selected food directly.
- **Weight Progress:** dated check-ins, chronological chart and history. Saving the same day updates that day's weight.
- **Profile / Goals:** your name, IANA time zone, calories and four macro/fibre targets. Active goals are explicitly accepted recommendations or custom targets. Review recalculated suggestions after changing your measurements or activity. Weight has no default.

Registration signs you in. Passwords must be 10–72 UTF-8 bytes. An account's diary dates follow its time zone; changing time zone does not move historical dated entries. The chart uses actual elapsed dates. There is no external nutrition API dependency.

## Personalized onboarding

See [ONBOARDING.md](ONBOARDING.md) for the versioned recommendation formulas, assumptions, supported population, additive migration, APIs and test coverage. New accounts no longer receive universal calorie or protein defaults. Recommendations are starting estimates; accepting or customizing them is always explicit.

## Nutrition provenance

The starter catalogue contains 22 common foods and ingredients. Generic food values primarily use **USDA SR Legacy** entries identified in each food's source field; see [FoodData Central datasets](https://fdc.nal.usda.gov/download-datasets/). Paneer uses **ICMR-NIN IFCT 2017**, food L003, table 1, printed page 19, with 1278 kJ converted to kcal; see [IFCT 2017](https://www.nin.res.in/ebooks/IFCT2017.pdf). Paneer fibre is unreported and shown as zero. Roti is explicitly a flour-and-water recipe estimate. Plain lentils represent dal without oil/tadka; yogurt is a curd proxy. All starter entries are marked approximate. Labels and recipes can differ substantially.

All values reference 100 g edible portion. A medium banana is 118 g; two provide 210.04 kcal, 2.5724 g protein, 53.9024 g carbohydrate, 0.7788 g fat and 6.136 g fibre. A 250 ml milk glass uses an approximate density of 1.03 g/ml, rounded to 258 g. Serving weights are estimates unless weighed. No product-specific branded nutrition is invented. Make a private copy and enter your packaging label or weighed serving when available. USDA carbohydrate includes fibre; IFCT's paneer entry uses available carbohydrate. These references should not be interpreted as analytically identical.

## Architecture and security

- `backend/main.go`: configuration, migrations, HTTP server, graceful shutdown.
- `backend/internal/api`: standard-library routing, request validation, session/CSRF controls and orchestration.
- `backend/internal/domain`: typed domain records, nutrition validation, scaling and aggregation.
- `backend/internal/store`: parameterized pgx queries and transactional persistence.
- `database/migrations`: Goose schema and seed migrations. Query results use typed scans; sqlc was not necessary for this small query set.
- `frontend/src`: responsive app and cookie-based API client.

Passwords use bcrypt cost 12. Random session credentials are stored only as SHA-256 hashes in PostgreSQL. HttpOnly, SameSite=Lax cookies expire after seven days and are revoked on logout. Mutation requests require an exact trusted Origin and JSON content type; authenticated mutations also require a session-bound CSRF token held in memory. No tokens are stored in localStorage. Login/registration have a basic per-IP in-memory rate limit; restarting resets it. Forwarded IP headers are deliberately not trusted. All private queries include the authenticated user's ID. Database constraints enforce relationships and numeric bounds. Updates to food definitions and diary entries use optimistic versions; stale changes return 409. Serving replacement is transactional.

Each log stores its food name, brand, serving weight, per-100-g nutrition reference and computed nutrition snapshot. Later edits scale the original snapshot; changing a catalogue item never rewrites history. Repeating an old entry uses its snapshot too. To use a changed reference, delete the old entry and add the current food again.

This is a local MVP. Public deployment, HTTPS/reverse-proxy configuration, email verification, password reset, backups, and multi-instance rate limiting are not included. Before hosting, configure HTTPS, `COOKIE_SECURE=true`, an exact `APP_ORIGIN`, private database credentials/TLS and a production static frontend server. Vite is a development server. No external deployment was performed.

## Production deployment preparation

[deploy/README.md](deploy/README.md) contains the isolated DigitalOcean/Neon deployment plan, production Docker/Compose and Nginx files, resource validation, verification and rollback steps. This is preparation only: no VPS, DNS or production database changes are authorized or performed. Production startup never auto-migrates; it requires an explicitly approved migration and a matching schema. The included local start commands above continue to use the development database.

## Verification

```sh
cd backend
go test ./...             # Unit tests; integration test skips without TEST_DATABASE_URL
TEST_DATABASE_URL='postgres://myfit:myfit_local_dev@localhost:5433/myfit?sslmode=disable' go test -race ./...
go vet ./...
```

The PostgreSQL integration test creates an isolated temporary schema, runs the actual Goose migrations, and drops only that schema afterwards. It verifies registration, login/logout, authorization, session revocation, CSRF/origin checks, serving calculations, editing, stale version rejection, private food access, nutrition snapshots, repeat entries, weight persistence and time-zone defaults.

```sh
npm run build --prefix frontend
cd frontend
npx playwright install chromium  # Once, for browser tests
npm run test:e2e                 # With the default frontend and API running
```

Playwright runs the daily journey at desktop and mobile sizes, checks all five nutrients, edits two bananas to one, records weight, creates/favourites private food, changes goals, signs out/in and checks persistence, cookies, localStorage and horizontal overflow. Test accounts are recognizable as `myfit-e2e-…@example.test`; they are test-only records, never your own account. Browser screenshots and traces go in ignored test result directories. Browser coverage uses Chromium, including mobile emulation; Safari and Firefox have not been verified.
