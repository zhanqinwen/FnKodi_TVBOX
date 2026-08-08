#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=common.sh
source "$ROOT/scripts/common.sh"

VER="$(tr -d '[:space:]' < "$ROOT/VERSION")"
OUT_DIR="$ROOT/release/addons"

make_zip() {
  local id="$1"
  local dest="$2"
  local zip="$3"

  if command -v zip >/dev/null 2>&1; then
    (
      cd "$OUT_DIR"
      zip -rq "$(basename "$zip")" "$id"
    )
    return 0
  fi

  local pybin="" pyargs=()
  local cand
  for cand in /mnt/c/Windows/py.exe py python3 python; do
    if ! command -v "$cand" >/dev/null 2>&1 && [[ ! -x "$cand" ]]; then
      continue
    fi
    if [[ "$(basename "$cand")" == py || "$(basename "$cand")" == py.exe ]]; then
      if "$cand" -3 -c 'import zipfile' >/dev/null 2>&1; then
        pybin="$cand"
        pyargs=(-3 -)
        break
      fi
    elif "$cand" -c 'import zipfile' >/dev/null 2>&1; then
      pybin="$cand"
      pyargs=(-)
      break
    fi
  done

  if [[ -z "$pybin" ]]; then
    echo "WARN: zip/python unavailable; skipped $zip (directory package is enough for Docker)" >&2
    return 0
  fi

  DEST="$dest" ZIP="$zip" ADDON_ID="$id" "$pybin" "${pyargs[@]}" <<'PY'
import os
import zipfile
from pathlib import Path

dest = Path(os.environ["DEST"])
out = Path(os.environ["ZIP"])
addon_id = os.environ["ADDON_ID"]
with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED) as zf:
    for p in dest.rglob("*"):
        if p.is_file():
            arc = Path(addon_id) / p.relative_to(dest)
            zf.write(p, arcname=arc.as_posix())
print("zipped", out)
PY
}

package_one() {
  local id="$1"
  local src="$ROOT/plugins/$id"
  local dest="$OUT_DIR/$id"
  local zip="$OUT_DIR/${id}-${VER}.zip"

  if [[ ! -f "$src/addon.xml" ]]; then
    echo "ERROR: missing $src/addon.xml" >&2
    exit 1
  fi

  rm -rf "$dest"
  mkdir -p "$OUT_DIR"
  if command -v rsync >/dev/null 2>&1; then
    rsync -a \
      --exclude '__pycache__/' \
      --exclude '*.pyc' \
      --exclude '.DS_Store' \
      "$src/" "$dest/"
  else
    mkdir -p "$dest"
    cp -R "$src/." "$dest/"
    find "$dest" -type d -name '__pycache__' -prune -exec rm -rf {} + 2>/dev/null || true
    find "$dest" -type f -name '*.pyc' -delete 2>/dev/null || true
  fi

  rm -f "$zip"
  make_zip "$id" "$dest" "$zip"

  echo "packaged $dest"
  if [[ -f "$zip" ]]; then
    echo "packaged $zip"
  fi
}

package_one "plugin.video.fntvbox"
package_one "skin.fntvbox"
