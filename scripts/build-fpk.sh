#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=common.sh
source "$ROOT/scripts/common.sh"

VER="$(tr -d '[:space:]' < "$ROOT/VERSION")"
MANIFEST="$ROOT/fpk/manifest"
COMPOSE="$ROOT/fpk/app/docker/docker-compose.yaml"
OUT_DIR="$ROOT/release/fpk"
OUT_FPK="$OUT_DIR/fnkodi-tvbox_all_v${VER}.fpk"
FNPACK_VERSION="1.2.3"
TOOLS_DIR="$ROOT/tools"

if [[ ! -f "$MANIFEST" ]]; then
  echo "ERROR: missing $MANIFEST" >&2
  exit 1
fi
if [[ ! -f "$COMPOSE" ]]; then
  echo "ERROR: missing $COMPOSE" >&2
  exit 1
fi

# Sync VERSION into manifest and compose image tag.
tmp="$(mktemp)"
sed -E "s/^version[[:space:]]*=.*/version               = ${VER}/" "$MANIFEST" >"$tmp"
mv "$tmp" "$MANIFEST"

tmp="$(mktemp)"
sed -E "s|^([[:space:]]*image:[[:space:]]*)fnkodi-tvbox:[^[:space:]]+|\\1fnkodi-tvbox:${VER}|" "$COMPOSE" >"$tmp"
mv "$tmp" "$COMPOSE"

# Reject host port mappings of 18765; allow FNTVBOX_LISTEN=127.0.0.1:18765 only.
if grep -vE 'FNTVBOX_LISTEN' "$COMPOSE" | grep -E '18765:' >/dev/null 2>&1; then
  echo "ERROR: gateway port 18765 must not be mapped in $COMPOSE" >&2
  exit 1
fi
if grep -E 'service_port.*18765' "$MANIFEST" >/dev/null 2>&1; then
  echo "ERROR: 18765 must not appear in manifest service_port" >&2
  exit 1
fi

ensure_fnpack() {
  local os bin url
  case "$(uname -s)" in
    Linux*)
      os=linux
      bin=fnpack
      ;;
    MINGW*|MSYS*|CYGWIN*)
      os=windows
      bin=fnpack.exe
      ;;
    Darwin*)
      echo "ERROR: fnpack binary is not published for macOS; build on Linux or Windows" >&2
      exit 1
      ;;
    *)
      echo "ERROR: unsupported OS for fnpack: $(uname -s)" >&2
      exit 1
      ;;
  esac

  mkdir -p "$TOOLS_DIR"
  local path="$TOOLS_DIR/$bin"
  if [[ -f "$path" ]]; then
    chmod +x "$path" 2>/dev/null || true
    if [[ -x "$path" ]]; then
      echo "$path"
      return
    fi
    echo "ERROR: $path exists but is not executable (try: chmod +x $path)" >&2
    exit 1
  fi
  if command -v fnpack >/dev/null 2>&1; then
    command -v fnpack
    return
  fi

  url="https://static2.fygonas.com/fygopack/fygopack-${FNPACK_VERSION}-${os}-amd64"
  echo "Downloading $url" >&2
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$path" "$url"
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O "$path" "$url"
  else
    echo "ERROR: need curl or wget to download fnpack" >&2
    exit 1
  fi
  if [[ ! -f "$path" ]]; then
    echo "ERROR: failed to download fnpack to $path" >&2
    exit 1
  fi
  chmod +x "$path"
  if [[ ! -x "$path" ]]; then
    echo "ERROR: cannot chmod +x $path" >&2
    exit 1
  fi
  echo "$path"
}

FNPACK="$(ensure_fnpack)"
mkdir -p "$OUT_DIR"

# fnpack writes <appname>.fpk into the process cwd — always run from repo root.
cd "$ROOT"
rm -f "$ROOT/fnkodi-tvbox.fpk" "$ROOT/fnkodi-tvbox_all_v${VER}.fpk"
"$FNPACK" build --directory "$ROOT/fpk"

SOURCE=""
for cand in \
  "$ROOT/fnkodi-tvbox.fpk" \
  "$ROOT/fnkodi-tvbox_all_v${VER}.fpk" \
  "$ROOT/fpk/fnkodi-tvbox.fpk" \
  "$ROOT/fpk/fnkodi-tvbox_all_v${VER}.fpk"
do
  if [[ -f "$cand" ]]; then
    SOURCE="$cand"
    break
  fi
done
if [[ -z "${SOURCE}" ]]; then
  SOURCE="$(ls -t "$ROOT"/fnkodi-tvbox*.fpk "$ROOT"/fpk/fnkodi-tvbox*.fpk 2>/dev/null | head -n1 || true)"
fi

if [[ -z "${SOURCE}" ]] || [[ ! -f "$SOURCE" ]]; then
  echo "ERROR: fnpack did not produce fnkodi-tvbox*.fpk under $ROOT (cwd was $(pwd))" >&2
  ls -la "$ROOT"/*.fpk "$ROOT"/fpk/*.fpk 2>/dev/null || true
  exit 1
fi

mv -f "$SOURCE" "$OUT_FPK"
echo "built $OUT_FPK"
