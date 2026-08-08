#!/usr/bin/env bash
# fnkodi-entrypoint.sh — tini 的直接子进程。
# 禁止对本脚本使用 exec 替换自身（会丢失 trap，gateway 变孤儿）。
set -euo pipefail

DATA_DIR="${FNTVBOX_DATA_DIR:-/var/lib/fn-tvbox}"
ADDON_SRC="/opt/fnkodi/addons/plugin.video.fntvbox"
ADDON_DST="/root/.kodi/addons/plugin.video.fntvbox"
SKIN_SRC="/opt/fnkodi/addons/skin.fntvbox"
SKIN_DST="/root/.kodi/addons/skin.fntvbox"
SKIN_MARKER="/root/.kodi/userdata/.fnkodi-default-skin"
GATEWAY_BIN="/usr/local/bin/fn-tvbox-gateway"
HEALTH_URL="http://127.0.0.1:18765/health"
GATEWAY_PID_FILE="${DATA_DIR}/gateway.pid"

KODI_PID=""
WATCHDOG_PID=""
SHUTTING_DOWN=0
EXIT_CODE=0

log() { printf '[fnkodi] %s\n' "$*"; }
err() { printf '[fnkodi] ERROR: %s\n' "$*" >&2; }

sync_addon_dir() {
  local src="$1"
  local dst="$2"
  local label="$3"
  if [[ ! -f "${src}/addon.xml" ]]; then
    err "bundled ${label} missing: ${src}/addon.xml"
    return 1
  fi
  mkdir -p /root/.kodi/addons
  if [[ ! -f "${dst}/addon.xml" ]]; then
    log "installing ${label} -> ${dst}"
    cp -a "${src}" /root/.kodi/addons/
  else
    log "refreshing ${label} -> ${dst}"
    cp -a "${src}/." "${dst}/"
  fi
}

set_default_skin_once() {
  # Only on first userdata init; never re-force if user switched back to Estuary.
  if [[ -f "${SKIN_MARKER}" ]]; then
    return 0
  fi
  if [[ ! -f "${SKIN_DST}/addon.xml" ]]; then
    err "cannot set default skin: ${SKIN_DST}/addon.xml missing"
    return 1
  fi
  mkdir -p /root/.kodi/userdata
  local gui="/root/.kodi/userdata/guisettings.xml"
  if [[ -f "${gui}" ]] && grep -q 'id="lookandfeel.skin"' "${gui}"; then
    log "setting lookandfeel.skin=skin.fntvbox (existing guisettings)"
    sed -i 's#\(id="lookandfeel.skin"[^>]*>\)[^<]*#\1skin.fntvbox#' "${gui}"
  elif [[ -f "${gui}" ]]; then
    log "inserting lookandfeel.skin=skin.fntvbox into guisettings"
    sed -i 's#</settings>#    <setting id="lookandfeel.skin">skin.fntvbox</setting>\n</settings>#' "${gui}"
  else
    log "writing minimal guisettings with default skin.fntvbox"
    cat > "${gui}" <<'EOF'
<?xml version="1.0" encoding="utf-8" standalone="yes"?>
<settings version="2">
    <setting id="lookandfeel.skin">skin.fntvbox</setting>
</settings>
EOF
  fi
  touch "${SKIN_MARKER}"
}

install_addon() {
  mkdir -p /root/.kodi/userdata/addon_data/plugin.video.fntvbox
  sync_addon_dir "${ADDON_SRC}" "${ADDON_DST}" "addon" || return 1
  sync_addon_dir "${SKIN_SRC}" "${SKIN_DST}" "skin" || return 1
  set_default_skin_once || return 1
}

stop_kodi() {
  if [[ -z "${KODI_PID}" ]]; then
    return 0
  fi
  if ! kill -0 "${KODI_PID}" 2>/dev/null; then
    wait "${KODI_PID}" 2>/dev/null || true
    KODI_PID=""
    return 0
  fi
  log "stopping Kodi pgid=${KODI_PID}"
  # 进程组策略：Kodi 经 setsid 启动，PGID==KODI_PID；负 PID 向整组发信号
  kill -TERM -- "-${KODI_PID}" 2>/dev/null || kill -TERM "${KODI_PID}" 2>/dev/null || true
  local i
  for i in $(seq 1 30); do
    if ! kill -0 "${KODI_PID}" 2>/dev/null; then
      break
    fi
    sleep 1
  done
  if kill -0 "${KODI_PID}" 2>/dev/null; then
    log "Kodi still alive, SIGKILL"
    kill -KILL -- "-${KODI_PID}" 2>/dev/null || kill -KILL "${KODI_PID}" 2>/dev/null || true
  fi
  wait "${KODI_PID}" 2>/dev/null || true
  KODI_PID=""
}

stop_gateway_stack() {
  if [[ -n "${WATCHDOG_PID}" ]] && kill -0 "${WATCHDOG_PID}" 2>/dev/null; then
    log "stopping gateway watchdog pid=${WATCHDOG_PID}"
    kill -TERM "${WATCHDOG_PID}" 2>/dev/null || true
  fi
  if [[ -f "${GATEWAY_PID_FILE}" ]]; then
    local gpid
    gpid="$(cat "${GATEWAY_PID_FILE}" 2>/dev/null || true)"
    if [[ -n "${gpid}" ]] && kill -0 "${gpid}" 2>/dev/null; then
      log "stopping gateway pid=${gpid}"
      kill -TERM "${gpid}" 2>/dev/null || true
      local j
      for j in $(seq 1 10); do
        kill -0 "${gpid}" 2>/dev/null || break
        sleep 1
      done
      kill -KILL "${gpid}" 2>/dev/null || true
    fi
    rm -f "${GATEWAY_PID_FILE}"
  fi
  # 兜底：按进程名清理（防 pid 文件丢失）
  if command -v pidof >/dev/null 2>&1; then
    pidof fn-tvbox-gateway >/dev/null 2>&1 && kill -TERM $(pidof fn-tvbox-gateway) 2>/dev/null || true
  fi
  if [[ -n "${WATCHDOG_PID}" ]]; then
    wait "${WATCHDOG_PID}" 2>/dev/null || true
    WATCHDOG_PID=""
  fi
}

cleanup() {
  local ec="${EXIT_CODE:-$?}"
  if [[ "${SHUTTING_DOWN}" -eq 1 ]]; then
    return 0
  fi
  SHUTTING_DOWN=1
  trap - SIGTERM SIGINT EXIT
  log "cleanup begin exit_code=${ec}"
  # 关闭顺序：先 Kodi，再 watchdog/gateway
  stop_kodi
  stop_gateway_stack
  log "cleanup done"
  exit "${ec}"
}

on_signal() {
  EXIT_CODE=0
  cleanup
}

gateway_watchdog() {
  local crashes=0
  local max="${FNTVBOX_GATEWAY_MAX_CRASHES:-10}"
  local delay="${FNTVBOX_GATEWAY_RESTART_DELAY_SEC:-1}"
  local stable="${FNTVBOX_GATEWAY_STABLE_SEC:-60}"

  while true; do
    "${GATEWAY_BIN}" &
    local gpid=$!
    echo "${gpid}" > "${GATEWAY_PID_FILE}"
    log "gateway started pid=${gpid}"
    local start=${SECONDS}
    set +e
    wait "${gpid}"
    local ec=$?
    set -e
    local alive=$((SECONDS - start))
    if (( alive >= stable )); then
      crashes=0
    fi
    crashes=$((crashes + 1))
    err "gateway_crash exit=${ec} count=${crashes} alive=${alive}s"
    if (( crashes >= max )); then
      err "gateway_crash_limit_exceeded max=${max}"
      # 触发主脚本清理并以非 0 退出（依赖 compose restart）
      EXIT_CODE=1
      kill -TERM $$ 2>/dev/null || true
      exit 1
    fi
    sleep "${delay}"
  done
}

wait_health() {
  local i
  for i in $(seq 1 30); do
    if curl -fsS "${HEALTH_URL}" >/dev/null 2>&1; then
      log "gateway healthy"
      return 0
    fi
    sleep 1
  done
  err "gateway health check failed after 30s"
  return 1
}

main() {
  trap on_signal SIGTERM SIGINT
  trap cleanup EXIT

  mkdir -p "${DATA_DIR}"

  if [[ ! -x "${GATEWAY_BIN}" ]]; then
    err "missing ${GATEWAY_BIN}"
    EXIT_CODE=1
    exit 1
  fi

  # Overlap addon/skin sync with gateway boot (P10.3).
  local addon_rc=0
  install_addon &
  local ADDON_PID=$!
  log "addon sync pid=${ADDON_PID}"

  gateway_watchdog &
  WATCHDOG_PID=$!
  log "watchdog pid=${WATCHDOG_PID}"

  set +e
  wait "${ADDON_PID}"
  addon_rc=$?
  set -e
  if [[ "${addon_rc}" -ne 0 ]]; then
    err "addon/skin sync failed code=${addon_rc}"
    EXIT_CODE=1
    exit 1
  fi
  log "addon/skin sync done"

  if ! wait_health; then
    EXIT_CODE=1
    exit 1
  fi

  # 子进程启动上游 Kodi entrypoint（禁止 exec 掉本脚本）
  # 进程组策略：setsid，关闭时 kill -- -PGID
  if [[ -x /usr/local/bin/entrypoint.sh ]]; then
    log "starting upstream /usr/local/bin/entrypoint.sh (setsid, no exec-self)"
    setsid /usr/local/bin/entrypoint.sh &
  else
    log "starting kodi-standalone (setsid)"
    setsid kodi-standalone &
  fi
  KODI_PID=$!
  log "kodi supervisor pid/pgid=${KODI_PID}"

  set +e
  wait "${KODI_PID}"
  EXIT_CODE=$?
  set -e
  KODI_PID=""
  log "Kodi exited code=${EXIT_CODE}"
  # EXIT trap → cleanup gateway
}

main "$@"
