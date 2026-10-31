#!/usr/bin/env bash
# Read-only resource sampler. Run with a staged MyFit container and realistic load.
set -euo pipefail
if [[ $# -lt 1 || $# -gt 2 ]]; then echo "Usage: $0 MYFIT_CONTAINER_ID [seconds=300]"; exit 2; fi
cid="$1"; seconds="${2:-300}"
[[ "$seconds" =~ ^[0-9]+$ ]] || exit 2
project="$(docker inspect --format '{{index .Config.Labels "com.docker.compose.project"}}' "$cid")"
[[ "$project" == myfit-prod || "$project" == myfit-memory-test ]] || { echo 'Refusing to monitor an unrelated container.'; exit 2; }
echo 'timestamp,memory_current_bytes,memory_peak_bytes,memory_max_bytes,oom,oom_kill,restarts'
for ((i=0;i<seconds;i+=5)); do
  values="$(docker exec "$cid" /app/myfit memory-stats)"
  restarts="$(docker inspect --format '{{.RestartCount}}' "$cid")"
  echo "$(date -u +%FT%TZ),$values,$restarts"
  sleep 5
done
docker stats --no-stream "$cid"
echo '96 MiB is a candidate limit only. Assess peak under realistic concurrency, cold starts and sustained load, with headroom and zero OOM events.' >&2
