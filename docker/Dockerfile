# fnkodi-tvbox — single image: Kodi + fn-tvbox-gateway + video addon
# Base: wjz304/kodi:latest (Kodi 21 / Ubuntu 24.04, includes tini)
# Pin note (dev machine, 2026-08-08): sha256:6cbba5b1bd8e5cb1b7d5ba1d7b330042e972c04fc5fa16dcf77a08e7e752c63b
# Prefer digest in production builds, e.g. FROM wjz304/kodi@sha256:...
FROM wjz304/kodi:latest

LABEL org.opencontainers.image.title="fnkodi-tvbox" \
      org.opencontainers.image.description="fnOS FPK Kodi + TVBox gateway (no Android)" \
      org.opencontainers.image.source="https://github.com/zhanqinwen/FnKodi_TVBOX"

# curl: health wait in entrypoint. Do not install android/redroid/emulator/jar runtimes.
RUN apt-get update \
 && apt-get install -y --no-install-recommends curl \
 && rm -rf /var/lib/apt/lists/*

COPY release/gateway/fn-tvbox-gateway /usr/local/bin/fn-tvbox-gateway
RUN chmod +x /usr/local/bin/fn-tvbox-gateway

COPY release/addons/plugin.video.fntvbox /opt/fnkodi/addons/plugin.video.fntvbox
COPY release/addons/skin.fntvbox /opt/fnkodi/addons/skin.fntvbox
COPY docker/entrypoint.sh /usr/local/bin/fnkodi-entrypoint.sh
RUN chmod +x /usr/local/bin/fnkodi-entrypoint.sh \
 && mkdir -p /var/lib/fn-tvbox /opt/fnkodi/addons

ENV FNTVBOX_LISTEN=127.0.0.1:18765 \
    FNTVBOX_DATA_DIR=/var/lib/fn-tvbox \
    FNTVBOX_HTTP_TIMEOUT_MS=8000 \
    FNTVBOX_PROXY_HEADER_TIMEOUT_MS=15000 \
    FNTVBOX_GATEWAY_MAX_CRASHES=10 \
    FNTVBOX_GATEWAY_RESTART_DELAY_SEC=1 \
    FNTVBOX_GATEWAY_STABLE_SEC=60

# tini is PID 1; fnkodi-entrypoint.sh must remain its direct child (do not exec away).
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/fnkodi-entrypoint.sh"]
