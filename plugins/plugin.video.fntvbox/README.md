# plugin.video.fntvbox

Kodi 视频插件（V1 主 UI）。在默认 Estuary 皮肤下即可完成源浏览、搜索、点播与直播。

- 仅通过 HTTP 调用本机 `fn-tvbox-gateway`（默认 `http://127.0.0.1:18765`）。
- **禁止**在 Python 内爬站或解析 TVBox 订阅。
- 启动时校验网关 `apiVersion`（V1 仅认 `"v1"`）。
- 播放：`POST /api/player/resolve` →（如需 headers）`POST /api/proxy/session` → `xbmcplugin.setResolvedUrl`。

设置项：网关地址、订阅 URL（进入插件时若变更会 `PUT /api/subscription`）。

打包：

```bash
bash scripts/package-addons.sh
# → release/addons/plugin.video.fntvbox/
# → release/addons/plugin.video.fntvbox-<VERSION>.zip
```

契约见 [`docs/api-contract.md`](../../docs/api-contract.md)。
