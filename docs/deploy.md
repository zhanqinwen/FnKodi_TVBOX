# FnKodi_TVBOX — 部署手册（fnOS）

> 面向构建机与设备操作。实现路线见 [`实现路线与部署.md`](../实现路线与部署.md)；HTTP 契约见 [`api-contract.md`](api-contract.md)。

**发行版本来源：** 仓库根目录 `VERSION`（当前构建以该文件为准）。

---

## 1. 产物清单

在 **Linux 构建机**执行 `./scripts/build-all.sh` 后，`release/` 应包含：

| 产物 | 路径 | 说明 |
|------|------|------|
| 网关二进制 | `release/gateway/fn-tvbox-gateway` | linux/amd64，CGO 关闭 |
| 视频插件 | `release/addons/plugin.video.fntvbox/` | Kodi 插件 |
| 自定义皮肤 | `release/addons/skin.fntvbox/` | 默认皮肤，已打进镜像 |
| 镜像 tar | `release/images/fnkodi-tvbox_${VER}_amd64.tar.gz` | `docker load` 用 |
| FPK | `release/fpk/fnkodi-tvbox_all_v${VER}.fpk` | 应用中心安装 |
| 校验 | `release/SHA256SUMS` | 上述文件 sha256 |

镜像 tag（无 registry 前缀）：`fnkodi-tvbox:<VERSION>`，须与 FPK 内 `docker-compose.yaml` 的 `image:` **完全一致**。

---

## 2. Linux 构建

```bash
git clone <本仓URL> FnKodi_TVBOX
cd FnKodi_TVBOX
./scripts/build-all.sh
ls -lh release/fpk release/images
# 可选：sha256sum -c release/SHA256SUMS
```

顺序锁定：`require_linux` → gateway → addons（含 skin）→ Docker 镜像 → FPK → `SHA256SUMS`。

**Windows 开发机：** 只提交源码；`git push` 后 SSH 到构建机 `git pull && ./scripts/build-all.sh`，再取 `release/` 部署。

子脚本可单独运行：`build-gateway.sh`、`package-addons.sh`、`build-image.sh`、`build-fpk.sh`。

---

## 3. 设备准备

- [ ] 电视已用 HDMI 连接主机，输入源正确。
- [ ] fnOS 已开启 Docker / 应用中心第三方安装能力。
- [ ] 构建机已产出 FPK + 镜像 tar，并拷贝到设备可访问位置。

---

## 4. 导入镜像

```bash
# 将 VER 换成 VERSION 文件内容，例如 0.2.0
gunzip -c fnkodi-tvbox_${VER}_amd64.tar.gz | docker load
docker images | grep fnkodi-tvbox
```

确认 tag 与 `fpk/app/docker/docker-compose.yaml` 中 `image:` 一致。

**分发方式（锁定）：** `docker save` → 设备 `docker load`。不使用私有 registry。

---

## 5. 安装 FPK

1. 应用中心 → 安装本地 FPK（`fnkodi-tvbox_all_v${VER}.fpk`）。
2. 向导填写 **TVBox 订阅 URL**（建议先用自建仅 CMS 的测试接口）。
3. 网络模式：一般 `bridge`；需要 DLNA/AirPlay 再按说明改 `host`。
4. 完成安装并启动应用。

---

## 6. 首次启动检查

```bash
docker ps | grep fnkodi-tvbox
docker logs fnkodi-tvbox 2>&1 | tail -n 100
docker exec fnkodi-tvbox curl -sS http://127.0.0.1:18765/health
docker exec fnkodi-tvbox curl -sS http://127.0.0.1:18765/api/subscription
```

确认：

- 日志有 gateway 启动；addon/skin 同步与 gateway 看门狗可重叠启动。
- `/health` 返回 `"ok":true`、`"apiVersion":"v1"`。
- 人为 `kill` gateway 后数秒内复活（看门狗）。

---

## 7. HDMI / DRM 检查

```bash
ls -l /dev/dri
docker exec fnkodi-tvbox ls -l /dev/dri
```

仍无画面：对照官方 `fn-kodi` 同机是否正常；比较 devices/caps。

---

## 8. 功能验收清单（P9 完成标准）

- [ ] 电视上看到 Kodi；**默认皮肤为 `skin.fntvbox`**（首次 userdata）。
- [ ] 从首页/插件入口能进 `plugin.video.fntvbox`。
- [ ] 能看到订阅中的 CMS/T4 源（无 JAR 源）。
- [ ] 能打开分类与海报列表。
- [ ] 详情页能看到线路/集数。
- [ ] 点播可播放至少一条直链或可解析源。
- [ ] 直播分组/频道可打开并播放（若订阅含 lives）。
- [ ] 搜索能出结果（源站可用时）。
- [ ] 坏订阅时插件有明确错误提示（`lastError`）。
- [ ] 遥控器上下左右/返回/确认正常。
- [ ] 重启应用后订阅与 Kodi 配置仍在。
- [ ] **宿主机看不到 18765 端口**：`ss -lntp | grep 18765` 或 `docker port fnkodi-tvbox` 无 18765。
- [ ] `docker kill -s TERM` 后 gateway 不残留；再次启动看门狗仍工作。
- [ ] （可选回退）禁用/删除 Skin 后，插件在 Estuary 下仍可用。

### 8.1 性能抽样（P10，非阻塞安装）

- [ ] 冷启动到插件列表可操作 **&lt; 30s**（N200 + SSD，不含拉订阅）。
- [ ] 本地直链 resolve 网关开销 **&lt; 50ms**（抽样）。
- [ ] 长视频经 proxy 播放 **&gt; 1 分钟**不因超时中断（抽样）。
- [ ] 聚合搜索约 8 个 HTTP 源时 P95 可接受（仍取决于源站）。

---

## 9. 端口与远程

| 端口 | 用途 | 宿主机映射 |
|------|------|------------|
| 8080 | Kodi Web | 是 |
| 9090 | JSON-RPC | 是 |
| 9777/udp | 事件服务器 | 是 |
| 18765 | 网关 | **否（禁止）** |

Web 账号若沿用上游默认，注意修改密码。

---

## 10. 升级

1. 构建机升 `VERSION`，执行 `./scripts/build-all.sh`。
2. 设备 `docker load` 新镜像。
3. 应用中心升级 FPK。
4. 数据卷保留则用户数据还在。
5. 插件校验 `apiVersion`；不匹配则提示升级镜像或插件。

升级后若仍显示旧皮肤：数据卷可能缓存旧 addon；升 addon 版本或清理 `/root/.kodi/addons/skin.fntvbox`（容器内路径，对应 shares 卷）。

---

## 11. 卸载

- **保留数据：** 仅移除容器/应用入口。
- **删除数据：** 清理 `/var/apps/fnkodi-tvbox/shares/**`（对齐 fn-kodi）。

---

## 12. 常见问题

| 问题 | 原因 | 处理 |
|------|------|------|
| 源列表为空 | 订阅全是 type3 或订阅失败 | 看 `lastError` / `skippedUnsupported`；换订阅 |
| 能列不能播 | resolve 失败/防盗链 | 看 gateway 日志；检查 proxy |
| 播放中途断 | 代理误用短超时 | 确认 `FNTVBOX_PROXY_HEADER_TIMEOUT_MS`；代理 Client 无整体 Timeout |
| 插件全挂但 Kodi 还在 | gateway 崩了 | 看门狗应拉起；查崩溃日志；超上限会整容器重启 |
| 花屏/绿屏 | 驱动/DRM | 查 intel media |
| 无声音 | `/dev/snd` 未透传 | 补 devices |
| 插件改了不生效 | 卷里旧 addons | 删对应 addon 或升版本强制覆盖 |
| 调试时映射了 18765 | 忘记删除 | 去掉 compose ports；验收清单打回 |
| Skin 加载失败 | XML/字体 | 查 `kodi.log`；确认 Noto CJK |

---

## 13. 环境变量速查

| 变量 | 作用域 | 说明 |
|------|--------|------|
| `FNTVBOX_LISTEN` | 容器 | 网关监听（必须 `127.0.0.1:18765`） |
| `FNTVBOX_SUBSCRIPTION_URL` | 容器 | 订阅 URL |
| `FNTVBOX_DATA_DIR` | 容器 | 网关数据目录 |
| `FNTVBOX_CACHE_TTL_SEC` | 容器 | 订阅/直播缓存秒数（默认 300） |
| `FNTVBOX_MEDIA_CACHE_TTL_SEC` | 容器 | 分类/列表 JSON 缓存秒数（默认 900） |
| `FNTVBOX_HTTP_TIMEOUT_MS` | 容器 | 短请求整体超时（搜索每源同此） |
| `FNTVBOX_PROXY_HEADER_TIMEOUT_MS` | 容器 | 媒体代理首字节超时 |
| `FNTVBOX_GATEWAY_MAX_CRASHES` | 容器 | 看门狗崩溃上限 |
| `FNTVBOX_GATEWAY_RESTART_DELAY_SEC` | 容器 | 重启间隔 |
| `FNTVBOX_GATEWAY_STABLE_SEC` | 容器 | 稳定后重置崩溃计数 |
| `TZ` / `LANG` | 容器 | 时区与语言 |
| `wizard_subscription_url` | compose | FPK 向导 |
| `wizard_airplay_support` | compose | `bridge` / `host` |

---

## 14. 最小订阅模板

见 [`实现路线与部署.md`](../实现路线与部署.md) 附录 A；将 CMS / M3U URL 换成真实可访问地址后再用于向导。
