#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
umask 077
mkdir -p .run
if [[ -f .env ]]; then set -a; source .env; set +a; fi
for service in api frontend; do
  if [[ -f ".run/$service.pid" ]] && kill -0 "$(cat ".run/$service.pid")" 2>/dev/null; then
    echo "MyFit $service is already running. Use ./scripts/stop.sh first."; exit 1
  fi
done
for port in 8080 5173; do
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "Port $port is in use. Stop that service before starting MyFit."; exit 1
  fi
done
docker compose up -d --wait db
if [[ ! -d frontend/node_modules ]]; then npm ci --prefix frontend; fi
(cd backend && go build -o "$ROOT/.run/myfit-api" .)
(cd "$ROOT/backend"; nohup "$ROOT/.run/myfit-api" > "$ROOT/.run/api.log" 2>&1 & echo $! > "$ROOT/.run/api.pid")
# Run node directly so the saved PID owns the frontend server.
nohup node "$ROOT/frontend/node_modules/vite/bin/vite.js" "$ROOT/frontend" --host 127.0.0.1 --port 5173 > "$ROOT/.run/frontend.log" 2>&1 &
echo $! > .run/frontend.pid
for attempt in {1..40}; do
  if curl -fsS http://127.0.0.1:8080/health >/dev/null 2>&1 && curl -fsS http://localhost:5173 >/dev/null 2>&1; then
    echo 'MyFit is ready: http://localhost:5173'; echo 'Logs: .run/api.log and .run/frontend.log'; exit 0
  fi
  sleep 1
done
echo 'Startup failed. Check .run/api.log and .run/frontend.log.'
"$ROOT/scripts/stop.sh"
exit 1
