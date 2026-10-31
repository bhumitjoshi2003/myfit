# Private native VPS trial — awaiting approval and SSH target

No VPS access, transfer or production changes have been performed for this request. This procedure does not enable a public MyFit site. No static frontend installation is needed for the private API trial.

Approved scope to request: transfer the existing immutable image archive, MyFit Compose files, runtime-only mode-0600 env and monitoring script over SSH with verified host keys; stage in a new MyFit-only directory; load the image; start only the MyFit API for a three-minute monitored trial; always stop that API when the trial ends. No migrations, image builds, auth tests, Nginx/DNS changes or existing-container changes.

Release: `20261031T161631Z-amd64-b968e81d5b72`

Image: `myfit-api@sha256:a712b40445c47d466f0bb32859608478caf59d82d63b56b1ffac0506d2edbe07`

Limits: 96 MiB RAM with no extra container swap, 0.5 CPU, pool four, Go soft limit 64 MiB. The trial-only Compose override disables automatic restarts so a failed process stays stopped. Production Compose remains unchanged. Bind only `127.0.0.1:18080`.

Preflight: verify SSH target/host key, Docker/host amd64, image checksum/architecture, port availability, existing production container states and operator-supplied Edunexify health URLs. Require at least 224 MiB current MemAvailable (96 MiB cap plus a 128 MiB host reserve) and no swap-in/out during baseline. Reported 276 MiB available meets this proposed threshold, subject to the live check. Swap capacity is not counted as RAM headroom.

Abort and stop only MyFit on any API exit/restart/OOM, native peak above the reviewed 72 MiB headroom threshold, host available memory below 128 MiB, any failing Edunexify health check or existing-container state regression, two consecutive swap-active five-second intervals, or two Edunexify latency samples above max(1 second, three times baseline worst). Keep all state and reports for diagnosis; never touch Edunexify to make the trial pass.

Use a detached remote supervisor so SSH loss does not abandon a running trial. The script always stops the trial API, including on failed checks. Keep an independent operator session ready to stop only the recorded MyFit container if supervisor termination prevents cleanup. Capture cgroup peak/oom/restart counters, host swap counters and Edunexify status/latency every five seconds. No synthetic registration or unrelated regression tests.

After the target and SSH access are confirmed, prepare the exact transfer command with `StrictHostKeyChecking=yes`, umask 077 and a previously absent MyFit release directory. Do not use passwords on command lines or copy the local production env (which also contains migration credentials). The private workstation file `.run/native-vps-trial/runtime.env` contains only the runtime URL, immutable image reference and runtime limits. Do not display it or resolved Compose output.

After transfer and explicit production approval, the reviewed supervisor command is structurally:

```sh
# In the dedicated staged MyFit directory, with operator-confirmed health URLs:
nohup python3 -u native-vps-trial.py \
  --edunexify-health "$EDUNEXIFY_API_HEALTH_URL" \
  --edunexify-health "$EDUNEXIFY_AI_HEALTH_URL" \
  --seconds 180 > trial-supervisor.log 2>&1 < /dev/null &
```

The concrete URLs and SSH destination must be supplied and preflighted before execution. Retrieve `native-trial-report.json` afterward; independently confirm MyFit is stopped and Edunexify is unchanged. Report native peak/current memory, OOMs, exits/restarts, host swap and Edunexify health. A pass applies only to the private three-minute observed traffic window; public rollout and connection-interruption testing remain separate.
