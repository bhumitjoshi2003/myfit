# MyFit production deployment preparation

**Prepared only. Nothing here authorizes executing the VPS/DNS/Neon change steps now.** No VPS connection, production migration, public deployment, DNS change or host Nginx edit was made during preparation. Existing development PostgreSQL and Compose files remain unchanged.

Edunexify availability takes precedence. This plan adds one bounded Go container and one exact-host Nginx vhost to the existing host Nginx. No production PostgreSQL, Node/Vite, extra Nginx container, host-network container, shared Docker network or application volume is needed. Neon supplies the separate database. Build off-host.

```text
Internet → existing host Nginx :443 (exact myfit.edunexify.co.in only)
             ├─ /, /assets/* → /var/www/myfit/current (React release files)
             └─ /api/*, /health, /livez → 127.0.0.1:18080 → myfit-prod API
                                                                  └─ TLS → MyFit-only Neon database
```

The local loopback and relay-startup harness defects are fixed; **native AMD64 capacity remains unqualified**; see [controlled rollout report and commands](CONTROLLED-ROLLOUT.md). No deployment is authorized.

## Files

| File | Purpose |
| --- | --- |
| `../Dockerfile` | Go and frontend build stages; scratch, nonroot API runtime and static export target |
| `compose.production.yml` | MyFit-only project, loopback publishing, resource and log limits; no DB/volumes |
| `production.env.example` | Runtime Neon URL and tunables; copy outside checkout with mode 0600 |
| `migration.env.example` | Separate direct-endpoint DDL credential for an explicit one-shot migration |
| `neon-runtime-grants.sql` | Dedicated runtime role grants, prepared for later review/execution |
| `nginx/myfit.edunexify.co.in.conf` | Exact-host HTTP/HTTPS vhost; unique maps/zones/logs |
| `nginx/myfit-proxy.conf` | API proxy, original paths and MyFit-only cookies |
| `nginx/myfit-security-headers.conf` | HTTPS headers scoped to MyFit |
| `nginx/myfit-http-bootstrap.conf.example` | Optional temporary certificate-only vhost, not simultaneously enabled |
| `nginx/myfit.logrotate.example` | Only if the host's existing wildcard log rotation does not cover these files |
| `scripts/build-release.sh` | Builds locally/CI; never pushes or deploys |
| `scripts/verify.py` | Certificate-validating HTTPS/static/API checks; read-only unless `--auth` requested |
| `scripts/monitor-memory.sh` | Read-only cgroup v2 memory/OOM sampling for a MyFit container |
| `scripts/local-container-smoke.py` | Local development DB/disposable-schema smoke and resource test; never Neon/VPS |

## 1. Read-only preflight after authorization

First record Edunexify's existing health responses, traffic/error rates, memory, swap activity, CPU and disk headroom during a representative busy period. Obtain its actual health URLs from the existing operator; do not guess them or enumerate other projects.

Inspect `free -m`, `vmstat`, `uptime`, `df -h`, `uname -m`, `docker ps`, `docker stats --no-stream`, `docker network ls`, `docker compose version`, `ss -ltnp`, and the existing Nginx version/configuration using `nginx -V` / `nginx -T` / `nginx -t`. Treat Nginx output as potentially sensitive. Confirm:

- Available RAM supports the 96 MiB cap plus Docker/network overhead and a substantial Edunexify safety margin; **1 GB total is not proof of headroom**. If the host is under pressure or swapping, do not deploy. Add capacity or use another host instead of shrinking Edunexify or changing its limits.
- Port **18080** is free, the `myfit-prod` project/network names are unused, and no exact `myfit.edunexify.co.in` server block exists. If a port/name conflicts, pick a new MyFit-only value and update its proxy; never stop the occupying service.
- Determine architecture; do not assume x86. The image must match `linux/amd64` or `linux/arm64` as appropriate.
- Determine whether host Nginx is systemd-managed, where its `http{}` includes and pid file live, how TLS is renewed, and whether an existing wildcard certificate's SANs cover MyFit. Exact hostnames normally take priority over wildcard names, but inspect actual regex/default/catch-all blocks and TLS options before adding anything.
- Check existing global rate limits, maps, header inheritance, auth policies, connection limits and log rotation. The supplied config is tested standalone, not against the unknown host configuration. Resolve incompatibilities in the new MyFit file, never by changing Edunexify/global policy for convenience.
- Do not restart/reconfigure Docker, change daemon settings, run broad prune/down commands, alter existing Compose projects, reset a volume, or change global Nginx settings. Nothing in this plan requires those operations.

Take a secure, operator-approved backup of the existing Nginx configuration and baseline service state before the eventual change. Keep secrets out of tickets/logs. Stop immediately if Edunexify indicators deteriorate during any later step.

## 2. Build an immutable release OFF the VPS

Example commands for the authorized workstation/CI build stage (no deployment):

```sh
export MYFIT_RELEASE=REPLACE_WITH_UNIQUE_COMMIT_OR_RELEASE_ID
export TARGET_PLATFORM=linux/amd64   # only after confirming the VPS architecture
export MYFIT_IMAGE_REPOSITORY=registry.example.com/myfit-api
./deploy/scripts/build-release.sh
```

Both image and frontend must come from the same clean source revision. Keep `image-id.txt`, `platform.txt`, `assets.sha256`, the image tag and frontend tarball. For a real release, pin/review builder-image digests and scan the built image/dependencies. The Dockerfile builds against the project's Go 1.26.10 toolchain; it changes no host Go installation. Time-zone data is embedded and CA certificates are included for verified Neon TLS.

The script deliberately does not push. A later authorized artifact-transfer step may push the image, record the returned **repository digest**, and transfer only deployment files/static artifacts to the MyFit directories. Prefer `MYFIT_IMAGE=repository@sha256:...` in production. Never build Go/npm on the 1 GB VPS while school services are live. Do not copy `.env`, local database data, `.run`, test results or the whole repository into the web root.

## Neon staging validation

The current Neon staging request authorizes inspection, existing MyFit Up migrations on the confirmed dedicated target, and scoped API validation. The authorized validation has passed; see [NEON-STAGING-RESULTS.md](NEON-STAGING-RESULTS.md). Follow [NEON-STAGING.md](NEON-STAGING.md) for private credential-file setup, independent endpoint/database confirmation, read-only inspection, staged commands and retained fixture manifests. This authorization does not extend to DigitalOcean, DNS, host Nginx or destructive cleanup. Earlier "prepared only" notes below describe the original production handoff; the separately authorized Neon staging workflow is the current path.

## Least-privilege role validation

Completed: `myfit_runtime` now has only reviewed application grants, with `neondb_owner` retained for migrations. See [NEON-ROLE-SECURITY.md](NEON-ROLE-SECURITY.md) for actual DDL denial, owner-Goose and application test results. Private local production/migration env files are prepared; this does not authorize copying them to the VPS or deploying. Future object grants require explicit review, not blanket default privileges.

## 3. Prepare a separate Neon database/roles (production still gated)

Create/use a dedicated MyFit Neon project/database or independently isolated database, not Edunexify's database. Runtime and migration credentials should be separate. Use a **pooled** endpoint for `DATABASE_URL`, a **direct** endpoint for `MIGRATION_DATABASE_URL`, the same MyFit database name, URL-encoded passwords and **`sslmode=verify-full`**. Do not log connection strings. The runtime image uses public trusted CA roots and hostname verification; no TLS disabling fallback exists in production.

Obtain roles through Neon's supported controls. The migration role owns the MyFit schema; the runtime role should not own objects, inherit the migration role or have schema-creation rights. After migration, review/apply the prepared grants file only to this dedicated database, using psql variables for its actual role names. Runtime gets table DML, sequence use and read-only migration-version access; no DDL credential is injected into the API container. Re-apply/review grants when future migrations introduce new objects. Validate the effective runtime role's privileges on a Neon staging branch first.

Prepare mode-0600 files outside the web root:

```text
/opt/myfit/shared/production.env   # Compose interpolation syntax; quoted URLs supported
/opt/myfit/shared/migration.env    # Docker --env-file syntax: no surrounding quotes
```

Do not source either as a shell script. Do not use `docker compose config` without `--quiet` on real credentials; rendered output contains secrets. Environment variables remain visible to root/Docker administrators, as expected for this design. Keep migration credentials out of the long-running service and remove/restrict the separate file after use.

### Migrations are NEVER automatic in production

`APP_ENV=production` fails startup unless cookies are secure, the origin is HTTPS, a database URL uses verified TLS, and `AUTO_MIGRATE=false`. Normal startup reads the schema version and fails if it differs from this release's latest migration. `/health` does not run migrations.

Current schema version: **5**. The one-shot command below is a *future operator action*, not a command executed during preparation:

```sh
# Only AFTER validating the precise Neon project/database, taking a MyFit restore
# point/branch and explicitly approving its schema changes and rollback strategy:
docker run --rm --read-only --cap-drop ALL --security-opt no-new-privileges:true \
  --memory 96m --memory-swap 96m --cpus 0.50 --pids-limit 64 \
  --env-file /opt/myfit/shared/migration.env \
  REPLACE_WITH_APPROVED_IMMUTABLE_MYFIT_IMAGE migrate
```

The CLI requires a direct endpoint matching the runtime URL's Neon endpoint and database. It uses at most one SQL connection, a two-minute timeout and Goose's transactional SQL migrations. It never performs a reset/down migration. Run this against the separate Neon staging branch first, not production. Provision and test runtime grants afterwards. A database restore is a MyFit-only last resort requiring explicit approval, never an automatic rollback step.

## 4. Stage the container without exposing a public route

Suggested MyFit-only layout:

```text
/opt/myfit/releases/RELEASE_ID/deploy/      # immutable Compose/config/runbook snapshot
/opt/myfit/shared/production.env
/var/www/myfit/releases/RELEASE_ID/         # frontend-only index.html and assets/
/var/www/myfit/current -> releases/RELEASE_ID
```

Use a dedicated directory owner and readable static assets for the existing Nginx worker user. Never change ownership/permissions on Edunexify directories or `/var/www` as a whole. Keep prior MyFit release directories/image digests and secure env snapshots for rollback. Validate the frontend checksum manifest before activation. Carry forward retained hashed `assets/` from the previous MyFit release into the new release without overwriting any new hashed file so old browser tabs can still fetch them; retain the previous release for rollback.

Use this exact Compose file alone, never the development file or a merge:

```sh
# Prepared future staging command; not executed on the VPS now.
docker compose -p myfit-prod --env-file /opt/myfit/shared/production.env \
  -f /opt/myfit/releases/RELEASE_ID/deploy/compose.production.yml config --quiet

docker compose -p myfit-prod --env-file /opt/myfit/shared/production.env \
  -f /opt/myfit/releases/RELEASE_ID/deploy/compose.production.yml up -d --no-deps api
```

No `container_name` is hard-coded. Docker creates only the unique MyFit bridge and API container; runtime DB traffic must be able to leave the bridge. Only `127.0.0.1:18080` is published. Confirm this with `docker inspect`/`ss` and an external port check later; do not change the shared firewall to expose it. Docker versions older than 28 have documented caveats around localhost publishing; assess the installed daemon and host firewall before exposure, without upgrading the shared daemon as part of this change.

Readiness: `curl --fail http://127.0.0.1:18080/health`. Liveness: `/livez` (no database ping). Both return minimal JSON. Periodic database readiness probes can keep Neon compute active; review that monitoring behaviour on staging. Health checks are bounded; Neon cold starts can cause transient readiness failures. `on-failure:3` avoids an endless crash loop, but Docker does not automatically restart an unhealthy still-running container. A finite restart policy also means operators must confirm restart after VPS/Docker reboot; add monitoring rather than assuming recovery. No unattended deploy/restart controller is introduced.

## 5. Validate resource use BEFORE approving capacity

Initial container cap is **96 MiB** (`MYFIT_MEMORY_LIMIT=96m`), with **64 MiB Go soft limit** (`MYFIT_GOMEMLIMIT=64MiB`), one Go scheduler CPU, 0.50 CPU quota, 64 PIDs, no additional swap and rotating 10 MB Docker logs. The scratch runtime is nonroot, read-only, drops all capabilities, and has no Docker socket or application volume. Tune only MyFit, not school services.

Pool: max 4, min 0, idle 5 minutes, lifetime 30 minutes with jitter, connection timeout 10 seconds. Query mode avoids named/prepared statement cache assumptions with transaction pooling. SQL request contexts are bounded to ten seconds. A migration command uses a separate one-connection direct connection. These are starting limits, not throughput promises.

`GOMEMLIMIT` is **soft** and does not cap all RSS/cgroup memory; Docker's cgroup cap is the hard boundary. It can cause OOM kills if insufficient. When raising the cap, raise it only after confirming total VPS headroom; retain Go/runtime/stack/TLS headroom. Swapping or stealing RAM from Edunexify is not an acceptable fix.

Use the read-only sampler with the **specific MyFit container ID** while exercising real TLS/Neon and representative auth/diary/onboarding load on staging:

```sh
./deploy/scripts/monitor-memory.sh MYFIT_CONTAINER_ID 300 > myfit-memory.csv
```

Inspect `memory.peak`, `memory.events` (`oom`/`oom_kill`), restart count, CPU, latency, pool contention and host/Edunexify metrics. The sampler requires cgroup v2; on v1 use Docker stats plus host cgroup counters explicitly. Include startup, two simultaneous bcrypt logins, burst auth rejection, maximum accepted bodies, healthy-idle, Neon cold start/reconnect, normal logging and a sustained soak. Keep at least ~25% measured headroom below the hard cap as a provisional gate, zero OOM/restarts/timeouts and no host/school impact; this rule is not proof of safety under untested load. Stop staging if school metrics worsen. Do not run a stress test against the shared VPS without a separate safe traffic budget.

Local container test results are recorded in `VALIDATION.md`. Local dev-DB results **do not validate the VPS, Neon TLS latency, architecture or real peak traffic**. If safe capacity cannot be demonstrated, keep MyFit off this VPS. The app can fit a small container while the shared host still lacks capacity.

## 6. Prepare HTTPS and an exact-host Nginx vhost

Do not change the existing wildcard vhost or deploy another Nginx server. Copy the two new MyFit-only snippets to `/etc/nginx/snippets/` and the new vhost to a distinct site file only after review. This vhost belongs in the existing `http{}` includes because its unique `map` and limit zones are at http context. Keep hostname **exactly** `myfit.edunexify.co.in`; never use a default-server flag or wildcard. Preserve include order and the existing **implicit** default server too: install the new enabled file after the existing server files (for example `zz-myfit.edunexify.co.in.conf`) and verify the resolved `nginx -T` order before reload. If this new file became the first `listen 80`/`listen 443` server, it could change default routing even without a `default_server` flag; do not activate in that state. If `MYFIT_API_PORT` changes, update only `myfit-proxy.conf` to match.

The operator has verified the existing certificate covers `*.edunexify.co.in` and expires on **January 14, 2027**. The MyFit template now references `/etc/letsencrypt/live/edunexify-wildcard/fullchain.pem` and `/etc/letsencrypt/live/edunexify-wildcard/privkey.pem`. Reuse these files read-only; recheck validity and renewal at deployment time. No certificate issuance or replacement is needed for this hostname. Never replace existing certificate files, permissions or renewal configuration. Do not invoke `certbot --nginx` against the shared service. If a separate HTTP-01 certificate is required, use the optional MyFit-only bootstrap file and `certbot certonly --webroot` with `/var/www/myfit/acme` after DNS/port reachability is approved. Enable either the bootstrap or final MyFit HTTP block, not both. Certificate renewal and any graceful reload hook need the same shared-service review. No DNS/certificate action is included in this preparation.

Only after cert/static files/container readiness are verified: `sudo nginx -t`. If it fails, do not reload, restart or try global fixes. Remove/disable only the new MyFit file if needed. If it succeeds and the change window is approved, use the host's verified **graceful reload** mechanism, typically `sudo systemctl reload nginx`, then verify schools immediately. Never restart Nginx. Even a graceful reload is a shared-service action with nonzero risk; it requires the operator's approval and a prepared rollback.

Headers include a MyFit-only CSP, nosniff, frame denial, Referrer/Permissions policies and short initial HSTS (one day, no `includeSubDomains` or preload). The static CSP permits inline CSS for existing React progress styles, not inline JS. API responses are no-store; the entry document revalidates and hashed assets cache immutably. Locations with their own `add_header` repeat the security snippet to avoid inheritance loss on older Nginx versions. Edunexify's headers/logs/routes are not changed. Check existing logrotate wildcard coverage before using the optional example; never add duplicate rules. Match existing ownership and rotate without restarting the service.

### Cookie isolation

Production uses **`__Host-myfit_session`**, Secure, HttpOnly, SameSite=Lax, Path=/ and **no Domain**. Login/logout never set or clear Edunexify cookie names. The prefix prevents a sibling site from setting a parent-domain version of this cookie. MyFit's production API does not read the old development `myfit_session` cookie.

A parent-domain Edunexify cookie may still be sent by the browser to its subdomains; MyFit cannot change that without changing Edunexify. The new Nginx map forwards **only** MyFit's session cookie to the Go service and discards other cookies at that boundary. The unique name/host-only scope avoids collisions. There is no wildcard CORS: mutations require the exact `https://myfit.edunexify.co.in` Origin, JSON and the existing session-bound CSRF token. Forwarded headers are not trusted by the API; Nginx has a separate MyFit-only per-client auth limiter. The API's existing 30 attempts/15 minutes per peer is an additional shared-proxy backstop, so it can limit aggregate logins on a single proxy; capacity-plan this for growth before increasing it.

## 7. Verify before and after the approved DNS/routing change

Test TLS/SNI and the proposed host route before publishing a DNS change:

```sh
python3 deploy/scripts/verify.py --connect-ip VPS_IP
```

This keeps the real hostname for TLS verification and Host, and does not bypass certificates or change DNS. It checks health, the React shell, security headers, anonymous protection, origin rejection and missing assets. After public routing is intentionally enabled, repeat without `--connect-ip`. Check relevant Edunexify school endpoints/health, error rate and memory against baseline immediately, not only MyFit.

Optional auth verification uses a dedicated MyFit verification account via environment variables, not CLI arguments:

```sh
# Obtain/set credentials securely; do not put values in shell history or logs.
python3 deploy/scripts/verify.py --auth
```

This creates and revokes only a login session; it checks Secure/HttpOnly/host-only cookie attributes, `/api/me`, CSRF rejection and logout revocation, without changing food/goals/weight data. A separate deliberate browser check covers registration, onboarding, daily logging/editing and persistence. Test cookie coexistence in a browser with an existing Edunexify login: MyFit login/logout must not remove it. No live Edunexify account was accessed during local preparation.

## 8. Rollback and stop conditions

Record **before activation**: previous MyFit immutable image digest, schema version, exact env/config snapshot, static symlink target and MyFit vhost/snippet files. Retain the corresponding static assets. Do not rely on `latest` tags.

- **First deployment / school impact:** disable only the newly added MyFit vhost/symlink in Nginx's include location, restore only prior MyFit snippets if any, run `nginx -t` and perform the approved graceful reload. Stop only MyFit's `api` service using the explicit `myfit-prod` project/file. Leave Neon and all Edunexify containers, networks, volumes and route files untouched. The prior wildcard route may resume matching MyFit; check existing catch-all behaviour during preflight so this is understood. Do not edit the wildcard to implement rollback.
- **Normal MyFit rollback, same schema:** use the previous MyFit release's Compose file and secure env snapshot with its previous immutable image digest. `up -d --no-deps api` replaces only MyFit. Restore `/var/www/myfit/current` atomically to the prior MyFit static release (`ln -s ... current.next` then `mv -Tf current.next current` on the Linux host). Keep previous assets accessible. Verify internal readiness, HTTPS/auth and school baselines. This may briefly interrupt MyFit; it does not require restarting Nginx if the vhost is unchanged.
- **After a schema change:** runtime requires an exact schema-version match. An old image cannot automatically roll back across a newer schema. Before approving any future schema change, test a schema-compatible rollback image/forward-fix and a Neon restore point on staging. If no tested application rollback exists, do not promote that release. Never run `goose down`, drop tables, restore/reset Neon or delete a volume automatically; those operations can discard new MyFit data and need separate approval.
- If MyFit cannot start, becomes unhealthy/OOM-killed, runs out of pool capacity, or increases host pressure/school error rates, stop only MyFit and investigate. Do not raise its memory/cpu budget by reducing Edunexify resources.

Example service-only stop (future authorized operation):

```sh
docker compose -p myfit-prod --env-file /opt/myfit/shared/production.env \
  -f /opt/myfit/releases/RELEASE_ID/deploy/compose.production.yml stop api
```

No plan step uses `docker compose down -v`, `docker system prune`, a Docker/Nginx restart, production DB reset, edits to unrelated projects or unattended public deployment.

## References and remaining checks

- [Go GC guide](https://go.dev/doc/gc-guide): GOMEMLIMIT is a soft limit.
- [Docker Compose services](https://docs.docker.com/reference/compose-file/services/): memory, swap, security and restart settings.
- [Docker port publishing](https://docs.docker.com/engine/network/port-publishing/): loopback binding and older-version caveats.
- [Neon connection pooling](https://neon.com/docs/connect/connection-pooling): pooled and direct endpoints.
- [Nginx graceful control](https://nginx.org/en/docs/control.html), [headers](https://nginx.org/en/docs/http/ngx_http_headers_module.html), and [proxy module](https://nginx.org/en/docs/http/ngx_http_proxy_module.html).
- [Cookie prefixes](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie#cookie_prefixes).

Unverified until authorized: actual VPS architecture/resources/Docker and Nginx versions, existing vhost integration and certificate renewal rules, DNS state, live Neon TLS/permissions/cold starts, production memory/latency, current school baselines and real Edunexify cookie coexistence. These are promotion gates, not reasons to alter school infrastructure during preparation.
