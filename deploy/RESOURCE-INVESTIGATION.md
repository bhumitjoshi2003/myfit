# Focused restart and memory investigation

Application release `20261031T161631Z-amd64-b968e81d5b72` and its API/frontend archives are unchanged. No VPS, DNS, Nginx, Edunexify or database writes were performed. Only short local startup observations were used; no application/authentication/frontend regression tests or extended workload were repeated.

## Restart evidence and fix

Docker's retained events show the prior API container `myfit-readiness-d26c32db` started at 16:23:19 UTC, exited **1** that second and restarted. Its first health-check process ran at 16:23:24, after the restart. The captured memory events had zero OOM/oom_kill counters. The later API exit **0** at 16:23:31 was intentional cleanup. The relay's later exit **137** followed cleanup's SIGKILL after its stop timeout; it was not the API restart or evidence of an API OOM.

The prior container had already been removed, so its detailed state and stderr are irrecoverable through Docker. A short reproduction of the same startup ordering captured **one restart** and this redacted fatal log:

```text
failed to connect to user=myfit_runtime database=neondb:
dial tcp [LOCAL_RELAY_IP]:5432: connect: connection refused
MyFit API listening on 0.0.0.0:8080
```

The harness treated `docker run -d` as relay readiness. The API could attempt its startup database ping before the Python relay bound its TCP listener; startup errors intentionally exit 1, triggering the configured restart policy. This reproduces and explains the historical exit/timing; the original lost stderr cannot independently prove its exact fatal line.

**Fix:** both local harnesses now wait for the relay control listener before starting the API. The relay binds its TCP listener before its control listener, so a control HTTP 200 establishes that binding has completed. A startup-only verification with this gate reported API listening, **zero restarts** and no startup failure. Production Compose does not use this relay and needs no corresponding change. The previously fixed normal bridge attachment remains in place.

## Memory evidence

The historical **87.7 MiB** was `memory.peak` for the entire cgroup, not Go heap or application RSS. Its exact historical composition cannot be recovered because `memory.stat` was not captured then. The focused observation collected `memory.stat`, process command lines and `smaps_rollup` without changing the image:

| Observation | Cgroup current | Cgroup peak | API process RSS |
| --- | ---: | ---: | ---: |
| AMD64 via Rosetta, idle | 57.32 MiB | 85.47 MiB | 58.38 MiB |
| Same emulated API, after one probe | 60.30 MiB | 92.12 MiB | 58.38 MiB |
| Same Go sources, native ARM64, idle | 23.55 MiB | 23.55 MiB | 14.70 MiB |

The emulated idle cgroup contained **52.96 MiB anonymous**, **1.88 MiB file/cache**, and **1.28 MiB kernel** memory. Its PID 1 command was `/run/rosetta/rosetta /app/myfit /app/myfit`. File cache was therefore not the dominant charge in that observation. The short-lived emulated health-check process reached approximately **44.28 MiB RSS** in a captured sample; RSS across processes includes shared mappings and must not be added as independent usage. All observed memory OOM counters remained zero; the cap stayed **96 MiB**.

A native ARM64 executable built from verified unchanged release Go sources was mounted read-only for comparison only, not packaged into the release. Its lower RSS supports substantial translation overhead; ARM64 does not establish native AMD64 memory needs. The native comparison used the actual Neon endpoint directly, while the emulated case used the opaque relay; it is diagnostic rather than a controlled architecture benchmark. Both used verified-TLS runtime credentials, the same cap, CPU and Go limits. A small native observation process also contributes memory to these cgroup measurements. These figures do not establish exact Go heap usage or a passing application workload.

Cgroup accounting includes application/probe processes, anonymous memory, charged file cache and kernel allocations; it does not represent the Docker daemon/whole VM's memory. See the [kernel cgroup memory accounting documentation](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html). Thus **87.7 MiB must not be reported as the Go application's native memory requirement**, nor discounted entirely as harmless cache.

## Probe and startup code inspection

`healthcheck` uses an HTTP client with a three-second timeout and exits before configuration, database pooling, migration collection or HTTP server construction. `memory-stats` also returns before those steps. They launch dedicated short-lived processes, never another API server. Docker's retained healthcheck exec events and the observed `... myfit healthcheck` command agree with this code path. Healthcheck failure alone does not trigger Docker's `on-failure` restart policy; the API's startup exit did.

No confirmed unnecessary startup allocation was found: the dummy bcrypt credential is already precomputed; production auto-migration is disabled; pgx minimum connections is zero and maximum four, statement caches are disabled; no food/catalogue preload or unbounded startup cache is present. No speculative application optimization or probe rewrite was made.

## Recommendation

A **short, separately authorized native AMD64 VPS trial is reasonable**, not a production-readiness approval. First establish sufficient current RAM headroom for the entire 96 MiB cap plus an explicit Edunexify/host safety reserve. The supplied idle CPU and zero swap-I/O samples alone do not establish that budget. If headroom is insufficient, do not start MyFit on the shared VPS.

Keep the existing image, 96 MiB cap, no extra swap and 0.5 CPU limit; expose only loopback, with no public Nginx/DNS activation. Observe startup and a small agreed workload, including ordinary native probes, while watching cgroup anon/file/kernel/peak, OOM events, restart count, host swap and Edunexify health/latency. Abort on any API OOM/restart, worsening school-service metrics or inadequate native memory headroom. Do not increase the cap without a revised host budget. Native capacity and the previously uncompleted scoped reconnection validation remain unresolved.

Local evidence: release artifact `resource-investigation.json`; private detailed observations in `.run/resource-investigation/`. Test containers/networks were removed after evidence capture; credentials were never included in reports.
