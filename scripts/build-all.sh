#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=common.sh
source "$ROOT/scripts/common.sh"

require_cmd docker

GW="$ROOT/release/gateway/fn-tvbox-gateway"
if [[ -f "$GW" ]] && [[ "${FORCE_GATEWAY:-0}" != "1" ]]; then
  echo "==> build-gateway (skip: $GW already present; set FORCE_GATEWAY=1 to rebuild)"
else
  require_cmd go
  echo "==> build-gateway"
  bash "$ROOT/scripts/build-gateway.sh"
fi

echo "==> package-addons"
bash "$ROOT/scripts/package-addons.sh"

echo "==> build-image"
bash "$ROOT/scripts/build-image.sh"

echo "==> build-fpk"
bash "$ROOT/scripts/build-fpk.sh"

echo "==> SHA256SUMS"
cd "$ROOT/release"
find gateway addons images fpk -type f -print0 2>/dev/null | sort -z | xargs -0 sha256sum > SHA256SUMS
echo "wrote $ROOT/release/SHA256SUMS"

echo "build-all complete"
ls -lh "$ROOT/release/gateway" "$ROOT/release/images" "$ROOT/release/fpk" 2>/dev/null || true
