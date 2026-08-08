# -*- coding: utf-8 -*-
from __future__ import annotations

import sys
from urllib.parse import parse_qsl

import xbmcaddon
import xbmcplugin

from resources.lib.router import dispatch


def run():
    handle = int(sys.argv[1])
    addon = xbmcaddon.Addon()
    qs = sys.argv[2][1:] if len(sys.argv) > 2 and sys.argv[2].startswith("?") else ""
    params = dict(parse_qsl(qs, keep_blank_values=True))
    try:
        dispatch(handle, addon, params)
    except Exception as exc:  # noqa: BLE001 — surface to Kodi notification, avoid silent fail
        import xbmcgui

        xbmcgui.Dialog().notification(
            addon.getLocalizedString(30000) or "FnKodi TVBox",
            str(exc),
            xbmcgui.NOTIFICATION_ERROR,
            5000,
        )
        xbmcplugin.endOfDirectory(handle, succeeded=False)


if __name__ == "__main__":
    run()
