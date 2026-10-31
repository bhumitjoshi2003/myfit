# Controlled rollout: BLOCKED — do not deploy

Release `20261031T161631Z-amd64-b968e81d5b72` was built locally for `linux/amd64`, including production React assets. No registry push, VPS access, DNS changes or live Nginx changes occurred. Existing Edunexify services and the local PostgreSQL container were not modified.

Immutable local repository reference:

`myfit-api@sha256:a712b40445c47d466f0bb32859608478caf59d82d63b56b1ffac0506d2edbe07`

Manifest digest is the digest above. Image configuration digest is `sha256:dd6ebf67cf7b7e23f57ef03a6351de09ee2f232d6c5ec81e99869def0da8e74b`. These are different identifiers. Docker Desktop's containerd image store reports the manifest digest as the local image ID. Archives, checksums, build metadata, source manifest and reports are in `deploy/artifacts/20261031T161631Z-amd64-b968e81d5b72/`. No remote registry contains this release. Use offline `docker save/load` transport and verify the reference after loading; never silently substitute a mutable tag.

## Results and stop condition

- API build and React TypeScript/Vite production build passed; final image inspection reported Linux/amd64 and nonroot UID/GID 65532.
- Private configuration inspection passed: pooled `myfit_runtime` runtime URL, direct `neondb_owner` migration URL, TLS `verify-full`. Compose forwards only the runtime URL; no migration credential reaches the service.
- Compose is MyFit-only, fixed to `127.0.0.1:18080:8080`, linux/amd64, 96 MiB memory and total memory-plus-swap, 0.5 CPU, pool maximum four, Go soft limit 64 MiB. No PostgreSQL service or application volumes.
- Nginx template references exactly `/etc/letsencrypt/live/edunexify-wildcard/fullchain.pem` and `/etc/letsencrypt/live/edunexify-wildcard/privkey.pem`; no certificates were changed.
- **Loopback root cause resolved:** the test API was attached only to a Docker `--internal` bridge, which did not provide the requested published host port in this local Docker setup. The application was internally healthy. Host `/health` and `/livez` both failed, including proxy-bypassed curl (exit 7, HTTP 000), while relay control returned 200. Attaching the same API container to a normal bridge made both host endpoints return 200. Its Neon hostname override and opaque TLS relay stayed unchanged. This isolates the original failure to harness Docker networking, not application health or Neon TLS. Production Compose already uses a normal bridge and needs no networking change.
- Fixed only the extended soak harness's missing normal-bridge attachment. Added a focused diagnostic script, `deploy/scripts/check-release-readiness.py`, which reproduces the original topology before checking the correction. No application, authentication, database or frontend regression tests were repeated; no fixtures, login sessions or database writes were created.
- **Focused resource investigation:** [restart and memory evidence](RESOURCE-INVESTIGATION.md) identifies a local relay-listener startup race. Waiting for relay readiness yielded zero restarts with the unchanged image. Historical exit 1 preceded all probes; it was not an OOM event. The original stderr was removed, but the same race reproduced one restart with a connection-refused fatal log.
- **87.7 MiB was total emulated-container peak, not native Go application memory.** Detailed local observations show Rosetta/probe overhead: emulated API RSS 58.38 MiB versus same-source native ARM64 RSS 14.70 MiB. File cache was only 1.88 MiB in the emulated idle observation. Native ARM results do not qualify native AMD64 capacity. The 96 MiB cap is unchanged; no representative workload was completed.
- **Remaining gates:** establish an explicit shared-host RAM reserve and perform a separately authorized, brief native AMD64 trial under the existing cap. Stop on school-service degradation, any API OOM/restart or insufficient native headroom. Scoped reconnection validation remains uncompleted. No unrelated tests need repeating solely for the harness fix.

The immutable application image and frontend archives remain byte-for-byte unchanged. No production environment credential changes, application changes or deployment occurred. Native trial is conditional on actual VPS headroom and separate authorization; no public routing changes are part of that proposed trial.

Runbook shell blocks were audited for shell syntax and accidental Markdown URL formatting; none contained Markdown links in command arguments. Health and HTTPS verification commands below use literal, quoted URLs and the verifier's actual `--url` option. The updated nonsecret deployment configuration is packaged separately as `deployment-config-resource-review.tar.gz`; original image/static archives remain unchanged.

## Proposed sequence after all gates pass and deployment is authorized

1. Recheck native architecture, port availability, available RAM, swap activity and Edunexify health/latency; preserve their baseline. Agree on abort thresholds. Build nothing on the VPS.
2. Transfer only this release's image/static archives, checksum files, MyFit deployment files and a mode-0600 **runtime-only** environment file. Keep migration-owner credentials on the workstation. Do not copy `production.env` wholesale: it contains a migration credential.
3. Load and validate the image; stage static files in a new MyFit release directory. Start only MyFit on loopback. Run native load/memory and Neon recovery validation before public activation. Stop MyFit immediately if Edunexify or host memory deteriorates.
4. Review existing Nginx include/default ordering, exact hostname conflicts, certificate validity and DNS separately. Stage only new MyFit snippets/vhost. After explicit authorization, `nginx -t` then graceful reload; immediately verify Edunexify baselines and MyFit HTTPS/auth/CSRF. Do not restart Nginx or Docker. DNS changes require separate authorization.
5. Retain the previous image, env and static directory. No migrations are required for this schema-v5 release; AUTO_MIGRATE remains false. Never roll back the database to undo an application deployment.

## Exact future VPS commands (NOT authorized now)

Run as the deployment operator with appropriate privileges, only after resolving blockers. These assume reviewed artifacts are already staged in the stated MyFit-only directories, standard Debian `sites-enabled` includes, and no conflicting MyFit files. Stop if any assumption fails. Set `MYFIT_IMAGE` in the runtime-only env file to the immutable reference above, never print the resolved Compose configuration.

```sh
set -eu
RELEASE=20261031T161631Z-amd64-b968e81d5b72
IMAGE=myfit-api@sha256:a712b40445c47d466f0bb32859608478caf59d82d63b56b1ffac0506d2edbe07
cd "/opt/myfit/releases/$RELEASE"
sha256sum -c resource-review.SHA256SUMS
tar -xzf deployment-config-resource-review.tar.gz
docker load --input myfit-api.tar
test "$(docker image inspect --format '{{.Os}}/{{.Architecture}}' "$IMAGE")" = linux/amd64
test "$(docker image inspect --format '{{.Config.User}}' "$IMAGE")" = 65532:65532
# /opt/myfit/runtime.env: mode 0600; runtime URL only, no MIGRATION_DATABASE_URL.
test "$(stat -c %a /opt/myfit/runtime.env)" = 600
! grep -q '^MIGRATION_DATABASE_URL=' /opt/myfit/runtime.env
docker compose --project-name myfit-prod --env-file /opt/myfit/runtime.env -f compose.production.yml config --quiet
install -d "/var/www/myfit/releases/$RELEASE"
tar -xzf frontend.tar.gz -C "/var/www/myfit/releases/$RELEASE"
(cd "/var/www/myfit/releases/$RELEASE" && sha256sum -c "/opt/myfit/releases/$RELEASE/assets.sha256")
# Capture previous MyFit image/env/current link securely before replacing any existing release.
docker compose --project-name myfit-prod --env-file /opt/myfit/runtime.env -f compose.production.yml up -d --no-deps --pull never api
curl --noproxy '*' --fail --max-time 15 'http://127.0.0.1:18080/health'
CID=$(docker compose --project-name myfit-prod --env-file /opt/myfit/runtime.env -f compose.production.yml ps -q api)
./monitor-memory.sh "$CID" 600
# Gate: native workload results, memory headroom and Edunexify baselines must pass here.
```

Only after that gate and explicit Nginx activation authorization:

```sh
# Preserve previous hashed assets for browsers with an older index.html on later upgrades.
ln -s "/var/www/myfit/releases/$RELEASE" /var/www/myfit/current.next
mv -Tf /var/www/myfit/current.next /var/www/myfit/current
# These destination paths must be unused or already managed exclusively by MyFit.
install -m 644 nginx/myfit-proxy.conf /etc/nginx/snippets/myfit-proxy.conf
install -m 644 nginx/myfit-security-headers.conf /etc/nginx/snippets/myfit-security-headers.conf
install -m 644 nginx/myfit.edunexify.co.in.conf /etc/nginx/sites-available/myfit.edunexify.co.in.conf
# Confirm this sorts after existing files and does not change any implicit default.
ln -s /etc/nginx/sites-available/myfit.edunexify.co.in.conf /etc/nginx/sites-enabled/zz-myfit.edunexify.co.in.conf
nginx -t
nginx -s reload
python3 verify.py --url 'https://myfit.edunexify.co.in'
# Immediately compare existing Edunexify health/latency with the saved baseline.
```

## Exact rollback (MyFit only)

For the **first** deployment, where there is no prior MyFit version, withdraw only the newly added vhost and stop only MyFit. This requires the same operational authorization as activation:

```sh
rm /etc/nginx/sites-enabled/zz-myfit.edunexify.co.in.conf
nginx -t
nginx -s reload
docker compose --project-name myfit-prod --env-file /opt/myfit/runtime.env -f "/opt/myfit/releases/$RELEASE/compose.production.yml" stop -t 20 api
```

If activation never occurred, skip the Nginx commands and stop MyFit only. Retain all Neon data and archives. For a later release, restore the privately saved prior runtime env and atomic static symlink, and run `up -d --no-deps --pull never api` with that prior release's Compose file; verify its immutable image and schema-v5 compatibility first. A concrete prior-release rollback cannot be filled in until a first production release exists. Never run `down -v`, broad restarts, prune, database resets or migration Down commands.
