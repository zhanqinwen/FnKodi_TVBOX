# -*- coding: utf-8 -*-
from __future__ import annotations

from typing import Any, Dict

import sys

import xbmcplugin

from .api_client import APIClient
from . import list_ui
from . import subs_ui
from .play import play_item


def _base_url() -> str:
    return sys.argv[0]


def dispatch(handle: int, addon, params: Dict[str, Any]) -> None:
    api = APIClient(addon)
    if not api.ensure_api_version():
        xbmcplugin.endOfDirectory(handle, succeeded=False)
        return

    action = (params.get("action") or "").strip()
    base_url = _base_url()

    if action == "":
        list_ui.show_root(handle, base_url, addon, api)
        return
    if action == "subscriptions":
        subs_ui.show_subscriptions(handle, base_url, addon, api)
        return
    if action == "sub_item":
        subs_ui.show_subscription_item(
            handle, base_url, addon, api, params.get("subId") or ""
        )
        return
    if action == "sub_add":
        subs_ui.do_add(addon, api)
        xbmcplugin.endOfDirectory(handle, succeeded=True, updateListing=True)
        return
    if action == "sub_sync":
        subs_ui.do_sync(addon, api, params.get("subId") or "")
        xbmcplugin.endOfDirectory(handle, succeeded=True, updateListing=True)
        return
    if action == "sub_test":
        subs_ui.do_test(addon, api, params.get("subId") or "")
        xbmcplugin.endOfDirectory(handle, succeeded=True, updateListing=True)
        return
    if action == "sub_toggle":
        subs_ui.do_toggle(addon, api, params.get("subId") or "")
        xbmcplugin.endOfDirectory(handle, succeeded=True, updateListing=True)
        return
    if action == "sub_delete":
        subs_ui.do_delete(addon, api, params.get("subId") or "")
        xbmcplugin.endOfDirectory(handle, succeeded=True, updateListing=True)
        return
    if action == "categories":
        list_ui.show_categories(handle, base_url, api, params.get("sourceId") or "")
        return
    if action == "media":
        page = int(params.get("page") or "1")
        list_ui.show_media(
            handle,
            base_url,
            addon,
            api,
            params.get("sourceId") or "",
            params.get("categoryId") or "",
            page,
        )
        return
    if action == "detail":
        list_ui.show_detail(
            handle,
            base_url,
            api,
            params.get("sourceId") or "",
            params.get("mediaId") or "",
        )
        return
    if action == "search":
        page = int(params.get("page") or "1")
        list_ui.show_search(
            handle,
            base_url,
            addon,
            api,
            params.get("keyword") or "",
            page,
        )
        return
    if action == "live_groups":
        list_ui.show_live_groups(handle, base_url, api)
        return
    if action == "live_channels":
        list_ui.show_live_channels(handle, base_url, api, params.get("group") or "")
        return
    if action == "play":
        play_item(handle, addon, api, params)
        return

    xbmcplugin.endOfDirectory(handle, succeeded=False)
