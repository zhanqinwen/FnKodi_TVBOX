#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=common.sh
source "$ROOT/scripts/common.sh"

require_linux
require_cmd go
require_cmd docker

echo "==> build-gateway"
bash "$ROOT/scripts/build-gateway.sh"

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
