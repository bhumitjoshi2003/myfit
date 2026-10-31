#!/usr/bin/env bash
# Run on a workstation/CI, never the shared 1 GB VPS. No push or deployment.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"
: "${MYFIT_RELEASE:?Set a unique immutable release id (e.g. a Git commit)}"
: "${TARGET_PLATFORM:?Confirm VPS architecture and set linux/amd64 or linux/arm64}"
: "${MYFIT_IMAGE_REPOSITORY:?Set the intended image repository; this script does not push}"
[[ "$MYFIT_RELEASE" =~ ^[a-zA-Z0-9][a-zA-Z0-9._-]{0,79}$ ]] || { echo 'Invalid release id'; exit 2; }
[[ "$TARGET_PLATFORM" == linux/amd64 || "$TARGET_PLATFORM" == linux/arm64 ]] || exit 2
out="$ROOT/deploy/artifacts/$MYFIT_RELEASE"
[[ ! -e "$out" ]] || { echo 'Release output already exists; use a new release id.'; exit 2; }
mkdir -p "$out"
image="$MYFIT_IMAGE_REPOSITORY:$MYFIT_RELEASE"
docker buildx build --platform "$TARGET_PLATFORM" --target runtime --load -t "$image" .
docker buildx build --platform "$TARGET_PLATFORM" --target frontend-assets --output "type=local,dest=$out/site" .
docker image inspect --format '{{.Id}}' "$image" > "$out/image-id.txt"
printf '%s\n' "$image" > "$out/image-tag.txt"
printf '%s\n' "$TARGET_PLATFORM" > "$out/platform.txt"
python3 - "$out/site" > "$out/assets.sha256" <<'PY'
import hashlib, pathlib, sys
root = pathlib.Path(sys.argv[1])
for path in sorted(root.rglob('*')):
    if path.is_file(): print(hashlib.sha256(path.read_bytes()).hexdigest()+'  '+str(path.relative_to(root)))
PY
tar -czf "$out/frontend.tar.gz" -C "$out/site" .
echo "Built $image and $out/frontend.tar.gz. No push, VPS access or deployment performed."
