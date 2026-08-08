# -*- coding: utf-8 -*-
from __future__ import annotations

import json
from typing import Any, Dict, Optional
from urllib.error import HTTPError, URLError
from urllib.parse import quote, urlencode, urljoin
from urllib.request import Request, urlopen

import xbmcgui

SUPPORTED_API_VERSION = "v1"
DEFAULT_TIMEOUT = 8.0


class APIError(Exception):
    def __init__(self, code: str, message: str, http_status: int = 0):
        super().__init__(message)
        self.code = code
        self.message = message
        self.http_status = http_status


class APIClient:
    def __init__(self, addon):
        self._addon = addon
        base = (addon.getSetting("gateway_base") or "http://127.0.0.1:18765").strip()
        if not base.endswith("/"):
            base += "/"
        self.base = base
        self.timeout = DEFAULT_TIMEOUT

    def _notify(self, message: str, error: bool = True) -> None:
        title = self._addon.getLocalizedString(30000) or "FnKodi TVBox"
        icon = xbmcgui.NOTIFICATION_ERROR if error else xbmcgui.NOTIFICATION_INFO
        xbmcgui.Dialog().notification(title, message, icon, 4500)

    def _url(self, path: str, query: Optional[Dict[str, Any]] = None) -> str:
        path = path.lstrip("/")
        url = urljoin(self.base, path)
        if query:
            filtered = {k: v for k, v in query.items() if v is not None and v != ""}
            if filtered:
                url += "?" + urlencode(filtered, doseq=True, quote_via=quote)
        return url

    def _request(
        self,
        method: str,
        path: str,
        query: Optional[Dict[str, Any]] = None,
        body: Optional[Dict[str, Any]] = None,
        notify: bool = True,
    ) -> Any:
        data = None
        headers = {"Accept": "application/json"}
        if body is not None:
            data = json.dumps(body).encode("utf-8")
            headers["Content-Type"] = "application/json; charset=utf-8"
        req = Request(self._url(path, query), data=data, headers=headers, method=method)
        try:
            with urlopen(req, timeout=self.timeout) as resp:
                raw = resp.read()
                if not raw:
                    return {}
                return json.loads(raw.decode("utf-8"))
        except HTTPError as exc:
            payload = {}
            try:
                payload = json.loads(exc.read().decode("utf-8"))
            except Exception:  # noqa: BLE001
                payload = {}
            err = payload.get("error") or {}
            code = err.get("code") or "http_error"
            message = err.get("message") or str(exc.reason or exc)
            if notify:
                self._notify(message or self._addon.getLocalizedString(30026))
            raise APIError(code, message, exc.code) from exc
        except URLError as exc:
            msg = self._addon.getLocalizedString(30020) or "Gateway unreachable"
            detail = "%s: %s" % (msg, getattr(exc, "reason", exc))
            if notify:
                self._notify(detail)
            raise APIError("unreachable", detail) from exc
        except TimeoutError as exc:
            msg = self._addon.getLocalizedString(30026) or "Request failed"
            if notify:
                self._notify(msg)
            raise APIError("timeout", msg) from exc

    def health(self, notify: bool = True) -> Dict[str, Any]:
        return self._request("GET", "/health", notify=notify)

    def ensure_api_version(self) -> bool:
        """Return True if apiVersion is supported; otherwise notify and return False."""
        try:
            data = self.health(notify=True)
        except APIError:
            return False
        ver = data.get("apiVersion")
        if ver != SUPPORTED_API_VERSION:
            msg = self._addon.getLocalizedString(30021) or "API version mismatch"
            detail = "%s (got %r, need %r)" % (msg, ver, SUPPORTED_API_VERSION)
            xbmcgui.Dialog().ok(
                self._addon.getLocalizedString(30000) or "FnKodi TVBox",
                detail,
            )
            return False
        return True

    def get_subscription(self) -> Dict[str, Any]:
        return self._request("GET", "/api/subscription")

    def put_subscription(self, url: str) -> Dict[str, Any]:
        return self._request("PUT", "/api/subscription", body={"url": url})

    def sync_subscription_from_settings(self) -> None:
        url = (self._addon.getSetting("subscription_url") or "").strip()
        if not url:
            return
        try:
            current = self.get_subscription()
            if (current.get("url") or "").strip() == url and not current.get("lastError"):
                return
        except APIError:
            pass
        try:
            self.put_subscription(url)
        except APIError:
            # notification already shown
            return

    def sources(self) -> Dict[str, Any]:
        return self._request("GET", "/api/sources")

    def categories(self, source_id: str) -> Dict[str, Any]:
        return self._request("GET", "/api/sources/%s/categories" % quote(source_id, safe=""))

    def media(
        self,
        source_id: str,
        category_id: str,
        page: int = 1,
        filters: Optional[str] = None,
    ) -> Dict[str, Any]:
        q: Dict[str, Any] = {"categoryId": category_id, "page": page}
        if filters:
            q["filters"] = filters
        return self._request(
            "GET",
            "/api/sources/%s/media" % quote(source_id, safe=""),
            query=q,
        )

    def detail(self, source_id: str, media_id: str) -> Dict[str, Any]:
        return self._request(
            "GET",
            "/api/sources/%s/detail" % quote(source_id, safe=""),
            query={"mediaId": media_id},
        )

    def search(self, keyword: str, page: int = 1) -> Dict[str, Any]:
        return self._request("GET", "/api/search", query={"keyword": keyword, "page": page})

    def live_groups(self) -> Dict[str, Any]:
        return self._request("GET", "/api/live/groups")

    def live_channels(self, group: str = "", keyword: str = "") -> Dict[str, Any]:
        return self._request(
            "GET",
            "/api/live/channels",
            query={"group": group, "keyword": keyword},
        )

    def resolve(
        self,
        source_id: str,
        play_url: str,
        media_id: str = "",
        episode_id: str = "",
        play_from: str = "",
        headers: Optional[Dict[str, str]] = None,
    ) -> Dict[str, Any]:
        body: Dict[str, Any] = {
            "sourceId": source_id,
            "playUrl": play_url,
        }
        if media_id:
            body["mediaId"] = media_id
        if episode_id:
            body["episodeId"] = episode_id
        if play_from:
            body["playFrom"] = play_from
        if headers:
            body["headers"] = headers
        return self._request("POST", "/api/player/resolve", body=body)

    def proxy_session(self, url: str, headers: Optional[Dict[str, str]] = None) -> Dict[str, Any]:
        body: Dict[str, Any] = {"url": url}
        if headers:
            body["headers"] = headers
        return self._request("POST", "/api/proxy/session", body=body)
