# -*- coding: utf-8 -*-
from __future__ import annotations

from typing import Any, Dict, List, Optional
from urllib.parse import urlencode

import xbmcgui
import xbmcplugin

from .api_client import APIClient, APIError


def _plugin_url(base_url: str, **params: Any) -> str:
    clean = {k: v for k, v in params.items() if v is not None and v != ""}
    return base_url + "?" + urlencode(clean, doseq=True)


def _add_folder(
    handle: int,
    base_url: str,
    label: str,
    action: str,
    art: Optional[Dict[str, str]] = None,
    info: Optional[Dict[str, Any]] = None,
    **params: Any,
) -> None:
    url = _plugin_url(base_url, action=action, **params)
    li = xbmcgui.ListItem(label=label, offscreen=True)
    if art:
        li.setArt(art)
    if info:
        li.setInfo("video", info)
    xbmcplugin.addDirectoryItem(handle, url, li, isFolder=True)


def _add_playable(
    handle: int,
    base_url: str,
    label: str,
    art: Optional[Dict[str, str]] = None,
    info: Optional[Dict[str, Any]] = None,
    **params: Any,
) -> None:
    url = _plugin_url(base_url, action="play", **params)
    li = xbmcgui.ListItem(label=label, offscreen=True)
    li.setProperty("IsPlayable", "true")
    if art:
        li.setArt(art)
    if info:
        li.setInfo("video", info)
    xbmcplugin.addDirectoryItem(handle, url, li, isFolder=False)


def show_root(handle: int, base_url: str, addon, api: APIClient) -> None:
    api.sync_subscription_from_settings()
    try:
        data = api.sources()
    except APIError:
        xbmcplugin.endOfDirectory(handle, succeeded=False)
        return

    sources: List[Dict[str, Any]] = data.get("sources") or []
    if not sources:
        _notify_empty_sources(addon, api)

    _add_folder(handle, base_url, addon.getLocalizedString(30010) or "Search", "search")
    _add_folder(handle, base_url, addon.getLocalizedString(30011) or "Live TV", "live_groups")

    for src in sources:
        if src.get("hidden"):
            continue
        sid = src.get("id") or ""
        name = src.get("name") or sid
        _add_folder(
            handle,
            base_url,
            name,
            "categories",
            sourceId=sid,
            info={"title": name, "plot": "type=%s runtime=%s" % (src.get("type"), src.get("runtime"))},
        )
    xbmcplugin.endOfDirectory(handle)


def _notify_empty_sources(addon, api: APIClient) -> None:
    title = addon.getLocalizedString(30000) or "FnKodi TVBox"
    try:
        sub = api.get_subscription()
    except APIError:
        xbmcgui.Dialog().notification(
            title,
            addon.getLocalizedString(30024) or "No sources",
            xbmcgui.NOTIFICATION_WARNING,
            5000,
        )
        return

    last_err = sub.get("lastError") or {}
    skipped = int(sub.get("skippedUnsupported") or 0)
    if last_err:
        msg = addon.getLocalizedString(30022) or "Subscription failed"
        detail = last_err.get("message") or last_err.get("code") or ""
        xbmcgui.Dialog().notification(title, "%s: %s" % (msg, detail), xbmcgui.NOTIFICATION_ERROR, 6000)
    elif skipped > 0:
        xbmcgui.Dialog().notification(
            title,
            addon.getLocalizedString(30023) or "No supported sources",
            xbmcgui.NOTIFICATION_WARNING,
            6000,
        )
    else:
        xbmcgui.Dialog().notification(
            title,
            addon.getLocalizedString(30024) or "No sources",
            xbmcgui.NOTIFICATION_INFO,
            4000,
        )


def show_categories(handle: int, base_url: str, api: APIClient, source_id: str) -> None:
    try:
        data = api.categories(source_id)
    except APIError:
        xbmcplugin.endOfDirectory(handle, succeeded=False)
        return
    for cat in data.get("categories") or []:
        cid = cat.get("id") or ""
        name = cat.get("name") or cid
        _add_folder(
            handle,
            base_url,
            name,
            "media",
            sourceId=source_id,
            categoryId=cid,
            page="1",
        )
    xbmcplugin.endOfDirectory(handle)


def show_media(
    handle: int,
    base_url: str,
    addon,
    api: APIClient,
    source_id: str,
    category_id: str,
    page: int,
) -> None:
    try:
        data = api.media(source_id, category_id, page=page)
    except APIError:
        xbmcplugin.endOfDirectory(handle, succeeded=False)
        return

    items = data.get("items") or []
    for item in items:
        mid = item.get("id") or ""
        title = item.get("title") or mid
        cover = item.get("coverUrl") or ""
        art = {"thumb": cover, "poster": cover, "fanart": cover} if cover else None
        plot = item.get("description") or item.get("remarks") or ""
        _add_folder(
            handle,
            base_url,
            title,
            "detail",
            sourceId=source_id,
            mediaId=mid,
            art=art,
            info={"title": title, "plot": plot, "year": item.get("year") or ""},
        )

    page_count = data.get("pageCount")
    cur = int(data.get("page") or page)
    if page_count is not None:
        if cur < int(page_count):
            _add_folder(
                handle,
                base_url,
                addon.getLocalizedString(30012) or "Next page",
                "media",
                sourceId=source_id,
                categoryId=category_id,
                page=str(cur + 1),
            )
    elif items:
        _add_folder(
            handle,
            base_url,
            addon.getLocalizedString(30012) or "Next page",
            "media",
            sourceId=source_id,
            categoryId=category_id,
            page=str(cur + 1),
        )

    xbmcplugin.setContent(handle, "movies")
    xbmcplugin.endOfDirectory(handle)


def show_detail(handle: int, base_url: str, api: APIClient, source_id: str, media_id: str) -> None:
    try:
        data = api.detail(source_id, media_id)
    except APIError:
        xbmcplugin.endOfDirectory(handle, succeeded=False)
        return

    cover = data.get("coverUrl") or ""
    art = {"thumb": cover, "poster": cover, "fanart": cover} if cover else None
    title = data.get("title") or media_id
    episodes = data.get("episodes") or []

    # Group by playFrom for clearer labels
    for ep in episodes:
        eid = ep.get("id") or ""
        play_from = ep.get("playFrom") or ""
        ep_title = ep.get("title") or eid
        label = ("%s - %s" % (play_from, ep_title)) if play_from else ep_title
        _add_playable(
            handle,
            base_url,
            label,
            art=art,
            info={"title": label, "tvshowtitle": title, "plot": data.get("description") or ""},
            sourceId=source_id,
            mediaId=media_id,
            episodeId=eid,
            playUrl=ep.get("playUrl") or "",
            playFrom=play_from,
        )
    xbmcplugin.setContent(handle, "episodes")
    xbmcplugin.endOfDirectory(handle)


def show_search(handle: int, base_url: str, addon, api: APIClient, keyword: str, page: int) -> None:
    if not keyword:
        keyboard = xbmcgui.Dialog().input(
            addon.getLocalizedString(30013) or "Enter search keyword",
            type=xbmcgui.INPUT_ALPHANUM,
        )
        keyword = (keyboard or "").strip()
        if not keyword:
            xbmcplugin.endOfDirectory(handle, succeeded=False)
            return

    try:
        data = api.search(keyword, page=page)
    except APIError:
        xbmcplugin.endOfDirectory(handle, succeeded=False)
        return

    items = data.get("items") or []
    for item in items:
        mid = item.get("id") or ""
        sid = item.get("sourceId") or ""
        title = item.get("title") or mid
        src_name = item.get("sourceName") or sid
        cover = item.get("coverUrl") or ""
        art = {"thumb": cover, "poster": cover} if cover else None
        label = "[%s] %s" % (src_name, title)
        _add_folder(
            handle,
            base_url,
            label,
            "detail",
            sourceId=sid,
            mediaId=mid,
            art=art,
            info={"title": title, "plot": src_name},
        )

    if items:
        _add_folder(
            handle,
            base_url,
            addon.getLocalizedString(30012) or "Next page",
            "search",
            keyword=keyword,
            page=str(page + 1),
        )
    xbmcplugin.setContent(handle, "movies")
    xbmcplugin.endOfDirectory(handle)


def show_live_groups(handle: int, base_url: str, api: APIClient) -> None:
    try:
        data = api.live_groups()
    except APIError:
        xbmcplugin.endOfDirectory(handle, succeeded=False)
        return
    for g in data.get("groups") or []:
        gid = g.get("id") or ""
        name = g.get("name") or gid
        count = g.get("channelCount")
        label = "%s (%s)" % (name, count) if count is not None else name
        _add_folder(handle, base_url, label, "live_channels", group=gid)
    xbmcplugin.endOfDirectory(handle)


def show_live_channels(handle: int, base_url: str, api: APIClient, group: str) -> None:
    try:
        data = api.live_channels(group=group)
    except APIError:
        xbmcplugin.endOfDirectory(handle, succeeded=False)
        return
    for ch in data.get("channels") or []:
        name = ch.get("name") or ch.get("id") or "channel"
        logo = ch.get("logoUrl") or ""
        art = {"thumb": logo, "icon": logo} if logo else None
        headers = ch.get("headers") or {}
        headers_json = ""
        if headers:
            import json

            headers_json = json.dumps(headers, ensure_ascii=False)
        _add_playable(
            handle,
            base_url,
            name,
            art=art,
            info={"title": name},
            sourceId="__live__",
            playUrl=ch.get("url") or "",
            headersJson=headers_json,
        )
    xbmcplugin.setContent(handle, "videos")
    xbmcplugin.endOfDirectory(handle)
