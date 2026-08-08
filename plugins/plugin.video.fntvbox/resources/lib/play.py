# -*- coding: utf-8 -*-
from __future__ import annotations

import json
from typing import Any, Dict, Optional
from urllib.parse import urlparse

import xbmcaddon
import xbmcgui
import xbmcplugin

from .api_client import APIClient, APIError


def _has_inputstream_adaptive() -> bool:
    try:
        return xbmcaddon.Addon("inputstream.adaptive") is not None
    except Exception:  # noqa: BLE001
        return False


def _looks_hls(url: str, fmt: str) -> bool:
    f = (fmt or "").lower()
    if f in ("hls", "m3u8"):
        return True
    path = (urlparse(url).path or "").lower()
    return path.endswith(".m3u8") or ".m3u8" in path


def play_item(handle: int, addon, api: APIClient, params: Dict[str, Any]) -> None:
    source_id = params.get("sourceId") or ""
    play_url = params.get("playUrl") or ""
    media_id = params.get("mediaId") or ""
    episode_id = params.get("episodeId") or ""
    play_from = params.get("playFrom") or ""
    headers: Optional[Dict[str, str]] = None
    headers_json = params.get("headersJson") or ""
    if headers_json:
        try:
            loaded = json.loads(headers_json)
            if isinstance(loaded, dict):
                headers = {str(k): str(v) for k, v in loaded.items()}
        except json.JSONDecodeError:
            headers = None

    if not source_id or not play_url:
        _fail(handle, addon, addon.getLocalizedString(30025) or "Resolve failed")
        return

    try:
        resolved = api.resolve(
            source_id=source_id,
            play_url=play_url,
            media_id=media_id,
            episode_id=episode_id,
            play_from=play_from,
            headers=headers,
        )
    except APIError as exc:
        _fail(handle, addon, exc.message or (addon.getLocalizedString(30025) or "Resolve failed"))
        return

    final_url = resolved.get("url") or ""
    resp_headers = resolved.get("headers") or {}
    if not final_url:
        _fail(handle, addon, addon.getLocalizedString(30025) or "Resolve failed")
        return

    if resp_headers:
        try:
            proxied = api.proxy_session(final_url, headers=resp_headers)
            final_url = proxied.get("playUrl") or final_url
        except APIError as exc:
            _fail(handle, addon, exc.message or (addon.getLocalizedString(30025) or "Resolve failed"))
            return

    li = xbmcgui.ListItem(offscreen=True)
    li.setPath(final_url)
    li.setProperty("IsPlayable", "true")

    fmt = resolved.get("format") or ""
    if _looks_hls(final_url, fmt) and _has_inputstream_adaptive():
        li.setProperty("inputstream", "inputstream.adaptive")
        li.setProperty("inputstream.adaptive.manifest_type", "hls")
        # When using proxy, headers already applied by gateway; do not double-set.

    xbmcplugin.setResolvedUrl(handle, True, li)


def _fail(handle: int, addon, message: str) -> None:
    title = addon.getLocalizedString(30000) or "FnKodi TVBox"
    xbmcgui.Dialog().notification(title, message, xbmcgui.NOTIFICATION_ERROR, 5000)
    li = xbmcgui.ListItem(offscreen=True)
    xbmcplugin.setResolvedUrl(handle, False, li)
