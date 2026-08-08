#!/usr/bin/env bash
set -euo pipefail

require_linux() {
  if [[ "$(uname -s)" != "Linux" ]]; then
    echo "ERROR: build must run on Linux build server" >&2
    exit 1
  fi
}

require_cmd() {
  local c="$1"
  if ! command -v "$c" >/dev/null 2>&1; then
    echo "ERROR: missing required command: $c" >&2
    exit 1
  fi
}

repo_root() {
  cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd
}
