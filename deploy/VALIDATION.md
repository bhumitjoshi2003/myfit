# Local deployment validation — 2026-10-31

No VPS, DNS, external registry push, Neon connection, production migration or host Nginx change was performed. All containers in these tests ran on local Docker Desktop. The existing development PostgreSQL container was reused only for disposable, uniquely named schemas; those schemas and the test API/Nginx containers were removed after each test. Existing MyFit records were preserved.

## Completed checks

- Multi-stage scratch API image built successfully, nonroot UID/GID 65532, CA roots and embedded time zones.
- React production export built inside the Docker frontend stage; output is `deploy/artifacts/site` (ignored build artifact).
- Production Compose validated with the example environment using `config --quiet`; only the API service and unique MyFit network are declared.
- Standalone `nginx -t` succeeded inside a disposable Nginx container with the supplied vhost/snippets and a local test certificate.
- HTTPS routing exercised through a disposable Nginx proxy on loopback, with certificate and hostname validation against an explicit local test CA (not `-k`/insecure mode).
- Verification script passed readiness, liveness, React shell/static routing, security headers, missing asset 404, anonymous API protection, exact-origin rejection, isolated Secure/HttpOnly/host-only cookie checks, login persistence, CSRF enforcement and logout revocation.
- Local container exercised registration, onboarding recommendations/acceptance, food search/logging, weight persistence and login/logout. Final image also stopped cleanly on SIGTERM with exit code 0; it waits for HTTP shutdown before closing the pool.
- Go unit/persistence tests cover the new production-config guards, no automatic migration/schema mismatch rejection, and secure cookie namespace/attributes. **Final serial `go test -race ./...` and `go vet ./...` passed**, including persistence, authorization, onboarding and production configuration/cookie tests.
- Shell scripts passed `bash -n`; Python scripts passed compilation. No deployment script connects to a VPS automatically.

## Actual measured container memory

| Setting/result | Value |
| --- | --- |
| Architecture | arm64 (local Linux container on Docker Desktop) |
| Final image size | 16,226,829 bytes, about 15.48 MiB unpacked |
| Container hard cap | 96 MiB (100,663,296 bytes) |
| Go soft memory limit | 64 MiB |
| CPU budget / Go scheduler | 0.50 CPU / GOMAXPROCS=1 |
| DB pool | max 4, min 0 |
| Peak cgroup memory, final run | **26.12 MiB** (27,385,856 bytes) |
| OOM / OOM-kill events | **0 / 0** |
| Auth workload | 4 registrations, 4 logins, plus one HTTPS verifier login/logout |
| Concurrent read workload | 280 requests, concurrency 2, 30 seconds |
| Additional checks | onboarding, diary write, weight write, health checks and read-only memory sampling |

An earlier pre-optimization run peaked at 30.23 MiB without OOMs. That run revealed unnecessary dummy-bcrypt initialization in each health-check process; the public dummy hash is now precomputed while password hashing and failed-login comparison still use bcrypt cost 12.

**This is a bounded local smoke test, not a capacity certification.** The local DB used development TCP without database TLS, and migrations ran only in a disposable development schema. The actual Neon endpoint/roles, database TLS/cold starts, x86 VPS if applicable, real connection bursts, longer soaks, host/kernel overhead and existing Edunexify load were not validated. HTTP TLS used a private test CA only in the local sandbox; live certificate coverage and renewal remain unverified. The 96 MiB limit stays provisional until representative staging and shared-host headroom gates pass. No claim of safe VPS capacity or guaranteed Edunexify uptime is made.

A parallel race-test/image-build attempt exceeded the API request deadline under local resource contention. The final regression run is performed serially with the test container stopped; production resource admission must likewise be based on measured host headroom, not only container fit.
