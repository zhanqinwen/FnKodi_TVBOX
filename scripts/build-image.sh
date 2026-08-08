#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=common.sh
source "$ROOT/scripts/common.sh"
require_cmd docker

VER="$(tr -d '[:space:]' < "$ROOT/VERSION")"
IMAGE="fnkodi-tvbox:${VER}"

if [[ ! -x "$ROOT/release/gateway/fn-tvbox-gateway" ]] && [[ ! -f "$ROOT/release/gateway/fn-tvbox-gateway" ]]; then
  echo "ERROR: missing release/gateway/fn-tvbox-gateway (run scripts/build-gateway.sh)" >&2
  exit 1
fi
if [[ ! -f "$ROOT/release/addons/plugin.video.fntvbox/addon.xml" ]]; then
  echo "ERROR: missing release/addons/plugin.video.fntvbox (run scripts/package-addons.sh)" >&2
  exit 1
fi
if [[ ! -f "$ROOT/release/addons/skin.fntvbox/addon.xml" ]]; then
  echo "ERROR: missing release/addons/skin.fntvbox (run scripts/package-addons.sh)" >&2
  exit 1
fi

docker build -f "$ROOT/docker/Dockerfile" -t "$IMAGE" "$ROOT"
mkdir -p "$ROOT/release/images"
docker save "$IMAGE" | gzip > "$ROOT/release/images/fnkodi-tvbox_${VER}_amd64.tar.gz"
echo "built $IMAGE"
echo "saved $ROOT/release/images/fnkodi-tvbox_${VER}_amd64.tar.gz"
