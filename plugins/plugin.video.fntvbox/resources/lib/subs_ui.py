# -*- coding: utf-8 -*-
from __future__ import annotations

from typing import Any, Dict, List, Optional

import xbmc
import xbmcgui
import xbmcplugin

from .api_client import APIClient, APIError
from .list_ui import _add_folder


def _kind_label(addon, kind: str, parent_id: str) -> str:
    if parent_id:
        return addon.getLocalizedString(30034) or "Child"
    if kind == "warehouse":
        return addon.getLocalizedString(30032) or "Warehouse"
    if kind == "live":
        return addon.getLocalizedString(30033) or "Live"
    return addon.getLocalizedString(30031) or "Single"


def _parent_name(items: List[Dict[str, Any]], parent_id: str) -> str:
    for s in items:
        if s.get("id") == parent_id:
            return s.get("name") or parent_id
    return "Warehouse"


def show_subscriptions(handle: int, base_url: str, addon, api: APIClient) -> None:
    try:
        data = api.list_subscriptions()
    except APIError:
        xbmcplugin.endOfDirectory(handle, succeeded=False)
        return

    items: List[Dict[str, Any]] = data.get("subscriptions") or []
    _add_folder(
        handle,
        base_url,
        addon.getLocalizedString(30035) or "Add subscription",
        "sub_add",
    )

    for sub in items:
        sid = sub.get("id") or ""
        name = sub.get("name") or sid
        kind = sub.get("kind") or "single"
        parent_id = sub.get("parentId") or ""
        enabled = bool(sub.get("enabled"))
        label_kind = _kind_label(addon, kind, parent_id)
        status = (
            (addon.getLocalizedString(30036) or "On")
            if enabled
            else (addon.getLocalizedString(30037) or "Off")
        )
        label = "[%s] %s · %s" % (label_kind, name, status)
        if parent_id:
            label += " · %s %s" % (
                addon.getLocalizedString(30038) or "from",
                _parent_name(items, parent_id),
            )
        plot = "%s\n%s" % (sub.get("url") or "", sub.get("healthStatus") or "")
        _add_folder(
            handle,
            base_url,
            label,
            "sub_item",
            subId=sid,
            info={"title": name, "plot": plot},
        )
    xbmcplugin.endOfDirectory(handle)


def show_subscription_item(handle: int, base_url: str, addon, api: APIClient, sub_id: str) -> None:
    try:
        data = api.list_subscriptions()
    except APIError:
        xbmcplugin.endOfDirectory(handle, succeeded=False)
        return
    sub: Optional[Dict[str, Any]] = None
    for s in data.get("subscriptions") or []:
        if s.get("id") == sub_id:
            sub = s
            break
    if not sub:
        xbmcgui.Dialog().notification(
            addon.getLocalizedString(30000) or "FnKodi TVBox",
            addon.getLocalizedString(30039) or "Subscription not found",
            xbmcgui.NOTIFICATION_ERROR,
            4000,
        )
        xbmcplugin.endOfDirectory(handle, succeeded=False)
        return

    enabled = bool(sub.get("enabled"))
    _add_folder(
        handle,
        base_url,
        addon.getLocalizedString(30040) or "Sync now",
        "sub_sync",
        subId=sub_id,
    )
    _add_folder(
        handle,
        base_url,
        addon.getLocalizedString(30041) or "Test connection",
        "sub_test",
        subId=sub_id,
    )
    toggle_label = (
        (addon.getLocalizedString(30042) or "Disable")
        if enabled
        else (addon.getLocalizedString(30043) or "Enable")
    )
    _add_folder(
        handle,
        base_url,
        toggle_label,
        "sub_toggle",
        subId=sub_id,
    )
    _add_folder(
        handle,
        base_url,
        addon.getLocalizedString(30044) or "Delete",
        "sub_delete",
        subId=sub_id,
    )
    xbmcplugin.endOfDirectory(handle)


def do_add(addon, api: APIClient) -> None:
    title = addon.getLocalizedString(30000) or "FnKodi TVBox"
    keyboard = xbmc.Keyboard(
        "",
        addon.getLocalizedString(30045) or "Subscription / warehouse URL",
    )
    keyboard.doModal()
    if not keyboard.isConfirmed():
        return
    url = (keyboard.getText() or "").strip()
    if not url:
        return
    try:
        data = api.add_subscription(url)
        sub = data.get("subscription") or {}
        kind = sub.get("kind") or ""
        kids = [
            s
            for s in (data.get("subscriptions") or [])
            if s.get("parentId") == sub.get("id")
        ]
        if kind == "warehouse":
            msg = (addon.getLocalizedString(30046) or "Warehouse added with %d children") % len(
                kids
            )
        else:
            msg = addon.getLocalizedString(30047) or "Subscription added"
        xbmcgui.Dialog().notification(title, msg, xbmcgui.NOTIFICATION_INFO, 4000)
    except APIError:
        return


def do_sync(addon, api: APIClient, sub_id: str) -> None:
    title = addon.getLocalizedString(30000) or "FnKodi TVBox"
    try:
        data = api.sync_subscription(sub_id)
        if data.get("ok") is False:
            xbmcgui.Dialog().notification(
                title,
                data.get("error") or (addon.getLocalizedString(30026) or "Request failed"),
                xbmcgui.NOTIFICATION_ERROR,
                5000,
            )
            return
        xbmcgui.Dialog().notification(
            title,
            addon.getLocalizedString(30048) or "Sync complete",
            xbmcgui.NOTIFICATION_INFO,
            3000,
        )
    except APIError:
        return


def do_test(addon, api: APIClient, sub_id: str) -> None:
    title = addon.getLocalizedString(30000) or "FnKodi TVBox"
    try:
        data = api.test_subscription(sub_id)
        probe = data.get("probe") or {}
        if probe.get("ok"):
            msg = "%s · %s (%s)" % (
                addon.getLocalizedString(30049) or "OK",
                probe.get("detectedKind") or "",
                probe.get("sourceCount"),
            )
            xbmcgui.Dialog().notification(title, msg, xbmcgui.NOTIFICATION_INFO, 4000)
        else:
            xbmcgui.Dialog().notification(
                title,
                probe.get("message") or (addon.getLocalizedString(30026) or "Request failed"),
                xbmcgui.NOTIFICATION_ERROR,
                5000,
            )
    except APIError:
        return


def do_toggle(addon, api: APIClient, sub_id: str) -> None:
    try:
        data = api.list_subscriptions()
    except APIError:
        return
    enabled = True
    for s in data.get("subscriptions") or []:
        if s.get("id") == sub_id:
            enabled = bool(s.get("enabled"))
            break
    try:
        api.patch_subscription(sub_id, enabled=not enabled)
        title = addon.getLocalizedString(30000) or "FnKodi TVBox"
        xbmcgui.Dialog().notification(
            title,
            addon.getLocalizedString(30050) or "Updated",
            xbmcgui.NOTIFICATION_INFO,
            2500,
        )
    except APIError:
        return


def do_delete(addon, api: APIClient, sub_id: str) -> None:
    title = addon.getLocalizedString(30000) or "FnKodi TVBox"
    if not xbmcgui.Dialog().yesno(
        title,
        addon.getLocalizedString(30051) or "Delete this subscription?",
    ):
        return
    try:
        api.delete_subscription(sub_id)
        xbmcgui.Dialog().notification(
            title,
            addon.getLocalizedString(30052) or "Deleted",
            xbmcgui.NOTIFICATION_INFO,
            2500,
        )
    except APIError:
        return
