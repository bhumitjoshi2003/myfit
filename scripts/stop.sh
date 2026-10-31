#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
for service in api frontend; do
  file="$ROOT/.run/$service.pid"
  if [[ -f "$file" ]]; then
    pid="$(cat "$file")"
    command="$(ps -p "$pid" -o command= 2>/dev/null || true)"
    if [[ "$command" == *"$ROOT/.run/myfit-api"* || "$command" == *"$ROOT/frontend/node_modules/vite/bin/vite.js"* ]]; then
      kill "$pid" 2>/dev/null || true
      echo "Stopped MyFit $service."
    fi
    rm -f "$file"
  fi
done
echo 'PostgreSQL is still running. To stop it without deleting data: docker compose stop db'
