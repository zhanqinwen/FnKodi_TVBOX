#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=common.sh
source "$ROOT/scripts/common.sh"
require_cmd go
VER="$(tr -d '[:space:]' < "$ROOT/VERSION")"
OUT="$ROOT/release/gateway"
mkdir -p "$OUT"
cd "$ROOT/plugins/fn-tvbox-gateway"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=${VER}" -o "$OUT/fn-tvbox-gateway" ./cmd/gateway
echo "built $OUT/fn-tvbox-gateway"
