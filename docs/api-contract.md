# FnKodi_TVBOX — Go 网关 ↔ Kodi 插件 HTTP 契约

> 版本：v1（锁定）  
> 服务：`plugins/fn-tvbox-gateway`  
> 默认监听：`127.0.0.1:18765`（仅容器内本机；不对外暴露也可）  
> Content-Type：`application/json; charset=utf-8`  
> 错误体统一：`{"error":{"code":"string","message":"string"}}`

Kodi 插件 `plugin.video.fntvbox` **只允许**通过本契约访问内容与播放解析，**禁止**在 Python 内直接爬站或解析 TVBox 订阅。

参考语义来源（只读）：`bao-tv-box-master/src/shared/media.ts`、`contentSourceRoutes.ts`、`playerRoutes.ts`。  
本项目 **删除** Android Spider / DRPY / Chromium / mpv 相关字段与接口。

---

## 0. 通用约定

### 0.1 环境变量

| 变量 | 默认 | 说明 |
|------|------|------|
| `FNTVBOX_LISTEN` | `127.0.0.1:18765` | 监听地址 |
| `FNTVBOX_SUBSCRIPTION_URL` | （空） | TVBox 单仓/多仓/直播订阅 URL |
| `FNTVBOX_DATA_DIR` | `/var/lib/fn-tvbox` | 缓存与偏好持久化目录 |
| `FNTVBOX_CACHE_TTL_SEC` | `300` | 订阅 JSON 缓存秒数 |
| `FNTVBOX_HTTP_TIMEOUT_MS` | `8000` | 上游 HTTP 超时 |
| `FNTVBOX_USER_AGENT` | `FnKodiTVBox/1.0` | 默认 UA |

### 0.2 站点类型支持矩阵（V1）

| TVBox `site.type` | 含义 | V1 |
|-------------------|------|-----|
| 0 | CMS XML | 支持 |
| 1 | CMS JSON | 支持 |
| 2 | CMS JSON（变体） | 支持 |
| 3 + `csp_*` / jar | Android JAR Spider | **不支持**，加载时跳过并记日志 |
| 3 + `drpy2.js` | DRPY | **不支持**，跳过 |
| 4 | T4 HTTP Spider | 支持 |

### 0.3 ID 规则

- `sourceId`：稳定字符串，推荐 `sub_{subscriptionHash}_{site.key}`；单订阅可简化为 `site.key`。
- `mediaId`：源站影片 ID，原样透传（URL encode 后出现在 query）。
- `episodeId`：`{playFrom}|{index}` 或网关生成的稳定 hash；插件播放时原样回传。

### 0.4 分页

列表与搜索统一：

```json
{
  "items": [],
  "page": 1,
  "pageCount": 10,
  "total": 100
}
```

`page` 从 **1** 开始。未知总数时 `total`/`pageCount` 可省略，客户端以「本页为空」为结束。

---

## 1. 健康检查

### `GET /health`

**200**

```json
{
  "ok": true,
  "version": "0.1.0",
  "subscriptionConfigured": true
}
```

---

## 2. 订阅管理

### `GET /api/subscription`

返回当前订阅摘要（不含完整 sites 原始 JSON）。

```json
{
  "url": "https://example.com/tvbox.json",
  "kind": "single",
  "loadedAt": "2026-08-08T10:00:00Z",
  "siteCount": 12,
  "skippedUnsupported": 3,
  "liveCount": 2,
  "parseCount": 5
}
```

`kind`：`single` | `warehouse` | `live`。

### `PUT /api/subscription`

```json
{ "url": "https://example.com/tvbox.json" }
```

**200**：同 `GET /api/subscription`。  
**400**：URL 无效。  
副作用：清空订阅缓存并异步预热。

### `POST /api/subscription/reload`

强制重新拉取。**200**：同 GET。

---

## 3. 源（Sites）

### `GET /api/sources`

```json
{
  "sources": [
    {
      "id": "dy1",
      "name": "示例影视",
      "type": "cms",
      "runtime": "http",
      "enabled": true,
      "hidden": false,
      "contentKind": "vod",
      "quickSearch": true,
      "capabilities": {
        "detail": true,
        "headers": true,
        "live": false,
        "play": true,
        "search": true,
        "requiresProxy": false,
        "audiobook": false,
        "music": false
      }
    }
  ]
}
```

字段约束：

- `type`：仅 `cms` | `t4` | `unsupported`（列表默认不返回 unsupported）。
- `runtime`：V1 固定 `http`。
- `contentKind`：`vod` | `live` | `search` | `utility`（片库只展示 `vod`）。

### `PUT /api/sources/{sourceId}/preference`

```json
{ "enabled": true, "favorite": false, "pinned": false, "group": "" }
```

**200**：更新后的 `Source`。

---

## 4. 分类 / 列表 / 详情 / 搜索

### `GET /api/sources/{sourceId}/categories`

```json
{
  "categories": [
    {
      "id": "1",
      "sourceId": "dy1",
      "name": "电影",
      "folder": false,
      "filters": [
        {
          "key": "class",
          "name": "类型",
          "options": [{ "label": "全部", "value": "" }, { "label": "动作", "value": "动作" }]
        }
      ]
    }
  ]
}
```

### `GET /api/sources/{sourceId}/media`

Query：

| 参数 | 必填 | 说明 |
|------|------|------|
| `categoryId` | 是 | 分类 ID |
| `page` | 否 | 默认 1 |
| `filters` | 否 | JSON 对象 URL-encode，如 `%7B%22class%22%3A%22动作%22%7D` |

**200**：`MediaPage`。

`MediaItem`：

```json
{
  "id": "12345",
  "sourceId": "dy1",
  "title": "示例电影",
  "subtitle": "2024",
  "coverUrl": "https://...",
  "description": "",
  "tags": ["动作"],
  "year": "2024",
  "rating": 8.1,
  "kind": "media"
}
```

### `GET /api/sources/{sourceId}/detail?mediaId=`

**200**

```json
{
  "id": "12345",
  "sourceId": "dy1",
  "title": "示例电影",
  "coverUrl": "https://...",
  "description": "...",
  "tags": ["动作"],
  "year": "2024",
  "actors": "张三",
  "director": "李四",
  "area": "大陆",
  "remarks": "更新至10集",
  "episodes": [
    {
      "id": "线路1|0",
      "mediaId": "12345",
      "title": "第1集",
      "playFrom": "线路1",
      "playUrl": "https://play.example/1.m3u8"
    }
  ]
}
```

说明：`playUrl` 可能是直链、待解析页、或源站私有串；**播放前必须**走 `/api/player/resolve`。

### `GET /api/sources/{sourceId}/search?keyword=&page=&quick=`

- `quick=1`：快搜（尊重源 `quickSearch`）。
- **200**：`MediaPage`（items 为 `MediaItem`）。

### `GET /api/search?keyword=&page=`

聚合搜索（仅 `runtime=http` 且 searchable 的源；并发上限默认 10）。

```json
{
  "keyword": "流浪",
  "page": 1,
  "searchedSourceCount": 8,
  "failedSourceCount": 1,
  "items": [
    {
      "id": "123",
      "sourceId": "dy1",
      "sourceName": "示例影视",
      "title": "流浪地球",
      "coverUrl": "https://...",
      "tags": [],
      "matchKind": "contains",
      "matchScore": 80
    }
  ]
}
```

---

## 5. 直播

### `GET /api/live/groups`

```json
{
  "groups": [
    { "id": "央视", "name": "央视", "channelCount": 15 }
  ]
}
```

### `GET /api/live/channels?group=&keyword=`

```json
{
  "channels": [
    {
      "id": "cctv1",
      "name": "CCTV1",
      "group": "央视",
      "url": "https://example/live.m3u8",
      "logoUrl": "https://...",
      "headers": { "User-Agent": "..." },
      "parse": 0,
      "lines": []
    }
  ]
}
```

直播播放：若 `parse!=0` 或需要 headers 代理，同样走 `POST /api/player/resolve`（`sourceId` 使用保留值 `__live__`，`playUrl` 为频道 url）。

---

## 6. 播放解析

### `POST /api/player/resolve`

Request：

```json
{
  "sourceId": "dy1",
  "mediaId": "12345",
  "episodeId": "线路1|0",
  "playUrl": "https://...",
  "playFrom": "线路1"
}
```

直播时可：

```json
{
  "sourceId": "__live__",
  "playUrl": "https://example/live.m3u8",
  "headers": { "Referer": "https://..." }
}
```

**200 `ResolvedPlay`**

```json
{
  "url": "https://cdn.example/final.m3u8",
  "headers": {
    "User-Agent": "...",
    "Referer": "..."
  },
  "format": "hls",
  "parse": 0,
  "positionSeconds": 0,
  "subtitles": [],
  "danmaku": []
}
```

插件行为：

1. 调用本接口拿到 `url` + `headers`。
2. 若存在 `headers` 且 Kodi 无法直接带齐，则改用 `GET /api/proxy/play?token=...`（见下）。
3. 使用 `xbmcplugin.setResolvedUrl` 交给 Kodi 播放（**禁止** `xbmc.Player().play` 作为主路径）。

### `POST /api/proxy/session`（可选但 V1 建议实现）

为带 headers 的播放创建短时会话。

Request：`ResolvedPlay` 或 `{ "url", "headers" }`  
Response：

```json
{
  "playUrl": "http://127.0.0.1:18765/api/proxy/play/TOKEN",
  "expiresAt": "2026-08-08T10:05:00Z"
}
```

### `GET /api/proxy/play/{token}`

网关按会话 headers 回源并流式转发。仅本机可访问。

---

## 7. 错误码

| code | HTTP | 含义 |
|------|------|------|
| `bad_request` | 400 | 参数错误 |
| `not_found` | 404 | 源/影片不存在 |
| `unsupported_site` | 422 | 站点类型不支持 |
| `upstream_error` | 502 | 上游失败 |
| `upstream_timeout` | 504 | 上游超时 |
| `resolve_failed` | 502 | 播放解析失败 |
| `internal` | 500 | 内部错误 |

---

## 8. 插件调用顺序（必须遵守）

```text
启动 → GET /health
设置订阅 → PUT /api/subscription
首页源列表 → GET /api/sources
进源 → GET .../categories
进分类 → GET .../media
详情 → GET .../detail
点集数 → POST /api/player/resolve → setResolvedUrl
直播 → GET /api/live/groups → channels → resolve → setResolvedUrl
搜索 → GET /api/search 或单源 search
```

---

## 9. 明确不在 V1 契约内

- `/api/sources/*/actions*`（Android 交互）
- DRPY proxy / browser-stream / mpv 控制
- 网盘扫码登录挑战（可后续扩展 `CloudDriveLoginChallenge`）
- 任何 Android / JAR 相关接口
