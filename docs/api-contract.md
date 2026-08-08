# FnKodi_TVBOX — Go 网关 ↔ Kodi 插件 HTTP 契约

> **契约版本（apiVersion）：`v1`（锁定）**  
> 发行版本见仓库根 `VERSION`（如 `0.1.0`），与 `apiVersion` 独立。  
> 破坏兼容的变更必须升 `apiVersion`（v1→v2），并同步改本文件与插件校验逻辑。  
> 服务：`plugins/fn-tvbox-gateway`  
> 默认监听：`127.0.0.1:18765`（仅容器内本机；**禁止** compose 映射到宿主机）  
> Content-Type：`application/json; charset=utf-8`  
> 错误体统一：`{"error":{"code":"string","message":"string"}}`

Kodi 插件 `plugin.video.fntvbox` **只允许**通过本契约访问内容与播放解析，**禁止**在 Python 内直接爬站或解析 TVBox 订阅。

参考语义来源（只读）：`bao-tv-box-master/src/shared/media.ts`、`contentSourceRoutes.ts`、`playerRoutes.ts`、`tvbox2.ts`（剧集拆分）。  
本项目 **删除** Android Spider / DRPY / Chromium / mpv 相关字段与接口。

---

## 0. 通用约定

### 0.1 环境变量

| 变量 | 默认 | 说明 |
|------|------|------|
| `FNTVBOX_LISTEN` | `127.0.0.1:18765` | 监听地址（必须本机回环） |
| `FNTVBOX_SUBSCRIPTION_URL` | （空） | TVBox 单仓/多仓/直播订阅 URL |
| `FNTVBOX_DATA_DIR` | `/var/lib/fn-tvbox` | 缓存与偏好持久化目录 |
| `FNTVBOX_CACHE_TTL_SEC` | `300` | 订阅 JSON 缓存秒数 |
| `FNTVBOX_MEDIA_CACHE_TTL_SEC` | `900` | 分类/列表响应 JSON 内存缓存秒数 |
| `FNTVBOX_HTTP_TIMEOUT_MS` | `8000` | **短请求**整体超时：订阅/CMS/T4/parses/元数据/搜索（聚合搜索每源同此） |
| `FNTVBOX_PROXY_HEADER_TIMEOUT_MS` | `15000` | **仅媒体代理**回源的 ResponseHeaderTimeout（首字节/响应头）；**不是**整段播放时长超时 |
| `FNTVBOX_USER_AGENT` | `FnKodiTVBox/1.0` | 默认 UA |

#### 超时使用规则（强制）

1. 短请求使用带 `Timeout=FNTVBOX_HTTP_TIMEOUT_MS` 的 `http.Client`。  
2. `GET /api/proxy/play/{token}` 必须使用**独立** Client：  
   - 设置 `Transport.ResponseHeaderTimeout = FNTVBOX_PROXY_HEADER_TIMEOUT_MS`；  
   - **禁止**设置会覆盖整个 body 传输的 `Client.Timeout`；  
   - body 流式转发直到 EOF 或下游断开。  
3. 禁止两个用途共用同一个带整体 `Timeout` 的 Client（会导致长视频播放被腰斩）。

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

- `sourceId`：稳定字符串 `sub_{subscriptionId前8位}_{site.key}`（多订阅/多仓合并时带前缀，避免 key 冲突）。  
- `mediaId`：源站影片 ID，原样透传（URL encode 后出现在 query）。  
- `episodeId`：**不**嵌入 `playFrom` 原文，避免 `|` / 特殊字符冲突。

#### episodeId 格式（锁定）

```text
episodeId = "{groupIndex}:{episodeIndex}"
```

- `groupIndex`、`episodeIndex` 均为从 **0** 开始的十进制整数。  
- `playFrom`、`title`、`playUrl` 始终作为独立字段返回/回传；插件播放时 `episodeId` 原样回传。  
- **禁止**使用 `playFrom|index` 作为 id（旧草案作废）。

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

### 0.5 CMS 剧集字符串拆分规则（锁定）

对齐 bao-tv-box / TVBox 常见约定（实现见 `tvbox2.ts` 语义，Go 重写）：

| 分隔符 | 层级 | 说明 |
|--------|------|------|
| `$$$` | 线路分组 | 分隔 `vod_play_from` 各组名，以及 `vod_play_url` 各组剧集串 |
| `#` | 集 | 在同一线路组内分隔各集条目 |
| `$` | 集内字段 | `标题$播放地址`；仅分割为 **两段**（第一个 `$`） |

伪代码：

```text
fromGroups = split(vod_play_from, "$$$")
urlGroups  = split(vod_play_url,  "$$$")
for g, group in urlGroups:
  playFrom = fromGroups[g] or fromGroups[0] or ("线路" + str(g+1))
  for e, entry in split(group, "#"):
    if entry empty: skip
    if entry contains "$":
      title, url = split_first(entry, "$")
    else:
      skip or treat as url-only（实现选一种，单测写死）
    emit Episode{ id: f"{g}:{e}", playFrom, title, playUrl: url }
```

边界与单测清单见《实现路线与部署》P2.7（E01–E09）。集标题或线路名中若字面含 `$$$`/`#`，按上表硬分隔（与主流 TVBox 源站约定一致）；**不要**发明百分号转义，以免与源站不兼容。稳定性靠 `episodeId={g}:{e}`，不靠拼接标题。

---

## 1. 健康检查

### `GET /health`

**200**

```json
{
  "ok": true,
  "version": "0.1.0",
  "apiVersion": "v1",
  "subscriptionConfigured": true
}
```

| 字段 | 说明 |
|------|------|
| `version` | 网关发行版本（来自 `VERSION` / ldflags） |
| `apiVersion` | **契约版本**，固定 `"v1"` 直至破坏兼容 |
| `subscriptionConfigured` | 是否配置了订阅 URL |

插件启动必须校验 `apiVersion`；缺失或不支持则提示用户，禁止静默继续。

---

## 2. 订阅管理

支持 **多条订阅**（对齐 bao-tv-box 多仓模型）：

- `kind`：`single` | `warehouse` | `live`
- `warehouse` 父项只存索引 URL；`POST .../sync` 或添加时展开为带 `parentId` 的子仓（`kind=single`）
- 内容聚合只加载 `enabled && kind != warehouse` 的项；父仓启停联动子仓
- 持久化：`{FNTVBOX_DATA_DIR}/subscriptions.json`（环境变量 `FNTVBOX_SUBSCRIPTION_URL` 仅作首次引导）

### `GET /api/subscription`

返回**聚合摘要**（不含完整 sites 原始 JSON；完整列表见 `GET /api/subscriptions`）。

**成功且无错误：**

```json
{
  "url": "https://example.com/tvbox.json",
  "kind": "warehouse",
  "loadedAt": "2026-08-08T10:00:00Z",
  "siteCount": 12,
  "skippedUnsupported": 3,
  "liveCount": 2,
  "parseCount": 5,
  "lastError": null,
  "subscriptionCount": 5,
  "childCount": 4
}
```

**拉取/解析失败（锁定行为）：**

- HTTP **仍为 200**（便于插件统一解析）。  
- 若存在上次成功缓存：保留 `loadedAt` / 计数等摘要字段，并设置 `lastError`。  
- 若从未成功：计数可为 0，`loadedAt` 可省略或 null，必须带 `lastError`。  
- URL 未配置：`subscriptionConfigured` 在 health 为 false；本接口可返回空摘要 + `lastError` 或等价明确状态。

`lastError.code` 建议复用第 7 节错误码：`upstream_error` | `upstream_timeout` | `bad_request` | `internal` 等。

插件逻辑：

- `siteCount==0` 且 `lastError!=null` → 提示「订阅拉取/解析失败」。  
- `siteCount==0` 且 `skippedUnsupported>0` 且无 lastError → 提示「订阅内无可支持源（可能全是 JAR/DRPY）」。  
- `siteCount==0` 且二者皆无 → 提示「订阅为空或未配置」。

### `PUT /api/subscription`

```json
{ "url": "https://example.com/tvbox.json" }
```

**200**：同 `GET /api/subscription`（可能含 `lastError`）。  
**400**：URL 无效（语法层，不发起拉取也可判定）。  
副作用：**upsert** 顶层订阅（同 URL 保留 id；多仓则自动 sync 展开子仓）；**不删除**其它已有订阅；清空目录缓存并预热。

### `POST /api/subscription/reload`

强制重新拉取全部启用内容订阅。**200**：同 GET（含可能的 `lastError`）。

### `GET /api/subscriptions`

```json
{
  "subscriptions": [
    {
      "id": "subscription-abc",
      "name": "Noimank",
      "url": "https://example.com/tvboxmuti.json",
      "kind": "warehouse",
      "enabled": true,
      "healthStatus": "healthy",
      "lastSyncAt": "2026-08-08T10:00:00Z"
    },
    {
      "id": "subscription-abc-child-deadbeefcafe",
      "name": "FongMI",
      "url": "https://example.com/0827.json",
      "kind": "single",
      "enabled": true,
      "healthStatus": "unknown",
      "parentId": "subscription-abc"
    }
  ]
}
```

### `POST /api/subscriptions`

```json
{ "url": "https://example.com/tvboxmuti.json", "name": "可选名称" }
```

探测 kind → 保存；若为 `warehouse` 立即 sync 展开子仓。  
**200**：`{ subscription, subscriptions }`  
**400** / **409**（URL 已存在）/ **502**（探测失败）。

### `POST /api/subscriptions/probe`

```json
{ "url": "https://example.com/tvboxmuti.json" }
```

**200**：`{ ok, detectedKind, sourceCount, name?, message? }`（多仓时 `sourceCount` 为子仓数）。

### `PATCH /api/subscriptions/{id}`

```json
{ "enabled": false, "name": "新名称" }
```

父仓改 `enabled` 时联动全部子仓。**200**：`{ subscription, subscriptions }`。

### `DELETE /api/subscriptions/{id}`

删除订阅；删父仓连带删除子仓。**200**：`{ ok: true, subscriptions }`。

### `POST /api/subscriptions/{id}/sync`

多仓：拉取索引并 reconcile 子仓（同 URL 保留用户 `enabled`）。  
单仓/直播：刷新健康状态。  
**200**：`{ ok, subscription, subscriptions, error? }`。

### `POST /api/subscriptions/{id}/test`

连通性探测，**不** reconcile 子仓。**200**：`{ ok, probe, subscription }`。

多源合并时 `sourceId` 形如 `sub_{id8}_{site.key}`，避免子仓 key 冲突。

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
| `filters` | 否 | JSON 对象 URL-encode |

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
      "id": "0:0",
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
- **200**：`MediaPage`。

### `GET /api/search?keyword=&page=`

聚合搜索（仅 `runtime=http` 且 searchable 的源；并发上限默认 10；单源使用短请求超时）。

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
  "episodeId": "0:0",
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
2. 若存在 `headers` 且 Kodi 无法直接带齐，则改用 proxy play URL。  
3. 使用 `xbmcplugin.setResolvedUrl` 交给 Kodi 播放（**禁止** `xbmc.Player().play` 作为主路径）。

### `POST /api/proxy/session`（V1 建议实现）

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

网关按会话 headers 回源并流式转发。

约束：

- 仅本机可访问（依赖 `FNTVBOX_LISTEN=127.0.0.1` + compose **不**映射 18765）。  
- 回源超时策略见 §0.1（仅 header/TTFB 超时）。  
- 验收：宿主机 `docker port <container>` 不得出现 18765。

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
| `not_implemented` | 501 | 未实现 |
| `internal` | 500 | 内部错误 |

订阅失败时这些 code 出现在 `lastError.code`，此时 HTTP 仍为 200（见 §2）。

---

## 8. 插件调用顺序（必须遵守）

```text
启动 → GET /health（校验 apiVersion==v1）
设置订阅 → PUT /api/subscription
看摘要/失败态 → GET /api/subscription（读 lastError）
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
- 网盘扫码登录挑战（可后续扩展）  
- 任何 Android / JAR 相关接口  
- Skin 专用 API（Skin 只消费本契约已有接口）

---

## 10. 鉴权与暴露面

| 项 | V1 策略 |
|----|---------|
| 监听 | `127.0.0.1` only |
| compose ports | 禁止 `18765:18765` |
| 认证 | V1 不做 token 鉴权（依赖本机回环） |
| 调试临时映射 | 允许开发机临时映射，**交付前必须删除**；P7/P9 清单验收 |

---

**维护要求：** 改接口字段/行为必须改本文件；破坏兼容升 `apiVersion`。
